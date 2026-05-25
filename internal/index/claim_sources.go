package index

import (
	"context"
	"database/sql"
	"fmt"
)

// ClaimSourceRow maps one row in the claim_sources table.
type ClaimSourceRow struct {
	ClaimID         string
	RawID           string
	Anchor          string
	StoredQuoteHash string
	QuotePreview    string
	SpanStart       int
	SpanEnd         int
	LastVerifiedAt  int64
	CachedStatus    string // "unknown", "verified", "drift", "anchor_missing", "raw_missing"
}

// InsertClaimSource inserts or replaces a claim source row.
func InsertClaimSource(ctx context.Context, db *DB, row *ClaimSourceRow) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	const q = `INSERT INTO claim_sources (claim_id, raw_id, anchor, stored_quote_hash, quote_preview, span_start, span_end, last_verified_at, cached_status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(claim_id, raw_id, anchor) DO UPDATE SET
    stored_quote_hash = excluded.stored_quote_hash,
    quote_preview = excluded.quote_preview,
    span_start = excluded.span_start,
    span_end = excluded.span_end,
    last_verified_at = excluded.last_verified_at,
    cached_status = excluded.cached_status`
	_, err := db.SQL().ExecContext(ctx, q,
		row.ClaimID, row.RawID, row.Anchor, row.StoredQuoteHash,
		row.QuotePreview, row.SpanStart, row.SpanEnd,
		row.LastVerifiedAt, row.CachedStatus)
	if err != nil {
		return fmt.Errorf("insert claim_source: %w", err)
	}
	return nil
}

// ListClaimSources returns all sources for a given claim.
func ListClaimSources(ctx context.Context, db *DB, claimID string) ([]*ClaimSourceRow, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	const q = `SELECT claim_id, raw_id, anchor, stored_quote_hash, quote_preview, span_start, span_end, last_verified_at, cached_status
FROM claim_sources WHERE claim_id = ?`
	rows, err := db.SQL().QueryContext(ctx, q, claimID)
	if err != nil {
		return nil, fmt.Errorf("list claim sources: %w", err)
	}
	defer rows.Close()

	var result []*ClaimSourceRow
	for rows.Next() {
		r := &ClaimSourceRow{}
		if err := rows.Scan(&r.ClaimID, &r.RawID, &r.Anchor, &r.StoredQuoteHash,
			&r.QuotePreview, &r.SpanStart, &r.SpanEnd, &r.LastVerifiedAt, &r.CachedStatus); err != nil {
			return nil, fmt.Errorf("scan claim source: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ListClaimSourcesByRaw returns all claim sources referencing a given raw file.
func ListClaimSourcesByRaw(ctx context.Context, db *DB, rawID string) ([]*ClaimSourceRow, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	const q = `SELECT claim_id, raw_id, anchor, stored_quote_hash, quote_preview, span_start, span_end, last_verified_at, cached_status
FROM claim_sources WHERE raw_id = ?`
	rows, err := db.SQL().QueryContext(ctx, q, rawID)
	if err != nil {
		return nil, fmt.Errorf("list claim sources by raw: %w", err)
	}
	defer rows.Close()

	var result []*ClaimSourceRow
	for rows.Next() {
		r := &ClaimSourceRow{}
		if err := rows.Scan(&r.ClaimID, &r.RawID, &r.Anchor, &r.StoredQuoteHash,
			&r.QuotePreview, &r.SpanStart, &r.SpanEnd, &r.LastVerifiedAt, &r.CachedStatus); err != nil {
			return nil, fmt.Errorf("scan claim source: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// UpdateClaimSourceStatus updates the cached drift status for a claim source.
func UpdateClaimSourceStatus(ctx context.Context, db *DB, claimID, rawID, anchor, status string, verifiedAt int64) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	const q = `UPDATE claim_sources SET cached_status = ?, last_verified_at = ? WHERE claim_id = ? AND raw_id = ? AND anchor = ?`
	res, err := db.SQL().ExecContext(ctx, q, status, verifiedAt, claimID, rawID, anchor)
	if err != nil {
		return fmt.Errorf("update claim source status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CountDriftClaims counts distinct claims that have at least one source in drift status.
func CountDriftClaims(ctx context.Context, db *DB) (int, error) {
	if db == nil || db.SQL() == nil {
		return 0, ErrIndexUnavailable
	}
	const q = `SELECT COUNT(DISTINCT claim_id) FROM claim_sources WHERE cached_status = 'drift'`
	var count int
	if err := db.SQL().QueryRowContext(ctx, q).Scan(&count); err != nil {
		return 0, fmt.Errorf("count drift claims: %w", err)
	}
	return count, nil
}
