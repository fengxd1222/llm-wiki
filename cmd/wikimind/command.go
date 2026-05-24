package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/fengxd1222/llm-wiki/internal/commit"
	"github.com/fengxd1222/llm-wiki/internal/index"
	mcppkg "github.com/fengxd1222/llm-wiki/internal/mcp"
	"github.com/fengxd1222/llm-wiki/internal/service"
	"github.com/fengxd1222/llm-wiki/internal/vault"
	worktreepkg "github.com/fengxd1222/llm-wiki/internal/worktree"
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
	cmd.AddCommand(newQueryCommand(stdout))
	cmd.AddCommand(newRevertCommand(stdout, os.Stdin))
	cmd.AddCommand(newMcpCommand(stdout, stderr))
	cmd.AddCommand(newWorktreeCommand(stdout))
	for _, name := range []string{"review", "lint"} {
		cmd.AddCommand(newStubCommand(stdout, name))
	}
	return cmd
}

// newMcpCommand 父命令 `wikimind mcp`，承载 D8 起的 MCP 子命令。
// D8 只有 `serve`；D10+ 可能加 `inspect` / `tools list` 等便利命令。
func newMcpCommand(stdout, stderr io.Writer) *cobra.Command {
	mcp := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
	}
	mcp.AddCommand(newMcpServeCommand(stdout, stderr))
	return mcp
}

// newMcpServeCommand 在 stdin/stdout 上跑 wikimind MCP server。
//
// 关键纪律：
//   - stdout 是 MCP protocol 通道——本命令运行期间禁止任何写 stdout 的
//     副作用（默认 cobra.Command 不会写 stdout，自定义打印必须改 stderr）
//   - 所有 logging 走 stderr（log.New(os.Stderr, ...)）
//   - 接 SIGINT/SIGTERM 优雅退出，让 Claude Desktop 能干净关 server
//
// 参数 stderr 是 root command 持有的 stderr writer——给 test/CLI 复用同一
// 通道；运行时 default 是 os.Stderr。
func newMcpServeCommand(stdout, stderr io.Writer) *cobra.Command {
	var vaultPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run WikiMind MCP server on stdio (agent_handshake, wiki_info, read_page, read_raw, list_index, search, read_raw_anchor, read_claim, graph_neighbors, get_history)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = stdout // stdout is the MCP protocol stream; not for human-readable output.
			logger := log.New(stderr, "wikimind-mcp: ", log.LstdFlags|log.Lmicroseconds)

			vaultRoot, err := resolveVaultForServe(vaultPath)
			if err != nil {
				return err
			}
			logger.Printf("vault=%s", vaultRoot)

			db, err := index.Open(vaultRoot)
			if err != nil {
				return fmt.Errorf("open index: %w", err)
			}
			defer db.Close()

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			server, err := mcppkg.NewServer(ctx, vaultRoot, db)
			if err != nil {
				return fmt.Errorf("build mcp server: %w", err)
			}
			logger.Printf("ready: 10 tools registered")

			if err := mcppkg.RunStdio(ctx, server); err != nil {
				// ctx cancel 触发的关闭走 SDK 内部 EOF/ContextDone；
				// 视作正常退出避免 wrap 噪声。
				if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
					logger.Printf("shutdown via signal")
					return nil
				}
				return err
			}
			logger.Printf("shutdown clean")
			return nil
		},
	}
	cmd.Flags().StringVar(&vaultPath, "vault", "",
		"vault root (default: walk up from cwd to find .wikimind/config.toml)")
	return cmd
}

func newWorktreeCommand(stdout io.Writer) *cobra.Command {
	worktree := &cobra.Command{
		Use:   "worktree",
		Short: "Inspect and clean agent worktrees",
	}
	worktree.AddCommand(newWorktreeListCommand(stdout))
	worktree.AddCommand(newWorktreeRemoveCommand(stdout))
	return worktree
}

func newWorktreeListCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List agent git worktrees",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultRoot, err := resolveVaultFromCWD()
			if err != nil {
				return err
			}
			worktrees, err := worktreepkg.ListWorktrees(cmd.Context(), vaultRoot)
			if err != nil {
				return err
			}
			if len(worktrees) == 0 {
				fmt.Fprintln(stdout, "no worktrees")
				return nil
			}
			for _, wt := range worktrees {
				path := wt.Path
				if rel, err := filepath.Rel(vaultRoot, wt.Path); err == nil {
					path = filepath.ToSlash(rel)
				}
				fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", wt.Branch, wt.Agent, wt.SessionID, path)
			}
			return nil
		},
	}
}

func newWorktreeRemoveCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <agent>/<session-id>",
		Short: "Force-remove an agent git worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, sessionID, ok := strings.Cut(args[0], "/")
			if !ok || strings.TrimSpace(agent) == "" || strings.TrimSpace(sessionID) == "" {
				return fmt.Errorf("worktree remove: expected <agent>/<session-id>, got %q", args[0])
			}
			vaultRoot, err := resolveVaultFromCWD()
			if err != nil {
				return err
			}
			if err := worktreepkg.RemoveWorktree(cmd.Context(), vaultRoot, agent, sessionID); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "removed: %s/%s\n", agent, sessionID)
			return nil
		},
	}
}

// resolveVaultForServe 解析 mcp serve 的 vault root：
//   - --vault <path> 显式指定 → vault.FindRoot 从该路径向上找
//   - 未指定 → 从 cwd 向上找
//
// 失败给清晰错误，不让 server 带空 config 起来。
func resolveVaultForServe(explicit string) (string, error) {
	start := strings.TrimSpace(explicit)
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve cwd: %w", err)
		}
		start = cwd
	}
	root, err := vault.FindRoot(start)
	if err != nil {
		return "", err
	}
	// LoadConfig 一遍当 sanity check —— 配置坏的 vault 不应 boot server。
	if _, err := vault.LoadConfig(root); err != nil {
		return "", fmt.Errorf("vault config invalid: %w", err)
	}
	return root, nil
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
	var noReindex bool
	cmd := &cobra.Command{
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
			if result.SourcePage != nil {
				if result.SourcePage.Created {
					fmt.Fprintf(stdout, "source_page: %s\n", result.SourcePage.RelPath)
				} else {
					fmt.Fprintf(stdout, "source_page: %s (existed)\n", result.SourcePage.RelPath)
				}
			}

			// D7: 默认 ingest 完自动 reindex，让新 source page 立即可 query。
			// reindex 失败不阻塞 ingest 主流程（commit 已成功）——
			// 打印 warning 到 stderr，user 可手动重跑 `wikimind page reindex`。
			if !noReindex && !result.Duplicate {
				if res, rerr := service.ReindexWiki(cmd.Context(), db, vaultRoot); rerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"warning: auto reindex failed (run 'wikimind page reindex' manually): %v\n", rerr)
				} else {
					fmt.Fprintf(stdout, "reindexed: %d pages\n", res.Count)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noReindex, "no-reindex", false,
		"skip the automatic 'page reindex' that runs after ingest")
	return cmd
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
	vaultRoot, err := resolveVaultFromCWD()
	if err != nil {
		return "", nil, err
	}
	db, err := index.Open(vaultRoot)
	if err != nil {
		return "", nil, err
	}
	return vaultRoot, db, nil
}

