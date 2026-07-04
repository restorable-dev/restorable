package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "version subcommand prints version, commit and platform",
			args: []string{"version"},
			want: []string{"restorable dev", "commit none"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newRootCmd()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs(tt.args)

			if err := root.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output %q does not contain %q", out.String(), want)
				}
			}
		})
	}
}

func TestVersionCommandRejectsArgs(t *testing.T) {
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"version", "extra"})

	if err := root.Execute(); err == nil {
		t.Fatal("Execute() with extra args should error")
	}
}
