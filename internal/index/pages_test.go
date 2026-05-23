package index

import (
	"context"
	"database/sql"
	"testing"
)

func TestUpsertAndGetPage(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)

	row := &PageRow{
		ID:            "cl-2026-05-21-001",
		Type:          "claim",
		Path:          "wiki/claims/foo.md",
		Title:         "Foo claim",
		Body:          "body text 值钱",
		Confidence:    sql.NullFloat64{Float64: 0.7, Valid: true},
		Status:        "supported",
		SchemaVersion: "1.0",
		CreatedBy:     "claude-code",
		UpdatedBy:     "claude-code",
		CreatedAt:     1716480000,
		UpdatedAt:     1716480100,
		Frontmatter:   `{"id":"cl-2026-05-21-001"}`,
	}
	if err := UpsertPage(ctx, db, row); err != nil {
		t.Fatalf("UpsertPage: %v", err)
	}

	got, err := GetPageByID(ctx, db, row.ID)
	if err != nil {
		t.Fatalf("GetPageByID: %v", err)
	}
	if got == nil {
		t.Fatal("page missing after upsert")
	}
	if got.Title != row.Title || got.Type != row.Type ||
		got.Path != row.Path || got.Body != row.Body ||
		got.SchemaVersion != row.SchemaVersion ||
		!got.Confidence.Valid || got.Confidence.Float64 != 0.7 ||
		got.Status != row.Status ||
		got.Frontmatter != row.Frontmatter {
		t.Fatalf("row mismatch: %+v want %+v", got, row)
	}
}

func TestUpsertPageIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)

	row := &PageRow{
		ID: "cl-x", Type: "claim", Path: "wiki/claims/x.md",
		Title: "X", Body: "v1", SchemaVersion: "1.0",
	}
	if err := UpsertPage(ctx, db, row); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	row.Body = "v2"
	row.Title = "X updated"
	if err := UpsertPage(ctx, db, row); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := GetPageByID(ctx, db, row.ID)
	if err != nil {
		t.Fatalf("GetPageByID: %v", err)
	}
	if got.Body != "v2" || got.Title != "X updated" {
		t.Fatalf("upsert did not overwrite: %+v", got)
	}
}

func TestUpsertPageEmptyID(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)
	err := UpsertPage(ctx, db, &PageRow{
		ID: "", Type: "claim", Path: "wiki/claims/y.md",
		Title: "Y", SchemaVersion: "1.0",
	})
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestListPagesFilterByType(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)

	seeds := []*PageRow{
		{ID: "cl-1", Type: "claim", Path: "wiki/claims/a.md", Title: "A", SchemaVersion: "1.0"},
		{ID: "cl-2", Type: "claim", Path: "wiki/claims/b.md", Title: "B", SchemaVersion: "1.0"},
		{ID: "en-1", Type: "entity", Path: "wiki/entities/c.md", Title: "C", SchemaVersion: "1.0"},
	}
	for _, r := range seeds {
		if err := UpsertPage(ctx, db, r); err != nil {
			t.Fatalf("seed %s: %v", r.ID, err)
		}
	}

	got, err := ListPages(ctx, db, "claim")
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("claim count = %d, want 2", len(got))
	}
	for _, p := range got {
		if p.Type != "claim" {
			t.Fatalf("non-claim leaked into filter: %+v", p)
		}
	}

	all, err := ListPages(ctx, db, "")
	if err != nil {
		t.Fatalf("ListPages all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("all count = %d, want 3", len(all))
	}
}

func TestGetPageByIDNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)
	got, err := GetPageByID(ctx, db, "missing")
	if err != nil {
		t.Fatalf("err = %v, want nil for not-found", err)
	}
	if got != nil {
		t.Fatalf("got = %+v, want nil", got)
	}
}

func TestPagesFTSSyncFromTriggers(t *testing.T) {
	// Confirms pages_fts is populated automatically by INSERT trigger and that
	// CJK trigram tokenization actually splits the body so substring search works
	// (D4 prerequisite for D5 query command).
	ctx := context.Background()
	db := openTempDB(t)

	if err := UpsertPage(ctx, db, &PageRow{
		ID: "cl-9", Type: "claim", Path: "wiki/claims/cjk.md",
		Title: "Wiki 是一个 compounding artifact",
		Body:  "每一次 ingest、每一次 query 都让 wiki 更值钱。",
		SchemaVersion: "1.0",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// trigram requires query length >= 3 chars; "更值钱" is 3 CJK chars (>= 3 codepoints).
	queries := []string{"更值钱", "compounding", "ingest"}
	for _, q := range queries {
		var got int
		if err := db.SQL().QueryRow(
			`SELECT COUNT(*) FROM pages_fts WHERE pages_fts MATCH ?`, q,
		).Scan(&got); err != nil {
			t.Fatalf("fts query %q: %v", q, err)
		}
		if got != 1 {
			t.Fatalf("fts MATCH %q = %d, want 1 (trigger not syncing pages_fts)", q, got)
		}
	}

	// Update title → DELETE+INSERT trigger must keep fts in sync.
	if err := UpsertPage(ctx, db, &PageRow{
		ID: "cl-9", Type: "claim", Path: "wiki/claims/cjk.md",
		Title: "Different title",
		Body:  "completely different body content",
		SchemaVersion: "1.0",
	}); err != nil {
		t.Fatalf("update upsert: %v", err)
	}
	var stale int
	if err := db.SQL().QueryRow(
		`SELECT COUNT(*) FROM pages_fts WHERE pages_fts MATCH ?`, "更值钱",
	).Scan(&stale); err != nil {
		t.Fatalf("fts after update: %v", err)
	}
	if stale != 0 {
		t.Fatalf("stale fts row after update: %d (expected 0)", stale)
	}
}

func TestMigration0002UpDownIdempotent(t *testing.T) {
	root := t.TempDir()
	db, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// 0002 must have created pages + pages_fts.
	assertTableExists(t, db.SQL(), "pages")
	if !virtualTableExists(t, db.SQL(), "pages_fts") {
		t.Fatal("pages_fts missing")
	}

	// Re-open should be no-op (goose tracks version).
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	db2, err := Open(root)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	t.Cleanup(func() { _ = db2.Close() })
	assertTableExists(t, db2.SQL(), "pages")
}

// --- helpers ---

func openTempDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func virtualTableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var got string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, name,
	).Scan(&got)
	if err != nil {
		return false
	}
	return got == name
}
