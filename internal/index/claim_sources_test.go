package index

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// TestUpdateClaimSourceStatusNotFound verifies that updating a non-existent
// claim source returns the package sentinel ErrClaimSourceNotFound and never
// leaks sql.ErrNoRows to callers (F-009).
func TestUpdateClaimSourceStatusNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)

	err := UpdateClaimSourceStatus(ctx, db, "cl-missing", "raw/inbox/x.md", "anchor-1", "drift", 1700000000)
	if !errors.Is(err, ErrClaimSourceNotFound) {
		t.Fatalf("UpdateClaimSourceStatus err = %v, want ErrClaimSourceNotFound", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateClaimSourceStatus leaked sql.ErrNoRows: %v", err)
	}
}

// TestUpdateClaimSourceStatusUpdatesExisting verifies the happy path: an
// existing row is updated without error.
func TestUpdateClaimSourceStatusUpdatesExisting(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)

	row := &ClaimSourceRow{
		ClaimID:         "cl-1",
		RawID:           "raw/inbox/x.md",
		Anchor:          "anchor-1",
		StoredQuoteHash: "hash-1",
		QuotePreview:    "quote",
		SpanStart:       0,
		SpanEnd:         5,
		LastVerifiedAt:  1700000000,
		CachedStatus:    "unknown",
	}
	if err := InsertClaimSource(ctx, db, row); err != nil {
		t.Fatalf("InsertClaimSource: %v", err)
	}

	if err := UpdateClaimSourceStatus(ctx, db, "cl-1", "raw/inbox/x.md", "anchor-1", "verified", 1700000100); err != nil {
		t.Fatalf("UpdateClaimSourceStatus: %v", err)
	}

	sources, err := ListClaimSources(ctx, db, "cl-1")
	if err != nil {
		t.Fatalf("ListClaimSources: %v", err)
	}
	if len(sources) != 1 || sources[0].CachedStatus != "verified" {
		t.Fatalf("sources = %+v, want one row with cached_status=verified", sources)
	}
}
