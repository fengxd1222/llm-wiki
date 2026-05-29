package lint

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

func openTestDB(t *testing.T, vaultRoot string) *index.DB {
	t.Helper()
	db, err := index.Open(vaultRoot)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func setupLintVault(t *testing.T) (string, *index.DB) {
	t.Helper()
	root := t.TempDir()
	// Create wiki directory structure.
	for _, d := range []string{"wiki/claims", "wiki/concepts", "wiki/sources"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	// Create wiki/index.md
	if err := os.WriteFile(filepath.Join(root, "wiki", "index.md"), []byte("# Index\n"), 0o644); err != nil {
		t.Fatalf("write index.md: %v", err)
	}
	db := openTestDB(t, root)
	return root, db
}

func TestOrphanRuleFindsOrphans(t *testing.T) {
	root, db := setupLintVault(t)
	ctx := context.Background()

	// Insert a claim page with no links.
	_ = index.UpsertPage(ctx, db, &index.PageRow{ID: "cl-orphan", Type: "claim", Path: "wiki/claims/orphan.md", Title: "Orphan"})

	rule := &OrphanRule{}
	findings := rule.Run(ctx, root, db)
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].PageID != "cl-orphan" {
		t.Fatalf("finding.PageID = %s, want cl-orphan", findings[0].PageID)
	}
}

func TestOrphanRuleSkipsLinkedPages(t *testing.T) {
	root, db := setupLintVault(t)
	ctx := context.Background()

	_ = index.UpsertPage(ctx, db, &index.PageRow{ID: "cl-a", Type: "claim", Path: "wiki/claims/a.md", Title: "A"})
	_ = index.UpsertPage(ctx, db, &index.PageRow{ID: "cl-b", Type: "claim", Path: "wiki/claims/b.md", Title: "B"})
	_ = index.InsertPageLink(ctx, db, &index.PageLink{SourceID: "cl-a", TargetID: "cl-b"})

	rule := &OrphanRule{}
	findings := rule.Run(ctx, root, db)
	// cl-a has outbound, cl-b has inbound — neither is orphan.
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want 0 (both linked)", findings)
	}
}

func TestBrokenLinkRuleDetectsMissing(t *testing.T) {
	root, db := setupLintVault(t)
	ctx := context.Background()

	_ = index.UpsertPage(ctx, db, &index.PageRow{ID: "cl-src", Type: "claim", Path: "wiki/claims/src.md", Title: "Src"})
	_ = index.InsertPageLink(ctx, db, &index.PageLink{SourceID: "cl-src", TargetID: "nonexistent"})

	rule := &BrokenLinkRule{}
	findings := rule.Run(ctx, root, db)
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Severity != SeverityError {
		t.Fatalf("severity = %s, want error", findings[0].Severity)
	}
}

func TestBrokenLinkRuleNoFalsePositive(t *testing.T) {
	root, db := setupLintVault(t)
	ctx := context.Background()

	_ = index.UpsertPage(ctx, db, &index.PageRow{ID: "cl-a", Type: "claim", Path: "wiki/claims/a.md", Title: "A"})
	_ = index.UpsertPage(ctx, db, &index.PageRow{ID: "cl-b", Type: "claim", Path: "wiki/claims/b.md", Title: "B"})
	_ = index.InsertPageLink(ctx, db, &index.PageLink{SourceID: "cl-a", TargetID: "cl-b"})

	rule := &BrokenLinkRule{}
	findings := rule.Run(ctx, root, db)
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want 0", findings)
	}
}

func TestSchemaViolationSkipsSystemPages(t *testing.T) {
	root, db := setupLintVault(t)
	ctx := context.Background()

	// System pages (index, log) should not trigger schema_violation.
	_ = index.UpsertPage(ctx, db, &index.PageRow{ID: "index", Type: "unknown", Path: "wiki/index.md", Title: "index"})
	_ = index.UpsertPage(ctx, db, &index.PageRow{ID: "log", Type: "unknown", Path: "wiki/log.md", Title: "log"})

	rule := &SchemaViolationRule{}
	findings := rule.Run(ctx, root, db)
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want 0 (system pages excluded)", findings)
	}
}

func TestSchemaViolationDetectsMissingType(t *testing.T) {
	root, db := setupLintVault(t)
	ctx := context.Background()

	_ = index.UpsertPage(ctx, db, &index.PageRow{ID: "bad-page", Type: "unknown", Path: "wiki/concepts/bad.md", Title: "Bad"})

	rule := &SchemaViolationRule{}
	findings := rule.Run(ctx, root, db)
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].Rule != "schema_violation" {
		t.Fatalf("rule = %s, want schema_violation", findings[0].Rule)
	}
}

func TestRunRulesSummary(t *testing.T) {
	root, db := setupLintVault(t)
	ctx := context.Background()

	// One orphan (warn) + one broken link (error).
	_ = index.UpsertPage(ctx, db, &index.PageRow{ID: "cl-x", Type: "claim", Path: "wiki/claims/x.md", Title: "X"})
	_ = index.InsertPageLink(ctx, db, &index.PageLink{SourceID: "cl-x", TargetID: "missing"})

	findings, summary := RunRules(ctx, root, db, AllRules())
	if summary.Total == 0 {
		t.Fatalf("expected findings, got 0")
	}
	if summary.Errors == 0 {
		t.Fatalf("expected at least 1 error (broken_link)")
	}
	_ = findings
}

// TestRuleCountMatchesAllRules 锁定 RuleCount 与 AllRules 同源——CLI lint
// banner 的规则数从 RuleCount 派生，不再硬编码（F-049）。
func TestRuleCountMatchesAllRules(t *testing.T) {
	if RuleCount() != len(AllRules()) {
		t.Fatalf("RuleCount() = %d, want %d", RuleCount(), len(AllRules()))
	}
	if RuleCount() != 5 {
		t.Fatalf("RuleCount() = %d, want 5 built-in rules", RuleCount())
	}
}

func TestIsSystemPage(t *testing.T) {
	if !isSystemPage("index") {
		t.Fatal("index should be system page")
	}
	if !isSystemPage("log") {
		t.Fatal("log should be system page")
	}
	if isSystemPage("cl-001") {
		t.Fatal("cl-001 should not be system page")
	}
}
