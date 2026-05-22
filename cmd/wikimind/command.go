package main

import (
	"fmt"
	"io"
	"os"

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
	for _, name := range []string{"ingest", "query", "review", "lint", "revert"} {
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
	fmt.Fprintln(w, "health: ok")
}
