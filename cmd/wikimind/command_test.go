package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/commit"
	"github.com/fengxd1222/llm-wiki/internal/vault"
	worktreepkg "github.com/fengxd1222/llm-wiki/internal/worktree"
)

func TestInitAndStatusCommands(t *testing.T) {
	root := filepath.Join(t.TempDir(), "knowledge")

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"init", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "initialized: "+root) {
		t.Fatalf("init output = %q, want initialized root", out.String())
	}

	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"status", filepath.Join(root, "wiki", "topics")})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status Execute() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"vault: " + root,
		"schema_version: 1.0",
		"raw_files: 0",
		"wiki_pages: 2",
		"claims: 0",
		"git_status: dirty",
		"config: ok",
		"health: ok",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q:\n%s", want, got)
		}
	}
}

func TestIngestCommand(t *testing.T) {
	tempDir := t.TempDir()
	vaultRoot := filepath.Join(tempDir, "vault")
	if _, err := vault.Init(vaultRoot); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}

	srcPath := filepath.Join(tempDir, "sample.md")
	srcBody := []byte("# Sample\n\nHello WikiMind.\n")
	if err := os.WriteFile(srcPath, srcBody, 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	// chdir into vault so vault.FindRoot picks it up; t.Chdir restores cwd.
	t.Chdir(vaultRoot)

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"ingest", srcPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ingest Execute() error = %v\nout=%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{
		"ingested: raw/inbox/sample.md",
		"sha256: ",
		"size: ",
		"status: pending",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ingest output missing %q:\n%s", want, got)
		}
	}

	// Copy must land in raw/inbox/.
	if _, err := os.Stat(filepath.Join(vaultRoot, "raw", "inbox", "sample.md")); err != nil {
		t.Fatalf("ingested file missing: %v", err)
	}
	entry, err := commit.ReadEntryBySeq(vaultRoot, 1)
	if err != nil {
		t.Fatalf("ingest change-log missing: %v", err)
	}
	if entry.Op != "ingest" || entry.Summary != "raw/inbox/sample.md" {
		t.Fatalf("change-log entry = %+v, want ingest raw/inbox/sample.md", entry)
	}
	if _, err := commit.FindCommitBySeq(cmd.Context(), vaultRoot, 1); err != nil {
		t.Fatalf("ingest commit missing seq=1: %v", err)
	}

	// Second ingest of same file → duplicate marker, no second copy under a different name.
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"ingest", srcPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("second ingest error = %v\nout=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "duplicate: raw/inbox/sample.md") {
		t.Fatalf("second ingest output missing duplicate marker:\n%s", out.String())
	}
	nextSeq, err := commit.NextSeq(vaultRoot)
	if err != nil {
		t.Fatalf("NextSeq after duplicate: %v", err)
	}
	if nextSeq != 2 {
		t.Fatalf("NextSeq after duplicate = %d, want 2", nextSeq)
	}
}

// TestIngestCommandAutoReindexAndQuery 端到端验证 W1 D7 出口 demo flow：
// init → ingest（自动 reindex）→ query 命中。
// 用中文 frontmatter title 覆盖 CJK trigram + source page 联动。
func TestIngestCommandAutoReindexAndQuery(t *testing.T) {
	tempDir := t.TempDir()
	vaultRoot := filepath.Join(tempDir, "vault")
	if _, err := vault.Init(vaultRoot); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}

	srcPath := filepath.Join(tempDir, "karpathy-demo.md")
	body := []byte("---\ntitle: \"Karpathy 的 LLM 笔记\"\n---\n\n# Karpathy 的 LLM 笔记\n\n每一次 ingest 都让 wiki 更值钱。\n")
	if err := os.WriteFile(srcPath, body, 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	t.Chdir(vaultRoot)

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"ingest", srcPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ingest Execute() error = %v\nout=%s", err, out.String())
	}
	ingestOut := out.String()
	for _, want := range []string{
		"ingested: raw/inbox/karpathy-demo.md",
		"source_page: wiki/sources/karpathy-demo.md",
		"reindexed:",
	} {
		if !strings.Contains(ingestOut, want) {
			t.Fatalf("ingest output missing %q:\n%s", want, ingestOut)
		}
	}
	// source page 文件应存在。
	if _, err := os.Stat(filepath.Join(vaultRoot, "wiki", "sources", "karpathy-demo.md")); err != nil {
		t.Fatalf("source page missing: %v", err)
	}

	// query 中文 title 必须命中 source page（无需手动 reindex）。
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"query", "Karpathy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("query Execute() error = %v\nout=%s", err, out.String())
	}
	queryOut := out.String()
	for _, want := range []string{"karpathy-demo", "[source]"} {
		if !strings.Contains(queryOut, want) {
			t.Fatalf("query output missing %q:\n%s", want, queryOut)
		}
	}
}

