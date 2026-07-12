package recipes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/restorable-dev/restorable/agent/internal/report"
	"github.com/restorable-dev/restorable/agent/internal/sandbox"
)

// DockerAppCheck boots the real application image against the restored data
// and verifies the app actually comes up — the strongest possible evidence a
// backup is restorable.
type DockerAppCheck struct {
	Type  string            `yaml:"type"`
	Image string            `yaml:"image"`
	Mount MountSpec         `yaml:"mount"`
	Env   map[string]string `yaml:"env"`
	Ready ReadySpec         `yaml:"ready"`
}

// MountSpec binds a restored path into the container.
type MountSpec struct {
	// Restored is the path within the snapshot to mount ("." = the root).
	Restored string `yaml:"restored"`
	// At is the mount point inside the container.
	At string `yaml:"at"`
}

// ReadySpec defines when the app counts as up.
type ReadySpec struct {
	// HTTP is a URL like "http://localhost:8080/status.php". Its port is
	// the CONTAINER port; the agent publishes it on an ephemeral host port
	// and probes that.
	HTTP string `yaml:"http"`
	// Contains optionally requires the response body to contain this text.
	Contains string   `yaml:"contains"`
	Timeout  Duration `yaml:"timeout"`
}

// TypeName implements Check.
func (c *DockerAppCheck) TypeName() string { return "docker-app" }

func (c *DockerAppCheck) validate() error {
	if c.Image == "" {
		return errors.New("needs an image")
	}
	if c.Mount.At == "" {
		return errors.New("needs mount.at (container path to mount restored data)")
	}
	if !strings.HasPrefix(c.Mount.At, "/") {
		return fmt.Errorf("mount.at %q must be an absolute container path", c.Mount.At)
	}
	if c.Ready.HTTP == "" {
		return errors.New("needs ready.http (URL to probe)")
	}
	u, err := url.Parse(c.Ready.HTTP)
	if err != nil || u.Scheme != "http" || u.Host == "" {
		return fmt.Errorf("ready.http %q must be an http:// URL", c.Ready.HTTP)
	}
	return nil
}

// Run implements Check.
func (c *DockerAppCheck) Run(ctx context.Context, t *Target) (report.Status, string) {
	restored := strings.TrimSuffix(c.Mount.Restored, "/")
	var hostDir string
	if restored == "" || restored == "." {
		// Mount the (single) snapshot root itself.
		if len(t.Roots) != 1 {
			return report.StatusError, fmt.Sprintf(
				"mount.restored \".\" is ambiguous: snapshot has %d roots", len(t.Roots))
		}
		hostDir = t.Roots[0].Dir
	} else {
		var ok bool
		hostDir, ok = resolve(t.Roots, restored)
		if !ok {
			return report.StatusFail, fmt.Sprintf("mount path %q not found in restored snapshot", c.Mount.Restored)
		}
	}

	probeURL, err := url.Parse(c.Ready.HTTP)
	if err != nil {
		return report.StatusError, fmt.Sprintf("parse ready.http: %v", err)
	}
	containerPort := probeURL.Port()
	if containerPort == "" {
		containerPort = "80"
	}

	runner, err := t.Docker(ctx)
	if err != nil {
		return report.StatusError, err.Error()
	}

	env := make([]string, 0, len(c.Env))
	for k, v := range c.Env {
		env = append(env, k+"="+v)
	}
	sort.Strings(env)

	id, err := runner.StartContainer(ctx, sandbox.ContainerSpec{
		Image:       c.Image,
		Env:         env,
		Binds:       []string{hostDir + ":" + c.Mount.At},
		PublishPort: containerPort + "/tcp",
	})
	if err != nil {
		return report.StatusError, err.Error()
	}

	hostPort, err := runner.MappedPort(ctx, id, containerPort+"/tcp")
	if err != nil {
		return report.StatusError, err.Error()
	}
	target := fmt.Sprintf("http://127.0.0.1:%s%s", hostPort, probeURL.RequestURI())

	client := &http.Client{}
	err = waitFor(ctx, runner, id, c.Ready.Timeout.orDefault(defaultReadyTimeout), func() (bool, string) {
		req, rerr := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if rerr != nil {
			return false, rerr.Error()
		}
		resp, rerr := client.Do(req)
		if rerr != nil {
			return false, rerr.Error()
		}
		defer resp.Body.Close() //nolint:errcheck // response fully read
		body, rerr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if rerr != nil {
			return false, rerr.Error()
		}
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Sprintf("HTTP %d from %s", resp.StatusCode, probeURL.Path)
		}
		if c.Ready.Contains != "" && !strings.Contains(string(body), c.Ready.Contains) {
			return false, fmt.Sprintf("HTTP 200 but body does not contain %q", c.Ready.Contains)
		}
		return true, "ready"
	})
	if err != nil {
		return report.StatusFail, fmt.Sprintf("app %s did not become healthy on restored data: %v", c.Image, err)
	}

	msg := fmt.Sprintf("%s serves HTTP 200 on %s with restored data", c.Image, probeURL.Path)
	if c.Ready.Contains != "" {
		msg += fmt.Sprintf(" (body contains %q)", c.Ready.Contains)
	}
	return report.StatusPass, msg
}
