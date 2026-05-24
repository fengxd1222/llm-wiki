package index

import (
	"context"
	"errors"
	"testing"
)

func TestReviewCRUD(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)

	seq, err := NextReviewSeq(ctx, db)
	if err != nil {
		t.Fatalf("NextReviewSeq: %v", err)
	}
	if seq != 1 {
		t.Fatalf("NextReviewSeq empty = %d, want 1", seq)
	}

	row := &ReviewRow{
		ID:           ReviewID(seq),
		Seq:          seq,
		BundleID:     "b-0001",
		Agent:        "codex-cli",
		SessionID:    "sess-1",
		Op:           "propose_edit",
		TargetPageID: "cl-001",
		PatchPath:    "wiki/_review/r-0001.patch",
		Status:       "pending",
		CreatedAt:    "2026-05-24T12:00:00Z",
		MetaJSON:     `{"quote_hash":"abcd1234"}`,
	}
	if err := InsertReview(ctx, db, row); err != nil {
		t.Fatalf("InsertReview: %v", err)
	}

	if next, err := NextReviewSeq(ctx, db); err != nil || next != 2 {
		t.Fatalf("NextReviewSeq after insert = %d, %v; want 2, nil", next, err)
	}

	got, err := GetReviewByID(ctx, db, "r-0001")
	if err != nil {
		t.Fatalf("GetReviewByID: %v", err)
	}
	if got.Agent != row.Agent || got.BundleID != row.BundleID ||
		got.TargetPageID != row.TargetPageID || got.MetaJSON != row.MetaJSON {
		t.Fatalf("review row mismatch: %+v want %+v", got, row)
	}

	pending, err := ListReviewsByStatus(ctx, db, "pending")
	if err != nil {
		t.Fatalf("ListReviewsByStatus: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != "r-0001" {
		t.Fatalf("pending reviews = %+v, want r-0001", pending)
	}
	count, err := CountReviewsByStatus(ctx, db, "pending")
	if err != nil {
		t.Fatalf("CountReviewsByStatus: %v", err)
	}
	if count != 1 {
		t.Fatalf("pending count = %d, want 1", count)
	}

	if err := UpdateReviewStatus(ctx, db, "r-0001", "accepted", "user"); err != nil {
		t.Fatalf("UpdateReviewStatus: %v", err)
	}
	got, err = GetReviewByID(ctx, db, "r-0001")
	if err != nil {
		t.Fatalf("GetReviewByID after update: %v", err)
	}
	if got.Status != "accepted" || got.DecidedBy != "user" || got.DecidedAt == "" {
		t.Fatalf("status update not persisted: %+v", got)
	}
}

func TestReviewNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)

	if _, err := GetReviewByID(ctx, db, "r-missing"); !errors.Is(err, ErrReviewNotFound) {
		t.Fatalf("GetReviewByID err = %v, want ErrReviewNotFound", err)
	}
	if err := UpdateReviewStatus(ctx, db, "r-missing", "rejected", "user"); !errors.Is(err, ErrReviewNotFound) {
		t.Fatalf("UpdateReviewStatus err = %v, want ErrReviewNotFound", err)
	}
}

func TestReviewIdempotencyAndAssignBundle(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)
	row := &ReviewRow{
		ID:        "r-0001",
		Seq:       1,
		Agent:     "codex-cli",
		SessionID: "sess-1",
		Op:        "propose_page",
		PatchPath: "wiki/_review/r-0001.patch",
		Status:    "pending",
		CreatedAt: "2026-05-24T12:00:00Z",
		MetaJSON:  `{"idempotency_key":"abc","path":"wiki/claims/a.md"}`,
	}
	if err := InsertReview(ctx, db, row); err != nil {
		t.Fatalf("InsertReview: %v", err)
	}
	got, err := FindReviewByIdempotencyKey(ctx, db, "codex-cli", "abc")
	if err != nil {
		t.Fatalf("FindReviewByIdempotencyKey: %v", err)
	}
	if got == nil || got.ID != "r-0001" {
		t.Fatalf("idempotency lookup = %+v, want r-0001", got)
	}
	if err := AssignReviewsToBundle(ctx, db, "b-0001", []string{"r-0001"}); err != nil {
		t.Fatalf("AssignReviewsToBundle: %v", err)
	}
	got, err = GetReviewByID(ctx, db, "r-0001")
	if err != nil {
		t.Fatalf("GetReviewByID: %v", err)
	}
	if got.BundleID != "b-0001" {
		t.Fatalf("BundleID = %q, want b-0001", got.BundleID)
	}
}
