// Command wikimindd is the WikiMind daemon process.
//
// Runs the main loop: watcher, lock reaper, bundle merger, IPC bridge.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/fengxd1222/llm-wiki/internal/daemon"
)

const version = "0.1.0-dev"

func main() {
	var vaultRoot string
	flag.StringVar(&vaultRoot, "vault", "", "Path to WikiMind vault")
	flag.Parse()

	if vaultRoot == "" {
		fmt.Fprintf(os.Stderr, "usage: wikimindd --vault <path>\n")
		os.Exit(1)
	}

	cfg := daemon.Config{VaultRoot: vaultRoot}
	d, err := daemon.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon init: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Fprintf(os.Stderr, "wikimindd %s starting (vault=%s)\n", version, vaultRoot)
	if err := d.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "daemon error: %v\n", err)
		os.Exit(1)
	}
}