func resolveVaultFromCWD() (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	vaultRoot, err := vault.FindRoot(start)
	if err != nil {
		return "", err
	}
	return vaultRoot, nil
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

func newQueryCommand(stdout io.Writer) *cobra.Command {
	var (
		noIndex bool
		regex   bool
		jsonOut bool
		verbose bool
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "query <text>",
		Short: "Search wiki pages (FTS5 trigram + ripgrep fallback)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultRoot, db, err := openVaultAndIndex()
			if err != nil {
				return err
			}
			defer db.Close()

			hits, err := service.Search(cmd.Context(), db, vaultRoot, args[0], service.SearchOptions{
				NoIndex: noIndex,
				Regex:   regex,
				Limit:   limit,
			})
			if err != nil {
				if errors.Is(err, service.ErrIndexEmpty) {
					return fmt.Errorf("no pages indexed yet — run 'wikimind page reindex' first")
				}
				return err
			}

			if jsonOut {
				return printQueryJSON(stdout, hits)
			}
			printQueryHuman(stdout, hits, verbose)
			return nil
		},
	}
	cmd.Flags().BoolVar(&noIndex, "no-index", false, "bypass SQLite index and use ripgrep")
	cmd.Flags().BoolVar(&regex, "regex", false, "treat query as regex (uses ripgrep)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit NDJSON (one hit per line)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "include BM25 score / source in output")
	cmd.Flags().IntVar(&limit, "limit", 20, "max hits to return")
	return cmd
}

// printQueryHuman 输出对人友好的两行式列表：
//
//	<id> [type] <title>
//	  <snippet>            (snippet 为空时省略)
//	  score=... source=... (仅 verbose)
//
// 空结果显式提示，避免静默。
func printQueryHuman(w io.Writer, hits []index.SearchHit, verbose bool) {
	if len(hits) == 0 {
		fmt.Fprintln(w, "no matches")
		return
	}
	for _, h := range hits {
		fmt.Fprintf(w, "%s [%s] %s\n", h.PageID, h.Type, h.Title)
		if snip := strings.TrimSpace(h.Snippet); snip != "" {
			fmt.Fprintf(w, "  %s\n", snip)
		}
		if verbose {
			fmt.Fprintf(w, "  score=%g source=%s\n", h.Score, h.Source)
		}
	}
}

// printQueryJSON 输出 NDJSON：每行一个 SearchHit 序列化对象。
// 兼容 stream 消费方（agent / jq -c）。
func printQueryJSON(w io.Writer, hits []index.SearchHit) error {
	enc := json.NewEncoder(w)
	// NDJSON 一行一个对象——保留默认换行。
	for _, h := range hits {
		if err := enc.Encode(searchHitJSON{
			PageID:  h.PageID,
			Type:    h.Type,
			Title:   h.Title,
			Snippet: h.Snippet,
			Score:   h.Score,
			Source:  h.Source,
		}); err != nil {
			return fmt.Errorf("encode hit %s: %w", h.PageID, err)
		}
	}
	return nil
}

// searchHitJSON 是 SearchHit 的 JSON 投影；显式定义 tag 让字段名稳定，
// 不直接 marshal index.SearchHit 以免内部字段重命名影响 NDJSON 兼容。
type searchHitJSON struct {
	PageID  string  `json:"page_id"`
	Type    string  `json:"type"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
	Source  string  `json:"source"`
}

// newRevertCommand 实现 W1 D6 的 `wikimind revert <seq>`：
//
//  1. NextSeq 反查 origEntry（友好错误）
//  2. FindCommitBySeq 用 commit message "(seq=<N>)" 反找原 commit short SHA
//  3. （除非 --no-confirm）stdin 二次确认
//  4. git revert --no-commit <sha> 应用反向 patch
//  5. commit.Commit 把反向 patch + op=revert log 行放进同一个 seq-tagged commit
func newRevertCommand(stdout io.Writer, stdin io.Reader) *cobra.Command {
	var noConfirm bool
	cmd := &cobra.Command{
		Use:   "revert <seq>",
		Short: "Revert a previous wikimind commit by change-log seq",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			seq, err := strconv.Atoi(strings.TrimSpace(args[0]))
			if err != nil || seq <= 0 {
				return fmt.Errorf("revert: seq must be a positive integer, got %q", args[0])
			}

			start, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve working directory: %w", err)
			}
			vaultRoot, err := vault.FindRoot(start)
			if err != nil {
				return err
			}

			origEntry, err := commit.ReadEntryBySeq(vaultRoot, seq)
			if err != nil {
				if errors.Is(err, commit.ErrSeqNotFound) {
					return fmt.Errorf("revert: no change-log entry for seq=%d", seq)
				}
				return err
			}

			origSHA, err := commit.FindCommitBySeq(cmd.Context(), vaultRoot, seq)
			if err != nil {
				return fmt.Errorf("revert: cannot locate commit for seq=%d: %w", seq, err)
			}

			fmt.Fprintf(stdout, "revert seq=%d (commit=%s op=%s summary=%s)\n",
				seq, origSHA, origEntry.Op, origEntry.Summary)

			if !noConfirm {
				if !confirmYes(stdout, stdin, "proceed? [y/N]: ") {
					fmt.Fprintln(stdout, "aborted")
					return nil
				}
			}

			revertedPaths, err := commit.GitRevertNoCommit(cmd.Context(), vaultRoot, origSHA)
			if err != nil {
				return fmt.Errorf("revert: git revert failed: %w", err)
			}

			summary := fmt.Sprintf("revert seq=%d (orig %s)", seq, origEntry.Op)
			logEntry, err := commit.Commit(cmd.Context(), vaultRoot, "revert", summary, revertedPaths)
			if err != nil {
				return fmt.Errorf("revert: write change log: %w", err)
			}

			fmt.Fprintf(stdout, "reverted: %s -> %s (new seq=%d)\n",
				origSHA, logEntry.GitSHA, logEntry.Seq)
			return nil
		},
	}
	cmd.Flags().BoolVar(&noConfirm, "no-confirm", false, "skip stdin confirmation prompt")
	return cmd
}

// confirmYes 从 stdin 读一行，仅 "y" / "yes"（大小写不敏感）视为同意。
// stdin 不可读（CI 无 tty）时返回 false——配 --no-confirm 一起用。
func confirmYes(stdout io.Writer, stdin io.Reader, prompt string) bool {
	if stdin == nil {
		return false
	}
	fmt.Fprint(stdout, prompt)
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
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
