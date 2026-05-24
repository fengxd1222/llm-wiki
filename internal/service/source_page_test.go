package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

func TestEnsureSourcePageCreatesFileWithFrontmatterTitle(t *testing.T) {
	vaultRoot := newTestVault(t)
	rawRel := "raw/inbox/karpathy-demo.md"
	rawAbs := filepath.Join(vaultRoot, filepath.FromSlash(rawRel))
	if err := os.MkdirAll(filepath.Dir(rawAbs), 0o755); err != nil {
		t.Fatalf("mkdir raw/inbox: %v", err)
	}
	body := []byte("---\ntitle: \"Karpathy 的 LLM 笔记\"\n---\n\n# 原文标题\n\n正文内容\n")
	if err := os.WriteFile(rawAbs, body, 0o644); err != nil {
		t.Fatalf("write raw: %v", err)
	}

	res, err := EnsureSourcePage(vaultRoot, rawRel)
	if err != nil {
		t.Fatalf("EnsureSourcePage: %v", err)
	}
	if !res.Created {
		t.Fatalf("Created = false, want true on first call")
	}
	if res.RelPath != "wiki/sources/karpathy-demo.md" {
		t.Fatalf("RelPath = %q, want wiki/sources/karpathy-demo.md", res.RelPath)
	}
	if res.Title != "Karpathy 的 LLM 笔记" {
		t.Fatalf("Title = %q, want Karpathy 的 LLM 笔记", res.Title)
	}

	parsed, err := ParsePage(res.AbsPath)
	if err != nil {
		t.Fatalf("ParsePage source page: %v", err)
	}
	if got := parsed.Frontmatter["id"]; got != "karpathy-demo" {
		t.Fatalf("id = %v, want karpathy-demo", got)
	}
	if got := parsed.Frontmatter["type"]; got != "source" {
		t.Fatalf("type = %v, want source", got)
	}
	if got := parsed.Frontmatter["title"]; got != "Karpathy 的 LLM 笔记" {
		t.Fatalf("title = %v, want Karpathy 的 LLM 笔记", got)
	}
	if got := parsed.Frontmatter["source_path"]; got != rawRel {
		t.Fatalf("source_path = %v, want %s", got, rawRel)
	}
	// ingested_at 应是 RFC3339 UTC。
	ingestedAt, ok := parsed.Frontmatter["ingested_at"].(string)
	if !ok || ingestedAt == "" {
		t.Fatalf("ingested_at missing or wrong type: %v", parsed.Frontmatter["ingested_at"])
	}
	if _, err := time.Parse(time.RFC3339, ingestedAt); err != nil {
		t.Fatalf("ingested_at = %q is not RFC3339: %v", ingestedAt, err)
	}
	if !strings.Contains(parsed.Body, "See raw file for full content") {
		t.Fatalf("body missing placeholder, got: %q", parsed.Body)
	}
}

func TestEnsureSourcePageFallsBackToFirstHeading(t *testing.T) {
	vaultRoot := newTestVault(t)
	rawRel := "raw/inbox/no-title.md"
	rawAbs := filepath.Join(vaultRoot, filepath.FromSlash(rawRel))
	if err := os.MkdirAll(filepath.Dir(rawAbs), 0o755); err != nil {
		t.Fatalf("mkdir raw/inbox: %v", err)
	}
	body := []byte("# 仅有标题\n\n正文\n")
	if err := os.WriteFile(rawAbs, body, 0o644); err != nil {
		t.Fatalf("write raw: %v", err)
	}

	res, err := EnsureSourcePage(vaultRoot, rawRel)
	if err != nil {
		t.Fatalf("EnsureSourcePage: %v", err)
	}
	if res.Title != "仅有标题" {
		t.Fatalf("Title = %q, want 仅有标题", res.Title)
	}
}

func TestEnsureSourcePageFallsBackToBasename(t *testing.T) {
	vaultRoot := newTestVault(t)
	rawRel := "raw/inbox/lonely.md"
	rawAbs := filepath.Join(vaultRoot, filepath.FromSlash(rawRel))
	if err := os.MkdirAll(filepath.Dir(rawAbs), 0o755); err != nil {
		t.Fatalf("mkdir raw/inbox: %v", err)
	}
	if err := os.WriteFile(rawAbs, []byte("无 frontmatter，无 heading，仅正文。\n"), 0o644); err != nil {
		t.Fatalf("write raw: %v", err)
	}

	res, err := EnsureSourcePage(vaultRoot, rawRel)
	if err != nil {
		t.Fatalf("EnsureSourcePage: %v", err)
	}
	if res.Title != "lonely" {
		t.Fatalf("Title = %q, want lonely (basename fallback)", res.Title)
	}
}

