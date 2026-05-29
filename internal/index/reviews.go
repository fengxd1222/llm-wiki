package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrReviewNotFound indicates that a requested review row does not exist.
var ErrReviewNotFound = errors.New("review not found")

// ReviewRow maps one row in the reviews table.
type ReviewRow struct {
	ID           string
	Seq          int
	BundleID     string
	Agent        string
	SessionID    string
	Op           string
	TargetPageID string
	PatchPath    string
	Status       string
	CreatedAt    string
	DecidedAt    string
	DecidedBy    string
	MetaJSON     string
}

// NextReviewSeq returns max(seq)+1. Empty table starts at 1.
func NextReviewSeq(ctx context.Context, db *DB) (int, error) {
	if db == nil || db.SQL() == nil {
		return 0, ErrIndexUnavailable
	}
	var next int
	if err := db.SQL().QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM reviews`,
	).Scan(&next); err != nil {
		return 0, fmt.Errorf("next review seq: %w", err)
	}
	return next, nil
}

// ReviewID formats the public review id for a sequence.
func ReviewID(seq int) string {
	return fmt.Sprintf("r-%04d", seq)
}

// InsertReview writes one review row.
func InsertReview(ctx context.Context, db *DB, row *ReviewRow) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	if row == nil || row.ID == "" {
		return fmt.Errorf("review row must have non-empty id")
	}
	if row.MetaJSON == "" {
		row.MetaJSON = "{}"
	}
	const q = `INSERT INTO reviews (
    id, seq, bundle_id, agent, session_id, op, target_page_id,
    patch_path, status, created_at, decided_at, decided_by, meta_json
) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)`
	if _, err := db.SQL().ExecContext(ctx, q,
		row.ID, row.Seq, row.BundleID, row.Agent, row.SessionID, row.Op,
		row.TargetPageID, row.PatchPath, row.Status, row.CreatedAt,
		row.DecidedAt, row.DecidedBy, row.MetaJSON,
	); err != nil {
		return fmt.Errorf("insert review %s: %w", row.ID, err)
	}
	return nil
}

// ListReviewsByStatus returns reviews matching a status, ordered by seq.
func ListReviewsByStatus(ctx context.Context, db *DB, status string) ([]*ReviewRow, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	rows, err := db.SQL().QueryContext(ctx, reviewSelectSQL+` WHERE status = ? ORDER BY seq`, status)
	if err != nil {
		return nil, fmt.Errorf("query reviews by status: %w", err)
	}
	defer rows.Close()
	return scanReviewRows(rows)
}

// FindReviewByIdempotencyKey returns an existing review for an agent/key pair.
func FindReviewByIdempotencyKey(ctx context.Context, db *DB, agent, key string) (*ReviewRow, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	// escapeLikePattern 转义 key 里的 LIKE 元字符（% / _ / \），配合 ESCAPE '\' 子句，
	// 避免 user-controlled idempotency_key 含通配符时误命中其他 review 的 meta_json。
	pattern := `%"idempotency_key":"` + escapeLikePattern(key) + `"%`
	row := db.SQL().QueryRowContext(ctx,
		reviewSelectSQL+` WHERE agent = ? AND meta_json LIKE ? ESCAPE '\' ORDER BY seq LIMIT 1`,
		agent, pattern)
	got, err := scanReviewRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query review by idempotency key: %w", err)
	}
	return got, nil
}

// CountReviewsByStatus counts reviews matching a status.
func CountReviewsByStatus(ctx context.Context, db *DB, status string) (int, error) {
	if db == nil || db.SQL() == nil {
		return 0, ErrIndexUnavailable
	}
	var count int
	if err := db.SQL().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM reviews WHERE status = ?`, status,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count reviews by status: %w", err)
	}
	return count, nil
}

// AssignReviewsToBundle attaches pending reviews to a bundle.
func AssignReviewsToBundle(ctx context.Context, db *DB, bundleID string, reviewIDs []string) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	for _, id := range reviewIDs {
		res, err := db.SQL().ExecContext(ctx,
			`UPDATE reviews SET bundle_id = ? WHERE id = ? AND COALESCE(bundle_id, '') = ''`,
			bundleID, id)
		if err != nil {
			return fmt.Errorf("assign review %s to bundle %s: %w", id, bundleID, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("assign review %s rows affected: %w", id, err)
		}
		if n == 0 {
			return fmt.Errorf("%w: %s", ErrReviewNotFound, id)
		}
	}
	return nil
}

// GetReviewByID returns one review row.
func GetReviewByID(ctx context.Context, db *DB, id string) (*ReviewRow, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	row := db.SQL().QueryRowContext(ctx, reviewSelectSQL+` WHERE id = ? LIMIT 1`, id)
	got, err := scanReviewRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("query review %s: %w", id, err)
	}
	return got, nil
}

// UpdateReviewStatus updates the state and decision metadata for one review.
func UpdateReviewStatus(ctx context.Context, db *DB, id, status, decidedBy string) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	res, err := db.SQL().ExecContext(ctx,
		`UPDATE reviews
SET status = ?, decided_by = NULLIF(?, ''), decided_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`,
		status, decidedBy, id)
	if err != nil {
		return fmt.Errorf("update review %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update review %s rows affected: %w", id, err)
	}
	if n == 0 {
		return ErrReviewNotFound
	}
	return nil
}

const reviewSelectSQL = `SELECT id, seq, COALESCE(bundle_id, ''), agent, session_id, op,
       COALESCE(target_page_id, ''), patch_path, status, created_at,
       COALESCE(decided_at, ''), COALESCE(decided_by, ''), meta_json
FROM reviews`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanReviewRows(rows *sql.Rows) ([]*ReviewRow, error) {
	var out []*ReviewRow
	for rows.Next() {
		got, err := scanReviewRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		out = append(out, got)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reviews: %w", err)
	}
	return out, nil
}

func scanReviewRow(row rowScanner) (*ReviewRow, error) {
	var r ReviewRow
	if err := row.Scan(
		&r.ID, &r.Seq, &r.BundleID, &r.Agent, &r.SessionID, &r.Op,
		&r.TargetPageID, &r.PatchPath, &r.Status, &r.CreatedAt,
		&r.DecidedAt, &r.DecidedBy, &r.MetaJSON,
	); err != nil {
		return nil, err
	}
	return &r, nil
}
