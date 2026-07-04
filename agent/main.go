package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dabelle/restorable/agent/cmd"
)

// Exit codes: 0 = pass, 1 = verification failed, 2 = could not test
// (configuration, usage, repository, or infrastructure problems).
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := cmd.Execute(ctx)
	if err == nil {
		return
	}
	if msg := err.Error(); msg != "" {
		fmt.Fprintln(os.Stderr, "Error:", msg)
	}
	code := 2
	var coder interface{ ExitCode() int }
	if errors.As(err, &coder) {
		code = coder.ExitCode()
	}
	os.Exit(code)
}