func TestEnsureSourcePageIdempotent(t *testing.T) {
	vaultRoot := newTestVault(t)
	rawRel := "raw/inbox/idem.md"
	rawAbs := filepath.Join(vaultRoot, filepath.FromSlash(rawRel))
	if err := os.MkdirAll(filepath.Dir(rawAbs), 0o755); err != nil {
		t.Fatalf("mkdir raw/inbox: %v", err)
	}
	if err := os.WriteFile(rawAbs, []byte("# 一"), 0o644); err != nil {
		t.Fatalf("write raw: %v", err)
	}

	first, err := EnsureSourcePage(vaultRoot, rawRel)
	if err != nil {
		t.Fatalf("first EnsureSourcePage: %v", err)
	}
	if !first.Created {
		t.Fatalf("first call Created = false, want true")
	}

	// 人手编辑 source page；第二次调用必须保留这次编辑。
	custom := []byte("---\nid: idem\ntype: source\ntitle: 我改过\n---\n\n# 用户编辑\n")
	if err := os.WriteFile(first.AbsPath, custom, 0o644); err != nil {
		t.Fatalf("rewrite source page: %v", err)
	}

	second, err := EnsureSourcePage(vaultRoot, rawRel)
	if err != nil {
		t.Fatalf("second EnsureSourcePage: %v", err)
	}
	if second.Created {
		t.Fatalf("second call Created = true, want false (idempotent)")
	}
	got, err := os.ReadFile(first.AbsPath)
	if err != nil {
		t.Fatalf("read after second: %v", err)
	}
	if string(got) != string(custom) {
		t.Fatalf("source page was overwritten:\n%s", got)
	}
}

func TestEnsureSourcePageInvalidVaultRoot(t *testing.T) {
	_, err := EnsureSourcePage("", "raw/inbox/x.md")
	if err == nil {
		t.Fatal("expected error for empty vault root")
	}
}

func TestEnsureSourcePageRawMissing(t *testing.T) {
	vaultRoot := newTestVault(t)
	_, err := EnsureSourcePage(vaultRoot, "raw/inbox/does-not-exist.md")
	if err == nil {
		t.Fatal("expected error for missing raw file")
	}
}

// TestIngestFileWritesSourcePageAndCommits 端到端验证 ingest 现在会
// 1) 生成 wiki/sources/<id>.md
// 2) 把 raw + source page 一起 git commit
func TestIngestFileWritesSourcePageAndCommits(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	srcPath := filepath.Join(t.TempDir(), "wiki-cookbook.md")
	body := []byte("---\ntitle: \"Wiki 烹饪手册\"\n---\n\n# Wiki 烹饪手册\n\n第一道菜：claim 抽取。\n")
	if err := os.WriteFile(srcPath, body, 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	res, err := IngestFile(ctx, db, vaultRoot, srcPath)
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	if res.SourcePage == nil {
		t.Fatal("SourcePage is nil")
	}
	if !res.SourcePage.Created {
		t.Fatal("SourcePage.Created = false, want true on first ingest")
	}
	if res.SourcePage.RelPath != "wiki/sources/wiki-cookbook.md" {
		t.Fatalf("SourcePage.RelPath = %q, want wiki/sources/wiki-cookbook.md", res.SourcePage.RelPath)
	}
	sourcePageAbs := filepath.Join(vaultRoot, filepath.FromSlash(res.SourcePage.RelPath))
	if _, err := os.Stat(sourcePageAbs); err != nil {
		t.Fatalf("source page file missing: %v", err)
	}

	// page 应能被 reindex 拿到。
	if _, err := ReindexWiki(ctx, db, vaultRoot); err != nil {
		t.Fatalf("ReindexWiki: %v", err)
	}
	row, err := index.GetPageByID(ctx, db, "wiki-cookbook")
	if err != nil {
		t.Fatalf("GetPageByID: %v", err)
	}
	if row == nil {
		t.Fatal("source page row missing after reindex")
	}
	if row.Type != "source" {
		t.Fatalf("page Type = %q, want source", row.Type)
	}
	if row.Title != "Wiki 烹饪手册" {
		t.Fatalf("page Title = %q, want Wiki 烹饪手册", row.Title)
	}
}

func TestIngestFileSourcePageIdempotentOnSecondIngest(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	srcPath := filepath.Join(t.TempDir(), "dupe.md")
	if err := os.WriteFile(srcPath, []byte("# Once"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	first, err := IngestFile(ctx, db, vaultRoot, srcPath)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first.SourcePage == nil || !first.SourcePage.Created {
		t.Fatalf("first ingest source page not created: %+v", first.SourcePage)
	}

	// 用户对 source page 加 note。
	abs := first.SourcePage.AbsPath
	original, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	edited := append(original, []byte("\n用户私人 note\n")...)
	if err := os.WriteFile(abs, edited, 0o644); err != nil {
		t.Fatalf("user-edit: %v", err)
	}

	// 第二次 ingest 同一文件 → duplicate 路径，source page 不变。
	second, err := IngestFile(ctx, db, vaultRoot, srcPath)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if !second.Duplicate {
		t.Fatalf("second ingest should be Duplicate")
	}
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read after second ingest: %v", err)
	}
	if string(got) != string(edited) {
		t.Fatalf("user edit clobbered:\n%s", got)
	}
}
