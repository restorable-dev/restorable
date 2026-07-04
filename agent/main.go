package main

import (
	"os"

	"github.com/dabelle/restorable/agent/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
