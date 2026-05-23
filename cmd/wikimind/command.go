package main

import (
	"fmt"
	"io"
	"os"

	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/service"
	"github.com/fengxd1222/llm-wiki/internal/vault"
	"github.com/spf13/cobra"
)

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "wikimind",
		Short:         "WikiMind local-first knowledge base CLI",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	cmd.AddCommand(newInitCommand(stdout))
	cmd.AddCommand(newStatusCommand(stdout))
	cmd.AddCommand(newIngestCommand(stdout))
	for _, name := range []string{"query", "review", "lint", "revert"} {
		cmd.AddCommand(newStubCommand(stdout, name))
	}
	return cmd
}

func newInitCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "init <vault>",
		Short: "Initialize a WikiMind vault",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := vault.Init(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "initialized: %s\nschema_version: %s\n", result.Root, result.SchemaVersion)
			return nil
		},
	}
}

func newStatusCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "status [vault]",
		Short: "Show WikiMind vault status",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start := "."
			if len(args) == 1 {
				start = args[0]
			} else if cwd, err := os.Getwd(); err == nil {
				start = cwd
			}
			status, err := vault.ReadStatus(start)
			if err != nil {
				return err
			}
			printStatus(stdout, status)
			return nil
		},
	}
}

func newIngestCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "ingest <path>",
		Short: "Ingest a file into raw/inbox/ and record sources",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			start, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve working directory: %w", err)
			}
			vaultRoot, err := vault.FindRoot(start)
			if err != nil {
				return err
			}
			db, err := index.Open(vaultRoot)
			if err != nil {
				return err
			}
			defer db.Close()

			result, err := service.IngestFile(cmd.Context(), db, vaultRoot, args[0])
			if err != nil {
				return err
			}

			src := result.Source
			marker := "ingested"
			if result.Duplicate {
				marker = "duplicate"
			}
			fmt.Fprintf(stdout, "%s: %s\n", marker, src.RawID)
			fmt.Fprintf(stdout, "sha256: %s\n", src.SHA256)
			fmt.Fprintf(stdout, "size: %d\n", src.Size)
			fmt.Fprintf(stdout, "status: %s\n", src.Status)
			return nil
		},
	}
}

func newStubCommand(stdout io.Writer, name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: "D1 stub",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(stdout, "wikimind %s: D1 未实现\n", name)
			return nil
		},
	}
}

func printStatus(w io.Writer, status *vault.Status) {
	gitState := "unavailable"
	gitBranch := "unknown"
	if status.Git.Available {
		gitBranch = status.Git.Branch
		if status.Git.Clean {
			gitState = "clean"
		} else {
			gitState = "dirty"
		}
	}

	fmt.Fprintf(w, "vault: %s\n", status.Root)
	fmt.Fprintf(w, "schema_version: %s\n", status.SchemaVersion)
	fmt.Fprintf(w, "raw_files: %d\n", status.RawFiles)
	fmt.Fprintf(w, "wiki_pages: %d\n", status.WikiPages)
	fmt.Fprintf(w, "claims: %d\n", status.Claims)
	fmt.Fprintf(w, "git_branch: %s\n", gitBranch)
	fmt.Fprintf(w, "git_status: %s\n", gitState)
	fmt.Fprintf(w, "config: %s\n", configStatusLine(status.Root))
	fmt.Fprintln(w, "health: ok")
}

func configStatusLine(root string) string {
	if _, err := vault.LoadConfig(root); err != nil {
		return "invalid: " + err.Error()
	}
	return "ok"
}
