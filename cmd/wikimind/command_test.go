package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/vault"
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
}

func TestStubCommands(t *testing.T) {
	for _, name := range []string{"review", "lint", "revert"} {
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