// TestIngestCommandNoReindex 验证 --no-reindex flag 跳过自动 reindex。
func TestIngestCommandNoReindex(t *testing.T) {
	tempDir := t.TempDir()
	vaultRoot := filepath.Join(tempDir, "vault")
	if _, err := vault.Init(vaultRoot); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}
	srcPath := filepath.Join(tempDir, "skip.md")
	if err := os.WriteFile(srcPath, []byte("# Skip\n"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	t.Chdir(vaultRoot)

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"ingest", srcPath, "--no-reindex"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ingest Execute() error = %v\nout=%s", err, out.String())
	}
	ingestOut := out.String()
	if !strings.Contains(ingestOut, "source_page: wiki/sources/skip.md") {
		t.Fatalf("expected source_page in output:\n%s", ingestOut)
	}
	if strings.Contains(ingestOut, "reindexed:") {
		t.Fatalf("--no-reindex did not suppress reindex output:\n%s", ingestOut)
	}

	// 未 reindex → query 应给出"先 reindex"提示，而不是命中。
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"query", "Skip"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("query against un-reindexed vault should error, got: %s", out.String())
	}
}

// TestW1DemoWalkthroughCISmokeTest 跑 W1 出口 demo 全套：
// init → ingest（中英混排 raw）→ query 命中 → revert → 内容消失。
// 此测试必须在 CI 5 OS 全跑通，作为 W1 出口的回归护栏。
func TestW1DemoWalkthroughCISmokeTest(t *testing.T) {
	tempDir := t.TempDir()
	vaultRoot := filepath.Join(tempDir, "demo-vault")

	// step 1: init
	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"init", vaultRoot})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init Execute() error = %v\nout=%s", err, out.String())
	}

	// step 2: 手写一份中英混排 raw markdown
	srcPath := filepath.Join(tempDir, "wiki-cookbook.md")
	body := []byte("---\ntitle: \"WikiMind 烹饪手册\"\n---\n\n# WikiMind 烹饪手册\n\n第一道菜：使用 TypeScript 写 claim 抽取算法。\n")
	if err := os.WriteFile(srcPath, body, 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	t.Chdir(vaultRoot)

	// step 3: ingest
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"ingest", srcPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ingest Execute() error = %v\nout=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "source_page: wiki/sources/wiki-cookbook.md") {
		t.Fatalf("ingest missing source page line:\n%s", out.String())
	}

	// step 4: query CJK substring → 命中 source page
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"query", "烹饪"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("query CJK Execute() error = %v\nout=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "wiki-cookbook") {
		t.Fatalf("query 烹饪 missed source page:\n%s", out.String())
	}

	// step 5: query 英文 token → 命中（混合语言）
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"query", "WikiMind"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("query EN Execute() error = %v\nout=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "wiki-cookbook") {
		t.Fatalf("query WikiMind missed source page:\n%s", out.String())
	}

	// step 6: revert 第一个 commit（seq=1 即 ingest）
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"revert", "1", "--no-confirm"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("revert Execute() error = %v\nout=%s", err, out.String())
	}
	// raw 应被删除
	if _, err := os.Stat(filepath.Join(vaultRoot, "raw", "inbox", "wiki-cookbook.md")); !os.IsNotExist(err) {
		t.Fatalf("raw file after revert err = %v, want not exist", err)
	}
	// source page 也应被删除
	if _, err := os.Stat(filepath.Join(vaultRoot, "wiki", "sources", "wiki-cookbook.md")); !os.IsNotExist(err) {
		t.Fatalf("source page after revert err = %v, want not exist", err)
	}
}

