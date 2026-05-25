package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newDemoCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "demo [vault-path]",
		Short: "Run a 5-minute guided demo (init + ingest + query + lint)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultPath := filepath.Join(os.TempDir(), "wikimind-demo")
			if len(args) > 0 {
				vaultPath = args[0]
			}

			fmt.Fprintf(stdout, "🚀 WikiMind 5-Minute Demo\n")
			fmt.Fprintf(stdout, "========================\n\n")
			fmt.Fprintf(stdout, "This demo will:\n")
			fmt.Fprintf(stdout, "  1. Initialize a vault at %s\n", vaultPath)
			fmt.Fprintf(stdout, "  2. Ingest a sample markdown file\n")
			fmt.Fprintf(stdout, "  3. Query the vault (FTS5 search)\n")
			fmt.Fprintf(stdout, "  4. Run lint checks\n")
			fmt.Fprintf(stdout, "  5. Show vault status\n\n")

			// Step 1: Init
			fmt.Fprintf(stdout, "Step 1: Initializing vault...\n")
			initCmd := newRootCommand(stdout, stdout)
			initCmd.SetArgs([]string{"init", vaultPath})
			if err := initCmd.Execute(); err != nil {
				return fmt.Errorf("demo init: %w", err)
			}
			fmt.Fprintf(stdout, "\n")

			// Step 2: Create and ingest a sample file
			fmt.Fprintf(stdout, "Step 2: Ingesting sample file...\n")
			samplePath := filepath.Join(os.TempDir(), "wikimind-demo-sample.md")
			sample := []byte(`---
title: "WikiMind Quick Start"
---

# WikiMind Quick Start

WikiMind is a local-first multi-agent wiki that compounds knowledge over time.

## Key Features

- **Local-first**: All data stays on your machine
- **Multi-agent**: Claude, Codex, and other AI agents can propose changes
- **Git-backed**: Every change is a git commit with full audit trail
- **FTS5 search**: Fast full-text search with CJK support
`)
			if err := os.WriteFile(samplePath, sample, 0o644); err != nil {
				return fmt.Errorf("write sample: %w", err)
			}
			defer os.Remove(samplePath)

			ingestCmd := newRootCommand(stdout, stdout)
			ingestCmd.SetArgs([]string{"ingest", samplePath, "--vault", vaultPath})
			// Use chdir approach since ingest resolves vault from CWD
			origDir, _ := os.Getwd()
			_ = os.Chdir(vaultPath)
			ingestCmd2 := newRootCommand(stdout, stdout)
			ingestCmd2.SetArgs([]string{"ingest", samplePath})
			if err := ingestCmd2.Execute(); err != nil {
				_ = os.Chdir(origDir)
				return fmt.Errorf("demo ingest: %w", err)
			}
			fmt.Fprintf(stdout, "\n")

			// Step 3: Query
			fmt.Fprintf(stdout, "Step 3: Searching for 'WikiMind'...\n")
			queryCmd := newRootCommand(stdout, stdout)
			queryCmd.SetArgs([]string{"query", "WikiMind"})
			if err := queryCmd.Execute(); err != nil {
				_ = os.Chdir(origDir)
				return fmt.Errorf("demo query: %w", err)
			}
			fmt.Fprintf(stdout, "\n")

			// Step 4: Lint
			fmt.Fprintf(stdout, "Step 4: Running lint checks...\n")
			lintCmd := newRootCommand(stdout, stdout)
			lintCmd.SetArgs([]string{"lint"})
			_ = lintCmd.Execute() // lint may return error for findings, that's OK
			fmt.Fprintf(stdout, "\n")

			// Step 5: Status
			fmt.Fprintf(stdout, "Step 5: Vault status...\n")
			statusCmd := newRootCommand(stdout, stdout)
			statusCmd.SetArgs([]string{"status"})
			if err := statusCmd.Execute(); err != nil {
				_ = os.Chdir(origDir)
				return fmt.Errorf("demo status: %w", err)
			}

			_ = os.Chdir(origDir)
			fmt.Fprintf(stdout, "\n✅ Demo complete! Vault at: %s\n", vaultPath)
			fmt.Fprintf(stdout, "Next steps:\n")
			fmt.Fprintf(stdout, "  - wikimind doctor          # check dependencies\n")
			fmt.Fprintf(stdout, "  - wikimind mcp serve       # start MCP server for AI agents\n")
			fmt.Fprintf(stdout, "  - wikimind watch --auto-ingest  # watch for new files\n")
			return nil
		},
	}
}
