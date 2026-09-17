package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

// cleanupTimeout bounds container removal. Cleanup uses its own context so it
// still runs when the surrounding run is cancelled (SIGTERM, timeout).
const cleanupTimeout = 60 * time.Second

// How long to wait for the daemon to publish a container's port, and how often
// to re-check. Publication is asynchronous, so the binding is routinely absent
// for a moment after start; a single inspect loses that race every time.
const (
	portPublishTimeout = 15 * time.Second
	portPollInterval   = 100 * time.Millisecond
)

// ContainerSpec describes a throwaway container for a verification check.
type ContainerSpec struct {
	Image string
	Env   []string
	// Binds are host bind mounts, "hostPath:containerPath" (+":ro" if wanted).
	Binds []string
	// PublishPort is a container port ("8080/tcp") to expose on an
	// ephemeral host port. Empty means no published ports.
	PublishPort string
}

// Docker manages throwaway containers for verification checks and guarantees
// their removal — every container it creates is force-removed, along with its
// anonymous volumes, in Cleanup.
type Docker struct {
	cli        *client.Client
	containers []string
}

// NewDocker connects to the Docker daemon. The error is user-facing and
// explains what to do when Docker is absent — the agent degrades gracefully:
// checks that need Docker report it, everything else keeps working.
//
// When DOCKER_HOST is unset and the default socket does not answer, the
// Docker Desktop per-user socket (~/.docker/run/docker.sock) is tried as a
// fallback: the SDK does not resolve docker CLI contexts, and on macOS the
// daemon usually only listens there.
func NewDocker(ctx context.Context) (*Docker, error) {
	cli, err := connect(ctx, client.FromEnv)
	if err == nil {
		return &Docker{cli: cli}, nil
	}
	if os.Getenv(client.EnvOverrideHost) == "" {
		home, herr := os.UserHomeDir()
		if herr == nil {
			desktopSock := "unix://" + filepath.Join(home, ".docker", "run", "docker.sock")
			if cli, derr := connect(ctx, client.WithHost(desktopSock)); derr == nil {
				return &Docker{cli: cli}, nil
			}
		}
	}
	return nil, fmt.Errorf("docker is not available (daemon not running or socket unreachable): %w", err)
}

func connect(ctx context.Context, hostOpt client.Opt) (*client.Client, error) {
	cli, err := client.NewClientWithOpts(hostOpt, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(pingCtx); err != nil {
		_ = cli.Close()
		return nil, err
	}
	return cli, nil
}

// StartContainer creates and starts a container, pulling the image first if
// it is missing locally. The container is registered for cleanup before it is
// started, so even a failed start cannot leak it.
func (d *Docker) StartContainer(ctx context.Context, spec ContainerSpec) (string, error) {
	cfg := &container.Config{
		Image:  spec.Image,
		Env:    spec.Env,
		Labels: map[string]string{"restorable": "sandbox"},
	}
	host := &container.HostConfig{Binds: spec.Binds}
	if spec.PublishPort != "" {
		// NewPort takes the protocol first, then the port. Passing them the
		// other way round parses "tcp" as a port number and fails every time.
		port, err := nat.NewPort("tcp", strings.TrimSuffix(spec.PublishPort, "/tcp"))
		if err != nil {
			return "", fmt.Errorf("invalid port %q: %w", spec.PublishPort, err)
		}
		cfg.ExposedPorts = nat.PortSet{port: struct{}{}}
		// Empty HostPort → the daemon picks an ephemeral port.
		host.PortBindings = nat.PortMap{port: []nat.PortBinding{{HostIP: "127.0.0.1"}}}
	}

	created, err := d.cli.ContainerCreate(ctx, cfg, host, nil, nil, "")
	if cerrdefs.IsNotFound(err) {
		if perr := d.pullImage(ctx, spec.Image); perr != nil {
			return "", perr
		}
		created, err = d.cli.ContainerCreate(ctx, cfg, host, nil, nil, "")
	}
	if err != nil {
		return "", fmt.Errorf("create container from %s: %w", spec.Image, err)
	}
	d.containers = append(d.containers, created.ID)

	if err := d.cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("start container from %s: %w", spec.Image, err)
	}
	return created.ID, nil
}