func TestStubCommands(t *testing.T) {
	for _, name := range []string{"review", "lint"} {
		var out bytes.Buffer
		cmd := newRootCommand(&out, &out)
		cmd.SetArgs([]string{name})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s Execute() error = %v", name, err)
		}
		want := "wikimind " + name + ": D1 未实现\n"
		if out.String() != want {
			t.Fatalf("%s output = %q, want %q", name, out.String(), want)
		}
	}
}

func TestWorktreeCommands(t *testing.T) {
	ctx := context.Background()
	vaultRoot := filepath.Join(t.TempDir(), "vault")
	if _, err := vault.Init(vaultRoot); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}
	if err := commit.GitAdd(ctx, vaultRoot, "."); err != nil {
		t.Fatalf("GitAdd: %v", err)
	}
	if _, err := commit.GitCommit(ctx, vaultRoot, "init"); err != nil {
		t.Fatalf("GitCommit: %v", err)
	}
	t.Chdir(vaultRoot)

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"worktree", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("worktree list empty: %v", err)
	}
	if !strings.Contains(out.String(), "no worktrees") {
		t.Fatalf("empty worktree list output = %q", out.String())
	}

	if _, err := worktreepkg.CreateWorktree(ctx, vaultRoot, "codex-cli", "sess-1"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"worktree", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("worktree list: %v", err)
	}
	for _, want := range []string{"wt-codex-cli-sess-1", "codex-cli", "sess-1", "wiki/_worktrees/agent-codex-cli-sess-1"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("worktree list missing %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"worktree", "remove", "codex-cli/sess-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("worktree remove: %v", err)
	}
	if !strings.Contains(out.String(), "removed: codex-cli/sess-1") {
		t.Fatalf("worktree remove output = %q", out.String())
	}

	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"worktree", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("worktree list after remove: %v", err)
	}
	if !strings.Contains(out.String(), "no worktrees") {
		t.Fatalf("worktree list after remove = %q", out.String())
	}
}

func TestRevertCommand(t *testing.T) {
	tempDir := t.TempDir()
	vaultRoot := filepath.Join(tempDir, "vault")
	if _, err := vault.Init(vaultRoot); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}
	srcPath := filepath.Join(tempDir, "sample.md")
	if err := os.WriteFile(srcPath, []byte("# Sample\n"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	t.Chdir(vaultRoot)

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"ingest", srcPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ingest Execute() error = %v\nout=%s", err, out.String())
	}
	rawPath := filepath.Join(vaultRoot, "raw", "inbox", "sample.md")
	if _, err := os.Stat(rawPath); err != nil {
		t.Fatalf("raw file missing after ingest: %v", err)
	}

	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"revert", "1", "--no-confirm"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("revert Execute() error = %v\nout=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "new seq=2") {
		t.Fatalf("revert output missing new seq:\n%s", out.String())
	}
	if _, err := os.Stat(rawPath); !os.IsNotExist(err) {
		t.Fatalf("raw file after revert err = %v, want not exist", err)
	}
	entry1, err := commit.ReadEntryBySeq(vaultRoot, 1)
	if err != nil {
		t.Fatalf("seq=1 log entry missing after revert: %v", err)
	}
	if entry1.Op != "ingest" {
		t.Fatalf("seq=1 Op = %q, want ingest", entry1.Op)
	}
	entry2, err := commit.ReadEntryBySeq(vaultRoot, 2)
	if err != nil {
		t.Fatalf("seq=2 log entry missing: %v", err)
	}
	if entry2.Op != "revert" {
		t.Fatalf("seq=2 Op = %q, want revert", entry2.Op)
	}

	// revert of revert：seq=2 必须指向内容反转 commit，而不是 log-only commit。
	// raw 文件应恢复，且 log 保持 append-only。
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"revert", "2", "--no-confirm"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("revert of revert Execute() error = %v\nout=%s", err, out.String())
	}
	if _, err := os.Stat(rawPath); err != nil {
		t.Fatalf("raw file missing after revert of revert: %v", err)
	}
	entry3, err := commit.ReadEntryBySeq(vaultRoot, 3)
	if err != nil {
		t.Fatalf("seq=3 log entry missing: %v", err)
	}
	if entry3.Op != "revert" {
		t.Fatalf("seq=3 Op = %q, want revert", entry3.Op)
	}
}

