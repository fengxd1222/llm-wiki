package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

func TestParsePageStandardFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.md")
	body := `---
id: cl-2026-05-21-001
type: claim
title: "Wiki 是一个 compounding artifact"
schema_version: "1.0"
confidence: 0.92
status: supported
sources:
  - raw_id: raw/inbox/x.md
    quote_hash: a7f2e3c1
---

# Wiki 是一个 compounding artifact

See [[karpathy]] and [[compounding-artifact|complex link]].
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ParsePage(path)
	if err != nil {
		t.Fatalf("ParsePage: %v", err)
	}
	if got.Frontmatter["id"] != "cl-2026-05-21-001" {
		t.Fatalf("id = %v, want cl-2026-05-21-001", got.Frontmatter["id"])
	}
	if got.Frontmatter["type"] != "claim" {
		t.Fatalf("type = %v", got.Frontmatter["type"])
	}
	if c, ok := got.Frontmatter["confidence"].(float64); !ok || c != 0.92 {
		t.Fatalf("confidence = %v (%T), want 0.92", got.Frontmatter["confidence"], got.Frontmatter["confidence"])
	}
	if len(got.Headings) != 1 || got.Headings[0].Level != 1 ||
		!strings.Contains(got.Headings[0].Text, "compounding") {
		t.Fatalf("headings = %+v", got.Headings)
	}
	if len(got.Outbounds) != 2 ||
		got.Outbounds[0] != "karpathy" ||
		got.Outbounds[1] != "compounding-artifact" {
		t.Fatalf("outbounds = %v, want [karpathy compounding-artifact]", got.Outbounds)
	}
}

func TestParsePageWithoutFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.md")
	body := "# WikiMind Log\n\nLine one.\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ParsePage(path)
	if err != nil {
		t.Fatalf("ParsePage: %v", err)
	}
	if got.Frontmatter != nil {
		t.Fatalf("Frontmatter = %v, want nil", got.Frontmatter)
	}
	if got.Body != body {
		t.Fatalf("Body changed:\n%q", got.Body)
	}
	if len(got.Headings) != 1 || got.Headings[0].Text != "WikiMind Log" {
		t.Fatalf("headings = %+v", got.Headings)
	}
}

func TestParsePageUnclosedFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.md")
	body := "---\nid: foo\ntitle: never closed\n\nbody here\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := ParsePage(path)
	if !errors.Is(err, ErrInvalidFrontmatter) {
		t.Fatalf("err = %v, want ErrInvalidFrontmatter", err)
	}
}

func TestParsePageInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad-yaml.md")
	body := "---\nid: foo\n  bad indentation: ][}{\n---\n\nbody\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := ParsePage(path)
	if !errors.Is(err, ErrInvalidFrontmatter) {
		t.Fatalf("err = %v, want ErrInvalidFrontmatter", err)
	}
}

func TestParsePageMultiLevelHeadings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.md")
	body := `---
id: x
type: concept
title: Hierarchy
schema_version: "1.0"
---

# H1
## H2
### H3
#### H4
##### H5
###### H6
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ParsePage(path)
	if err != nil {
		t.Fatalf("ParsePage: %v", err)
	}
	if len(got.Headings) != 6 {
		t.Fatalf("headings count = %d, want 6: %+v", len(got.Headings), got.Headings)
	}
	for i, h := range got.Headings {
		wantLevel := i + 1
		if h.Level != wantLevel {
			t.Fatalf("Heading %d Level = %d, want %d", i, h.Level, wantLevel)
		}
	}
}