func (d *Docker) pullImage(ctx context.Context, ref string) error {
	rc, err := d.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", ref, err)
	}
	defer rc.Close() //nolint:errcheck // best-effort close of pull stream
	// The pull only completes once the progress stream is drained.
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return fmt.Errorf("pull image %s: %w", ref, err)
	}
	return nil
}

// Exec runs cmd inside the container and returns its exit code and combined
// output (stdout + stderr).
func (d *Docker) Exec(ctx context.Context, id string, cmd []string) (int, string, error) {
	exec, err := d.cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return 0, "", fmt.Errorf("exec create: %w", err)
	}
	attach, err := d.cli.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		return 0, "", fmt.Errorf("exec attach: %w", err)
	}
	defer attach.Close()
	var out bytes.Buffer
	if _, err := stdcopy.StdCopy(&out, &out, attach.Reader); err != nil {
		return 0, "", fmt.Errorf("exec read output: %w", err)
	}
	inspect, err := d.cli.ContainerExecInspect(ctx, exec.ID)
	if err != nil {
		return 0, "", fmt.Errorf("exec inspect: %w", err)
	}
	return inspect.ExitCode, out.String(), nil
}

// CopyTo streams content into the container as file dstDir/name. size must be
// the exact content length (tar requires it up front).
func (d *Docker) CopyTo(ctx context.Context, id, dstDir, name string, content io.Reader, size int64) error {
	pr, pw := io.Pipe()
	go func() {
		tw := tar.NewWriter(pw)
		err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: size})
		if err == nil {
			_, err = io.Copy(tw, content)
		}
		if err == nil {
			err = tw.Close()
		}
		_ = pw.CloseWithError(err) // always returns nil
	}()
	if err := d.cli.CopyToContainer(ctx, id, dstDir, pr, container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("copy %s into container: %w", name, err)
	}
	return nil
}

// MappedPort returns the ephemeral host port bound to the given container
// port ("8080/tcp").
func (d *Docker) MappedPort(ctx context.Context, id, containerPort string) (string, error) {
	port := nat.Port(containerPort)
	deadline := time.Now().Add(portPublishTimeout)
	for {
		inspect, err := d.cli.ContainerInspect(ctx, id)
		if err != nil {
			return "", fmt.Errorf("inspect container: %w", err)
		}
		// The daemon fills NetworkSettings.Ports asynchronously, so an inspect
		// issued straight after start reliably returns an empty binding even
		// though HostConfig already carries it. Poll rather than read once.
		if b := inspect.NetworkSettings.Ports[port]; len(b) > 0 && b[0].HostPort != "" {
			return b[0].HostPort, nil
		}
		// An app that dies on the restored data never publishes anything. That
		// is the common real failure here, so name it instead of blaming the
		// port and sending the user looking in the wrong place.
		if !inspect.State.Running {
			return "", fmt.Errorf("container exited early with code %d before publishing port %s",
				inspect.State.ExitCode, containerPort)
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("container port %s was not published within %s",
				containerPort, portPublishTimeout)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(portPollInterval):
		}
	}
}

// State reports whether the container is still running, and its exit code
// once it is not.
func (d *Docker) State(ctx context.Context, id string) (running bool, exitCode int, err error) {
	inspect, err := d.cli.ContainerInspect(ctx, id)
	if err != nil {
		return false, 0, fmt.Errorf("inspect container: %w", err)
	}
	return inspect.State.Running, inspect.State.ExitCode, nil
}

// Cleanup force-removes every container this sandbox created, including
// their anonymous volumes. It runs on its own context so it works even when
// the run was cancelled, and keeps going past individual failures.
func (d *Docker) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	var errs []error
	for _, id := range d.containers {
		err := d.cli.ContainerRemove(ctx, id, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		})
		if err != nil && !cerrdefs.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("remove container %.12s: %w", id, err))
		}
	}
	d.containers = nil
	if cerr := d.cli.Close(); cerr != nil {
		errs = append(errs, cerr)
	}
	return errors.Join(errs...)
}
