package main

import (
	"fmt"
	"io"
	"os"
	"strings"

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
	cmd.AddCommand(newPageCommand(stdout))
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

func newPageCommand(stdout io.Writer) *cobra.Command {
	page := &cobra.Command{
		Use:   "page",
		Short: "Inspect and reindex wiki/ pages",
	}
	page.AddCommand(newPageReindexCommand(stdout))
	page.AddCommand(newPageListCommand(stdout))
	page.AddCommand(newPageShowCommand(stdout))
	return page
}

func newPageReindexCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "reindex",
		Short: "Walk wiki/ and reindex pages + pages_fts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultRoot, db, err := openVaultAndIndex()
			if err != nil {
				return err
			}
			defer db.Close()
			res, err := service.ReindexWiki(cmd.Context(), db, vaultRoot)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "indexed %d pages\n", res.Count)
			if len(res.Skipped) > 0 {
				fmt.Fprintf(stdout, "skipped %d files (parse error):\n", len(res.Skipped))
				for _, p := range res.Skipped {
					fmt.Fprintf(stdout, "  - %s\n", p)
				}
			}
			return nil
		},
	}
}

func newPageListCommand(stdout io.Writer) *cobra.Command {
	var typeFilter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List indexed pages (group by type)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, err := openVaultAndIndex()
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := index.ListPages(cmd.Context(), db, typeFilter)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(stdout, "no pages indexed yet — run 'wikimind page reindex' first")
				return nil
			}
			printPagesGrouped(stdout, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "filter by page type (claim/entity/concept/source/topic)")
	return cmd
}

func newPageShowCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a page by id (frontmatter + body excerpt)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, err := openVaultAndIndex()
			if err != nil {
				return err
			}
			defer db.Close()
			row, err := index.GetPageByID(cmd.Context(), db, args[0])
			if err != nil {
				return err
			}
			if row == nil {
				return fmt.Errorf("page %q not found (try 'wikimind page reindex')", args[0])
			}
			printPageDetail(stdout, row)
			return nil
		},
	}
}

// openVaultAndIndex resolves vault root from cwd and opens the SQLite index.
func openVaultAndIndex() (string, *index.DB, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("resolve working directory: %w", err)
	}
	vaultRoot, err := vault.FindRoot(start)
	if err != nil {
		return "", nil, err
	}
	db, err := index.Open(vaultRoot)
	if err != nil {
		return "", nil, err
	}
	return vaultRoot, db, nil
}

// printPagesGrouped prints pages grouped by type, sorted by id within each type.
func printPagesGrouped(w io.Writer, rows []*index.PageRow) {
	currentType := ""
	for _, p := range rows {
		if p.Type != currentType {
			if currentType != "" {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "## %s\n", p.Type)
			currentType = p.Type
		}
		fmt.Fprintf(w, "- %s\t%s\t(%s)\n", p.ID, p.Title, p.Path)
	}
}

// printPageDetail prints a single page's frontmatter (JSON) + first 20 body lines.
func printPageDetail(w io.Writer, p *index.PageRow) {
	fmt.Fprintf(w, "id: %s\n", p.ID)
	fmt.Fprintf(w, "type: %s\n", p.Type)
	fmt.Fprintf(w, "path: %s\n", p.Path)
	fmt.Fprintf(w, "title: %s\n", p.Title)
	if p.Confidence.Valid {
		fmt.Fprintf(w, "confidence: %g\n", p.Confidence.Float64)
	}
	if p.Status != "" {
		fmt.Fprintf(w, "status: %s\n", p.Status)
	}
	fmt.Fprintf(w, "schema_version: %s\n", p.SchemaVersion)
	if p.Frontmatter != "" {
		fmt.Fprintf(w, "frontmatter: %s\n", p.Frontmatter)
	}
	fmt.Fprintln(w, "---")
	lines := strings.Split(p.Body, "\n")
	limit := 20
	if len(lines) < limit {
		limit = len(lines)
	}
	for i := 0; i < limit; i++ {
		fmt.Fprintln(w, lines[i])
	}
	if len(lines) > limit {
		fmt.Fprintf(w, "... (%d more lines)\n", len(lines)-limit)
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
