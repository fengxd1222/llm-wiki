package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrBundleNotFound indicates that a requested bundle row does not exist.
var ErrBundleNotFound = errors.New("bundle not found")

// BundleRow maps one row in the bundles table.
type BundleRow struct {
	ID          string
	Seq         int
	Agent       string
	SessionID   string
	Summary     string
	Status      string
	CreatedAt   string
	SubmittedAt string
	DecidedAt   string
}

// NextBundleSeq returns max(seq)+1. Empty table starts at 1.
func NextBundleSeq(ctx context.Context, db *DB) (int, error) {
	if db == nil || db.SQL() == nil {
		return 0, ErrIndexUnavailable
	}
	var next int
	if err := db.SQL().QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM bundles`,
	).Scan(&next); err != nil {
		return 0, fmt.Errorf("next bundle seq: %w", err)
	}
	return next, nil
}

// BundleID formats the public bundle id for a sequence.
func BundleID(seq int) string {
	return fmt.Sprintf("b-%04d", seq)
}

// InsertBundle writes one bundle row.
func InsertBundle(ctx context.Context, db *DB, row *BundleRow) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	if row == nil || row.ID == "" {
		return fmt.Errorf("bundle row must have non-empty id")
	}
	const q = `INSERT INTO bundles (
    id, seq, agent, session_id, summary, status, created_at, submitted_at, decided_at
) VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''))`
	if _, err := db.SQL().ExecContext(ctx, q,
		row.ID, row.Seq, row.Agent, row.SessionID, row.Summary,
		row.Status, row.CreatedAt, row.SubmittedAt, row.DecidedAt,
	); err != nil {
		return fmt.Errorf("insert bundle %s: %w", row.ID, err)
	}
	return nil
}

// GetBundleByID returns one bundle row.
func GetBundleByID(ctx context.Context, db *DB, id string) (*BundleRow, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	row := db.SQL().QueryRowContext(ctx, bundleSelectSQL+` WHERE id = ? LIMIT 1`, id)
	got, err := scanBundleRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBundleNotFound
		}
		return nil, fmt.Errorf("query bundle %s: %w", id, err)
	}
	return got, nil
}

const bundleSelectSQL = `SELECT id, seq, agent, session_id, summary, status, created_at,
       COALESCE(submitted_at, ''), COALESCE(decided_at, '')
FROM bundles`

func scanBundleRow(row rowScanner) (*BundleRow, error) {
	var b BundleRow
	if err := row.Scan(
		&b.ID, &b.Seq, &b.Agent, &b.SessionID, &b.Summary,
		&b.Status, &b.CreatedAt, &b.SubmittedAt, &b.DecidedAt,
	); err != nil {
		return nil, err
	}
	return &b, nil
}
