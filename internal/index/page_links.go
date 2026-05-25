package index

import (
	"context"
	"database/sql"
	"fmt"
)

// PageLink represents a row in the page_links table.
type PageLink struct {
	SourceID string
	TargetID string
	LinkType string
}

// InsertPageLink inserts a single link. ON CONFLICT DO NOTHING makes it idempotent.
func InsertPageLink(ctx context.Context, db *DB, link *PageLink) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	if link == nil || link.SourceID == "" || link.TargetID == "" {
		return fmt.Errorf("page_link: source_id and target_id are required")
	}
	linkType := link.LinkType
	if linkType == "" {
		linkType = "ref"
	}
	const q = `INSERT INTO page_links (source_id, target_id, link_type)
VALUES (?, ?, ?)
ON CONFLICT DO NOTHING`
	if _, err := db.SQL().ExecContext(ctx, q, link.SourceID, link.TargetID, linkType); err != nil {
		return fmt.Errorf("insert page_link %s->%s: %w", link.SourceID, link.TargetID, err)
	}
	return nil
}

// ReplacePageLinks replaces all outbound links for a given source page atomically.
// Deletes existing links for sourceID, then inserts the new set.
func ReplacePageLinks(ctx context.Context, db *DB, sourceID string, targets []string) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	if sourceID == "" {
		return fmt.Errorf("page_links: source_id is required")
	}

	tx, err := db.SQL().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("page_links begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM page_links WHERE source_id = ?`, sourceID); err != nil {
		return fmt.Errorf("page_links delete old: %w", err)
	}

	if len(targets) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO page_links (source_id, target_id, link_type) VALUES (?, ?, 'ref') ON CONFLICT DO NOTHING`)
		if err != nil {
			return fmt.Errorf("page_links prepare: %w", err)
		}
		defer stmt.Close()
		for _, t := range targets {
			if t == "" {
				continue
			}
			if _, err := stmt.ExecContext(ctx, sourceID, t); err != nil {
				return fmt.Errorf("page_links insert %s->%s: %w", sourceID, t, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("page_links commit: %w", err)
	}
	return nil
}

// InboundLinks returns all page IDs that link TO the given targetID.
func InboundLinks(ctx context.Context, db *DB, targetID string) ([]PageLink, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	const q = `SELECT source_id, target_id, link_type FROM page_links WHERE target_id = ?`
	rows, err := db.SQL().QueryContext(ctx, q, targetID)
	if err != nil {
		return nil, fmt.Errorf("query inbound links for %s: %w", targetID, err)
	}
	defer rows.Close()

	var out []PageLink
	for rows.Next() {
		var l PageLink
		if err := rows.Scan(&l.SourceID, &l.TargetID, &l.LinkType); err != nil {
			return nil, fmt.Errorf("scan page_link: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// OutboundLinks returns all page IDs that the given sourceID links TO.
func OutboundLinks(ctx context.Context, db *DB, sourceID string) ([]PageLink, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	const q = `SELECT source_id, target_id, link_type FROM page_links WHERE source_id = ?`
	rows, err := db.SQL().QueryContext(ctx, q, sourceID)
	if err != nil {
		return nil, fmt.Errorf("query outbound links for %s: %w", sourceID, err)
	}
	defer rows.Close()

	var out []PageLink
	for rows.Next() {
		var l PageLink
		if err := rows.Scan(&l.SourceID, &l.TargetID, &l.LinkType); err != nil {
			return nil, fmt.Errorf("scan page_link: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// CountOrphanPages counts pages of given types that have no inbound links.
// Used for health score calculation.
func CountOrphanPages(ctx context.Context, db *DB, types []string) (int, error) {
	if db == nil || db.SQL() == nil {
		return 0, ErrIndexUnavailable
	}
	if len(types) == 0 {
		return 0, nil
	}

	// Build placeholders for IN clause
	placeholders := ""
	args := make([]interface{}, len(types))
	for i, t := range types {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args[i] = t
	}

	q := fmt.Sprintf(`SELECT COUNT(*) FROM pages
WHERE type IN (%s)
AND id NOT IN (SELECT DISTINCT target_id FROM page_links)`, placeholders)

	var count int
	if err := db.SQL().QueryRowContext(ctx, q, args...).Scan(&count); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("count orphan pages: %w", err)
	}
	return count, nil
}

// DeleteAllPageLinks removes all rows from page_links (used during full reindex).
func DeleteAllPageLinks(ctx context.Context, db *DB) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	if _, err := db.SQL().ExecContext(ctx, `DELETE FROM page_links`); err != nil {
		return fmt.Errorf("delete all page_links: %w", err)
	}
	return nil
}
