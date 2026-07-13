package recipes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/restorable-dev/restorable/agent/internal/report"
)

// appTarget wires a fake runner whose MappedPort points at a local HTTP
// server standing in for the containerized app.
func appTarget(t *testing.T, handler http.Handler) (*Target, *fakeRunner) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{port: u.Port()}
	root := t.TempDir()
	return &Target{
		Roots: []Root{{Dir: root, SnapPath: "/srv/app"}},
		Docker: func(context.Context) (ContainerRunner, error) {
			return runner, nil
		},
	}, runner
}

func TestDockerAppCheck(t *testing.T) {
	check := DockerAppCheck{
		Image: "nextcloud:apache",
		Mount: MountSpec{Restored: ".", At: "/var/www/html"},
		Ready: ReadySpec{
			HTTP:     "http://localhost:8080/status.php",
			Contains: `"installed":true`,
		},
	}

	t.Run("healthy app passes", func(t *testing.T) {
		target, runner := appTarget(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/status.php" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"installed":true,"version":"29.0.0"}`))
		}))
		status, msg := check.Run(context.Background(), target)
		if status != report.StatusPass {
			t.Fatalf("status = %s (%s)", status, msg)
		}
		// The container port from ready.http must be published.
		if runner.specs[0].PublishPort != "8080/tcp" {
			t.Errorf("published %q, want 8080/tcp", runner.specs[0].PublishPort)
		}
		// The restored root must be bind-mounted at mount.at.
		if len(runner.specs[0].Binds) != 1 || !strings.HasSuffix(runner.specs[0].Binds[0], ":/var/www/html:ro") {
			t.Errorf("binds = %v", runner.specs[0].Binds)
		}
	})

	t.Run("body without required text fails", func(t *testing.T) {
		shortCheck := check
		shortCheck.Ready.Timeout = Duration{Duration: 50 * 1e6} // 50ms
		target, _ := appTarget(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"installed":false,"maintenance":true}`))
		}))
		status, msg := shortCheck.Run(context.Background(), target)
		if status != report.StatusFail {
			t.Fatalf("status = %s (%s)", status, msg)
		}
		if !strings.Contains(msg, "does not contain") {
			t.Errorf("message %q should explain the body mismatch", msg)
		}
	})

	t.Run("http error fails after timeout", func(t *testing.T) {
		shortCheck := check
		shortCheck.Ready.Timeout = Duration{Duration: 50 * 1e6}
		target, _ := appTarget(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "app is broken", http.StatusInternalServerError)
		}))
		status, msg := shortCheck.Run(context.Background(), target)
		if status != report.StatusFail || !strings.Contains(msg, "HTTP 500") {
			t.Fatalf("status = %s (%s), want fail with HTTP 500", status, msg)
		}
	})

	t.Run("container exiting early fails fast", func(t *testing.T) {
		target, runner := appTarget(t, http.NotFoundHandler())
		runner.stopped = true
		status, msg := check.Run(context.Background(), target)
		if status != report.StatusFail || !strings.Contains(msg, "exited early") {
			t.Fatalf("status = %s (%s), want fail/exited-early", status, msg)
		}
	})
}
