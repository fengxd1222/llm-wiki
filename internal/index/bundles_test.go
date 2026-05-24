package index

import (
	"context"
	"errors"
	"testing"
)

func TestBundleCRUD(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)

	seq, err := NextBundleSeq(ctx, db)
	if err != nil {
		t.Fatalf("NextBundleSeq: %v", err)
	}
	if seq != 1 {
		t.Fatalf("NextBundleSeq empty = %d, want 1", seq)
	}

	row := &BundleRow{
		ID:        BundleID(seq),
		Seq:       seq,
		Agent:     "claude-code",
		SessionID: "sess-1",
		Summary:   "ingest karpathy notes",
		Status:    "open",
		CreatedAt: "2026-05-24T12:00:00Z",
	}
	if err := InsertBundle(ctx, db, row); err != nil {
		t.Fatalf("InsertBundle: %v", err)
	}
	if next, err := NextBundleSeq(ctx, db); err != nil || next != 2 {
		t.Fatalf("NextBundleSeq after insert = %d, %v; want 2, nil", next, err)
	}

	got, err := GetBundleByID(ctx, db, "b-0001")
	if err != nil {
		t.Fatalf("GetBundleByID: %v", err)
	}
	if got.Agent != row.Agent || got.SessionID != row.SessionID ||
		got.Summary != row.Summary || got.Status != row.Status {
		t.Fatalf("bundle row mismatch: %+v want %+v", got, row)
	}
}

func TestBundleNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)

	if _, err := GetBundleByID(ctx, db, "b-missing"); !errors.Is(err, ErrBundleNotFound) {
		t.Fatalf("GetBundleByID err = %v, want ErrBundleNotFound", err)
	}
}
