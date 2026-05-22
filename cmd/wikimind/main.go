package main

import (
	"fmt"
	"os"
)

var version = "0.1.0-d1"

func main() {
	cmd := newRootCommand(os.Stdout, os.Stderr)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
