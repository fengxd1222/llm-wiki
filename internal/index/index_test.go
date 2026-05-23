package index

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabaseAndRunsMigrations(t *testing.T) {
	root := t.TempDir()

	db, err := Open(root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := os.Stat(filepath.Join(root, ".wikimind", "index.db")); err != nil {
		t.Fatalf("index.db missing: %v", err)
	}

	// meta + sources 必须由 0001 创建。
	assertTableExists(t, db.SQL(), "meta")
	assertTableExists(t, db.SQL(), "sources")

	// bootstrap row.
	var value string
	if err := db.SQL().QueryRow(
		`SELECT value FROM meta WHERE key = ?`, "schema_bootstrap",
	).Scan(&value); err != nil {
		t.Fatalf("read meta row: %v", err)
	}
	if value != "0001" {
		t.Fatalf("schema_bootstrap = %q, want 0001", value)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	root := t.TempDir()

	first, err := Open(root)
	if err != nil {
		t.Fatalf("Open() first error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close first: %v", err)
	}

	// Second Open against the same vault should re-run goose up no-op.
	second, err := Open(root)
	if err != nil {
		t.Fatalf("Open() second error = %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	// goose version table exists.
	assertTableExists(t, second.SQL(), "goose_db_version")
}

func TestOpenBackupsExistingDatabase(t *testing.T) {
	root := t.TempDir()

	// First Open seeds a real SQLite file.
	first, err := Open(root)
	if err != nil {
		t.Fatalf("Open() first error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close first: %v", err)
	}

	dbPath := filepath.Join(root, ".wikimind", "index.db")
	original, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read seeded db: %v", err)
	}

	// Second Open must back up the existing index.db before running migrations.
	second, err := Open(root)
	if err != nil {
		t.Fatalf("Open() second error = %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	bak, err := os.ReadFile(dbPath + ".bak")
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(bak) != string(original) {
		t.Fatalf("backup content differs from original seed")
	}
}

func TestOpenRejectsEmptyVaultRoot(t *testing.T) {
	if _, err := Open(""); !errors.Is(err, ErrIndexUnavailable) {
		t.Fatalf("Open(\"\") = %v, want ErrIndexUnavailable", err)
	}
}

func TestBeginTxRoundTrip(t *testing.T) {
	root := t.TempDir()
	db, err := Open(root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO meta(key, value) VALUES (?, ?)`, "test_tx", "1",
	); err != nil {
		t.Fatalf("insert in tx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var got string
	if err := db.SQL().QueryRow(
		`SELECT value FROM meta WHERE key = ?`, "test_tx",
	).Scan(&got); err != nil {
		t.Fatalf("post-commit read: %v", err)
	}
	if got != "1" {
		t.Fatalf("value = %q, want 1", got)
	}
}

func TestInsertAndFindSource(t *testing.T) {
	root := t.TempDir()
	db, err := Open(root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	src := &SourceRow{
		RawID:      "raw/inbox/sample.md",
		SHA256:     "deadbeef",
		Size:       42,
		MTime:      1716480000,
		Status:     "pending",
		IngestedAt: 1716480100,
	}
	if err := InsertSource(ctx, db, src); err != nil {
		t.Fatalf("InsertSource: %v", err)
	}

	got, err := FindSourceBySHA256(ctx, db, "deadbeef")
	if err != nil {
		t.Fatalf("FindSourceBySHA256: %v", err)
	}
	if got == nil {
		t.Fatal("FindSourceBySHA256 returned nil, want row")
	}
	if got.RawID != src.RawID || got.SHA256 != src.SHA256 ||
		got.Size != src.Size || got.MTime != src.MTime ||
		got.Status != src.Status {
		t.Fatalf("row mismatch: %+v want %+v", got, src)
	}

	miss, err := FindSourceBySHA256(ctx, db, "cafef00d")
	if err != nil {
		t.Fatalf("FindSourceBySHA256 miss error = %v", err)
	}
	if miss != nil {
		t.Fatalf("FindSourceBySHA256 miss returned row %+v", miss)
	}
}

func TestInsertSourceConflictReturnsError(t *testing.T) {
	root := t.TempDir()
	db, err := Open(root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	src := &SourceRow{
		RawID: "raw/inbox/dup.md", SHA256: "abc", Size: 1, MTime: 1,
		Status: "pending", IngestedAt: 1,
	}
	if err := InsertSource(ctx, db, src); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := InsertSource(ctx, db, src); err == nil {
		t.Fatal("duplicate insert should error (PK conflict)")
	}
}

func assertTableExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var got string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, name,
	).Scan(&got)
	if err != nil {
		t.Fatalf("table %s missing: %v", name, err)
	}
	if got != name {
		t.Fatalf("table = %q, want %q", got, name)
	}
}