// TestQueryCommandRequiresArg 保证 query 已升级为真命令——
// stub 命令零参数会原样打印 "D1 未实现"；query 用 ExactArgs(1) 必须 reject
// 缺参调用并 surface 一个 usage 错误，TestStubCommands 才能与 query 分离。
func TestQueryCommandRequiresArg(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"query"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("query without arg should error, got output: %q", out.String())
	}
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Fatalf("expected 'accepts 1 arg' usage error, got: %v", err)
	}
}

// TestQueryCommandFTS5Path 端到端验证 CLI query：reindex 后用 trigram 命中
// 一个 seeded claim 页，确认输出包含 page id、type 标记和 snippet。
// 这是 query 命令在 cmd 层的唯一 happy-path smoke——细节路由由 service 层
// 单测保证。
func TestQueryCommandFTS5Path(t *testing.T) {
	tempDir := t.TempDir()
	vaultRoot := filepath.Join(tempDir, "vault")
	if _, err := vault.Init(vaultRoot); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}
	t.Chdir(vaultRoot)

	// Seed a claim page with a distinct multi-rune Chinese phrase + English token.
	claimPath := filepath.Join(vaultRoot, "wiki", "claims", "compounding.md")
	if err := os.MkdirAll(filepath.Dir(claimPath), 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	claimBody := `---
id: cl-q-001
type: claim
title: "Wiki 是一个 compounding artifact"
schema_version: "1.0"
---

# compounding

每一次 ingest 都让 wiki 更值钱。
`
	if err := os.WriteFile(claimPath, []byte(claimBody), 0o644); err != nil {
		t.Fatalf("seed claim: %v", err)
	}

	// reindex first so pages_fts has rows.
	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"page", "reindex"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("page reindex: %v\n%s", err, out.String())
	}

	// English query → FTS5 trigram hit.
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"query", "compounding"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("query: %v\n%s", err, out.String())
	}
	got := out.String()
	for _, want := range []string{"cl-q-001", "[claim]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("query output missing %q:\n%s", want, got)
		}
	}
}

// TestQueryCommandEmptyIndexFriendlyHint 验证空 vault 跑 query 提示用户先
// reindex，而不是裸 SQL 错误。
func TestQueryCommandEmptyIndexFriendlyHint(t *testing.T) {
	tempDir := t.TempDir()
	vaultRoot := filepath.Join(tempDir, "vault")
	if _, err := vault.Init(vaultRoot); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}
	t.Chdir(vaultRoot)

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"query", "anything"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("query against empty index should error, got output: %q", out.String())
	}
	if !strings.Contains(err.Error(), "wikimind page reindex") {
		t.Fatalf("expected reindex hint in error, got: %v", err)
	}
}

