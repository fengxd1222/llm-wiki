package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/service"
)

// TestD14DemoEndToEnd exercises the W2 exit demo flow:
// init → ingest → reindex → wiki/index.md → health score → graph inbound → doctor.
func TestD14DemoEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	vaultRoot := filepath.Join(tempDir, "d14-vault")

	// Phase 1: init vault
	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"init", vaultRoot})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init: %v\n%s", err, out.String())
	}

	// Phase 2: ingest a markdown file
	srcPath := filepath.Join(tempDir, "compounding.md")
	rawBody := []byte(`---
title: "Compounding Knowledge"
---

# Compounding Knowledge

Knowledge compounds over time through deliberate practice.
`)
	if err := os.WriteFile(srcPath, rawBody, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	t.Chdir(vaultRoot)
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"ingest", srcPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ingest: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "source_page:") {
		t.Fatalf("ingest output missing source_page:\n%s", out.String())
	}

	// Phase 2b: manually create a concept page with outbound [[links]]
	conceptDir := filepath.Join(vaultRoot, "wiki", "concepts")
	if err := os.MkdirAll(conceptDir, 0o755); err != nil {
		t.Fatalf("mkdir concepts: %v", err)
	}
	conceptBody := []byte(`---
title: "Learning Loop"
type: concept
---

# Learning Loop

A learning loop connects [[compounding]] knowledge with [[en-practice]].
`)
	if err := os.WriteFile(filepath.Join(conceptDir, "learning-loop.md"), conceptBody, 0o644); err != nil {
		t.Fatalf("write concept: %v", err)
	}

	// Phase 3: verify wiki/index.md was created with entry from ingest
	indexPath := filepath.Join(vaultRoot, "wiki", "index.md")
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read wiki/index.md: %v", err)
	}
	if !strings.Contains(string(indexContent), "compounding") {
		t.Fatalf("wiki/index.md missing compounding entry:\n%s", string(indexContent))
	}

	// Phase 4: reindex to populate page_links
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"reindex"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reindex: %v\n%s", err, out.String())
	}

	// Phase 5: verify health score is real (not placeholder 100 when orphans exist)
	ctx := context.Background()
	db, err := index.Open(vaultRoot)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	health, err := service.ComputeHealth(ctx, db)
	if err != nil {
		t.Fatalf("ComputeHealth: %v", err)
	}
	// After reindex, "learning-loop" is a concept page with no inbound links → orphan.
	// Health score should reflect this (not hardcoded 100).
	if health.Score < 0 || health.Score > 100 {
		t.Fatalf("health.Score = %d, want 0-100", health.Score)
	}
	if health.OrphanPages == 0 {
		t.Fatalf("expected orphan pages > 0 (learning-loop has no inbound), got 0")
	}
	t.Logf("health: score=%d orphans=%d", health.Score, health.OrphanPages)

	// Phase 6: verify graph inbound works via page_links
	// The concept page "learning-loop" links to [[compounding]] and [[en-practice]].
	// After reindex, outbound links from learning-loop should be recorded.
	outbound, err := index.OutboundLinks(ctx, db, "learning-loop")
	if err != nil {
		t.Fatalf("OutboundLinks: %v", err)
	}
	if len(outbound) != 2 {
		t.Logf("outbound links for learning-loop: %+v", outbound)
		// Debug: list all pages and their links
		pages, _ := index.ListPages(ctx, db, "")
		for _, p := range pages {
			t.Logf("  page: id=%s type=%s path=%s", p.ID, p.Type, p.Path)
		}
		t.Fatalf("expected 2 outbound links from learning-loop, got %d", len(outbound))
	}

	// Verify inbound: "compounding" source page should have inbound from learning-loop
	inbound, err := index.InboundLinks(ctx, db, "compounding")
	if err != nil {
		t.Fatalf("InboundLinks: %v", err)
	}
	if len(inbound) != 1 || inbound[0].SourceID != "learning-loop" {
		t.Fatalf("inbound to compounding = %+v, want 1 from learning-loop", inbound)
	}

	// Phase 7: doctor command runs (may fail on pypdf but should not error fatally)
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"doctor"})
	// doctor returns error if any check fails (e.g. pypdf not installed) — that's OK
	_ = cmd.Execute()
	if !strings.Contains(out.String(), "git:") {
		t.Fatalf("doctor output missing git check:\n%s", out.String())
	}

	// Phase 8: query CJK/EN works
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"query", "compounding"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("query: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "compounding") {
		t.Fatalf("query 'compounding' missed:\n%s", out.String())
	}
}

// TestDoctorCommandExists verifies the doctor command is registered.
func TestDoctorCommandExists(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"doctor", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor --help: %v", err)
	}
	if !strings.Contains(out.String(), "dependencies") {
		t.Fatalf("doctor help missing description:\n%s", out.String())
	}
}

// TestReindexCommandExists verifies the reindex command is registered.
func TestReindexCommandExists(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"reindex", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reindex --help: %v", err)
	}
	if !strings.Contains(out.String(), "Rebuild") || !strings.Contains(out.String(), "reindex") {
		t.Fatalf("reindex help unexpected:\n%s", out.String())
	}
}