func TestParsePageOutboundDeduplication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dup.md")
	body := `---
id: x
type: concept
title: T
schema_version: "1.0"
---

# T

[[foo]] then [[foo]] again, plus [[bar|alias text]] and [[foo|other alias]].
Nested in code: ` + "`[[baz]]`" + ` should still count (regex naive).

Multi line list:
- [[bar]]
- [[qux]]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ParsePage(path)
	if err != nil {
		t.Fatalf("ParsePage: %v", err)
	}
	want := []string{"foo", "bar", "baz", "qux"}
	if len(got.Outbounds) != len(want) {
		t.Fatalf("outbounds = %v, want %v", got.Outbounds, want)
	}
	for i, id := range want {
		if got.Outbounds[i] != id {
			t.Fatalf("outbounds[%d] = %s, want %s (full = %v)", i, got.Outbounds[i], id, got.Outbounds)
		}
	}
}

func TestParsePageEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.md")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ParsePage(path)
	if err != nil {
		t.Fatalf("ParsePage: %v", err)
	}
	if got.Frontmatter != nil || got.Body != "" || len(got.Headings) != 0 {
		t.Fatalf("unexpected parse: %+v", got)
	}
}

func TestReindexWikiWritesPagesAndIdempotent(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	seedPage(t, vaultRoot, "wiki/claims/wiki-is-compounding.md", `---
id: cl-2026-05-21-001
type: claim
title: "Wiki 是一个 compounding artifact"
schema_version: "1.0"
confidence: 0.92
status: supported
---

# Compounding

[[karpathy]]
`)
	seedPage(t, vaultRoot, "wiki/entities/karpathy.md", `---
id: en-2026-05-21-001
type: entity
title: "Andrej Karpathy"
schema_version: "1.0"
---

# Karpathy

bio.
`)

	res, err := ReindexWiki(ctx, db, vaultRoot)
	if err != nil {
		t.Fatalf("ReindexWiki: %v", err)
	}
	// 2 seeded + 2 initial wiki/index.md + wiki/log.md = 4
	if res.Count < 4 {
		t.Fatalf("Count = %d, want >= 4", res.Count)
	}
	if len(res.Skipped) != 0 {
		t.Fatalf("unexpected Skipped = %v", res.Skipped)
	}

	// Second run must not error nor blow up rows (UPSERT keeps idempotency).
	first, err := index.ListPages(ctx, db, "")
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	res2, err := ReindexWiki(ctx, db, vaultRoot)
	if err != nil {
		t.Fatalf("second ReindexWiki: %v", err)
	}
	if res2.Count != res.Count {
		t.Fatalf("second Count = %d, want %d", res2.Count, res.Count)
	}
	second, err := index.ListPages(ctx, db, "")
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("row count drifted: %d → %d", len(first), len(second))
	}
}

func TestReindexWikiSkipsUnderscoreDirs(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	// wiki/_review/ exists by default — drop a markdown file inside.
	reviewDir := filepath.Join(vaultRoot, "wiki", "_review")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review: %v", err)
	}
	reviewMD := filepath.Join(reviewDir, "draft.md")
	if err := os.WriteFile(reviewMD, []byte("# draft\n"), 0o644); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	// wiki/_worktrees/ may not exist; create one.
	wtDir := filepath.Join(vaultRoot, "wiki", "_worktrees", "agent-x")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtDir, "scratch.md"), []byte("# scratch\n"), 0o644); err != nil {
		t.Fatalf("write scratch: %v", err)
	}

	res, err := ReindexWiki(ctx, db, vaultRoot)
	if err != nil {
		t.Fatalf("ReindexWiki: %v", err)
	}
	rows, err := index.ListPages(ctx, db, "")
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	for _, p := range rows {
		if strings.Contains(p.Path, "_review/") ||
			strings.Contains(p.Path, "_worktrees/") ||
			strings.Contains(p.Path, "_reports/") {
			t.Fatalf("page under _ dir leaked: %+v", p)
		}
	}
	if res.Count != len(rows) {
		t.Fatalf("Count = %d, list = %d", res.Count, len(rows))
	}
}

func TestReindexWikiNilDB(t *testing.T) {
	vaultRoot := newTestVault(t)
	_, err := ReindexWiki(context.Background(), nil, vaultRoot)
	if !errors.Is(err, index.ErrIndexUnavailable) {
		t.Fatalf("err = %v, want ErrIndexUnavailable", err)
	}
}

func TestReindexWikiEmptyVaultRoot(t *testing.T) {
	db := openTestDB(t, newTestVault(t))
	_, err := ReindexWiki(context.Background(), db, "")
	if !errors.Is(err, ErrInvalidVaultRoot) {
		t.Fatalf("err = %v, want ErrInvalidVaultRoot", err)
	}
}

// --- helpers ---

func seedPage(t *testing.T, vaultRoot, relPath, body string) {
	t.Helper()
	abs := filepath.Join(vaultRoot, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
}