func TestPageCommands(t *testing.T) {
	tempDir := t.TempDir()
	vaultRoot := filepath.Join(tempDir, "vault")
	if _, err := vault.Init(vaultRoot); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}

	// Empty state: list should print friendly hint before reindex.
	t.Chdir(vaultRoot)
	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"page", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("page list (empty) Execute(): %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "no pages indexed yet") {
		t.Fatalf("empty list missing hint:\n%s", out.String())
	}

	// Seed a claim page + an entity page.
	claimPath := filepath.Join(vaultRoot, "wiki", "claims", "wiki-is-compounding.md")
	if err := os.MkdirAll(filepath.Dir(claimPath), 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	claimBody := `---
id: cl-2026-05-21-001
type: claim
title: "Wiki 是一个 compounding artifact"
schema_version: "1.0"
confidence: 0.92
status: supported
---

# Wiki is compounding

[[karpathy]]
`
	if err := os.WriteFile(claimPath, []byte(claimBody), 0o644); err != nil {
		t.Fatalf("seed claim: %v", err)
	}
	entityPath := filepath.Join(vaultRoot, "wiki", "entities", "karpathy.md")
	if err := os.MkdirAll(filepath.Dir(entityPath), 0o755); err != nil {
		t.Fatalf("mkdir entities: %v", err)
	}
	entityBody := `---
id: en-2026-05-21-001
type: entity
title: "Andrej Karpathy"
schema_version: "1.0"
---

# Karpathy
`
	if err := os.WriteFile(entityPath, []byte(entityBody), 0o644); err != nil {
		t.Fatalf("seed entity: %v", err)
	}

	// reindex
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"page", "reindex"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("page reindex Execute(): %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "indexed ") || !strings.Contains(out.String(), " pages") {
		t.Fatalf("reindex output missing 'indexed N pages':\n%s", out.String())
	}

	// list after reindex must contain both ids.
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"page", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("page list: %v\n%s", err, out.String())
	}
	listOut := out.String()
	for _, want := range []string{"cl-2026-05-21-001", "en-2026-05-21-001", "## claim", "## entity"} {
		if !strings.Contains(listOut, want) {
			t.Fatalf("page list missing %q:\n%s", want, listOut)
		}
	}

	// list --type entity must drop the claim.
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"page", "list", "--type", "entity"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("page list --type: %v\n%s", err, out.String())
	}
	filtered := out.String()
	if !strings.Contains(filtered, "en-2026-05-21-001") {
		t.Fatalf("filtered list missing entity:\n%s", filtered)
	}
	if strings.Contains(filtered, "cl-2026-05-21-001") {
		t.Fatalf("filtered list leaked claim:\n%s", filtered)
	}

	// show by id
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"page", "show", "cl-2026-05-21-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("page show: %v\n%s", err, out.String())
	}
	showOut := out.String()
	for _, want := range []string{
		"id: cl-2026-05-21-001",
		"type: claim",
		"title: Wiki 是一个 compounding artifact",
		"schema_version: 1.0",
		"# Wiki is compounding",
	} {
		if !strings.Contains(showOut, want) {
			t.Fatalf("page show missing %q:\n%s", want, showOut)
		}
	}

	// show on missing id → error.
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"page", "show", "no-such-id"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("page show missing id should error, got output:\n%s", out.String())
	}
}

// TestMcpServeCommandRegistered 不实跑 stdio——只验证 `wikimind mcp serve`
// 命令存在、--vault flag 暴露、help 文本指明 D8+D9 read tools。
// stdio 通路无法 mock os.Stdin/os.Stdout，端到端验证留 docs/demo/mcp-inspector.md
// 手动执行。
func TestMcpServeCommandRegistered(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"mcp", "serve", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mcp serve --help error = %v\nout=%s", err, out.String())
	}
	help := out.String()
	for _, want := range []string{
		"WikiMind MCP server",
		"wiki_info",
		"read_page",
		"read_raw",
		"list_index",
		"search",
		"read_raw_anchor",
		"read_claim",
		"graph_neighbors",
		"get_history",
		"--vault",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("mcp serve help missing %q:\n%s", want, help)
		}
	}
}

// TestMcpServeCommandWithoutVault 没找到 vault 时 mcp serve 必须 surface
// 一个友好错误而不是 silently boot 一个无源 server。
func TestMcpServeCommandWithoutVault(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"mcp", "serve", "--vault", tempDir})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("mcp serve without vault should error, got: %s", out.String())
	}
	if !strings.Contains(err.Error(), "no WikiMind vault found") {
		t.Fatalf("expected 'no WikiMind vault found' in error, got: %v", err)
	}
}
