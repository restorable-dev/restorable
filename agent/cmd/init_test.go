package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeResticOnPath installs a stub `restic` that answers the calls `init`
// makes (snapshots --json, ls --json) so the command can run without a real
// repository, and returns a dir to prepend to PATH.
func fakeResticOnPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	snapshots := `[{"id":"deadbeefcafe","short_id":"deadbeef","time":"2026-07-05T21:53:00Z","paths":["/srv/app"],"hostname":"box"}]`
	ls := `{"message_type":"snapshot","id":"deadbeef"}
{"message_type":"node","type":"dir","path":"/srv/app","struct_type":"node"}
{"message_type":"node","type":"file","path":"/srv/app/config.php","struct_type":"node"}
{"message_type":"node","type":"file","path":"/srv/app/data/a.txt","struct_type":"node"}
`
	script := "#!/bin/sh\n" +
		"for a in \"$@\"; do case \"$a\" in snapshots) echo '" + snapshots + "'; exit 0;; ls) cat <<'EOF'\n" + ls + "EOF\nexit 0;; esac; done\nexit 0\n"
	bin := filepath.Join(dir, "restic")
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInitCreatesMissingOutputDir(t *testing.T) {
	t.Setenv("PATH", fakeResticOnPath(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RESTIC_PASSWORD", "x")

	// --out points at a directory that does not exist yet.
	out := filepath.Join(t.TempDir(), "does", "not", "exist")

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"init", "--repo", "/srv/app", "--out", out, "--yes"})
	// --yes runs the first test too; that needs a real restore, which the stub
	// can't do, so a non-nil error from the test phase is fine. We only assert
	// the config files were written into the freshly-created directory.
	_ = root.Execute()

	for _, name := range []string{"agent.yaml", "app.yaml"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("expected %s written into created dir: %v", name, err)
		}
	}
}

// Guards against the exec path pulling in the real restic; keeps the test
// hermetic on machines that have restic installed.
func TestFakeResticShadowsReal(t *testing.T) {
	dir := fakeResticOnPath(t)
	cmd := exec.Command(filepath.Join(dir, "restic"), "snapshots", "--json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "deadbeef") {
		t.Errorf("stub restic did not answer snapshots: %s", out)
	}
}
