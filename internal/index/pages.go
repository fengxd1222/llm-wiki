package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// PageRow 映射 pages 表的一行。
//
// Path 是 vault-relative POSIX 路径（如 "wiki/claims/wiki-is-compounding.md"）。
// Frontmatter 是序列化后的 JSON 字符串，调用方自行 marshal / unmarshal。
// Body 冗余存 markdown 正文，让 pages_fts 同步可由 trigger 完成。
type PageRow struct {
	ID            string
	Type          string
	Path          string
	Title         string
	Body          string
	Confidence    sql.NullFloat64
	Status        string
	SchemaVersion string
	CreatedBy     string
	UpdatedBy     string
	CreatedAt     int64
	UpdatedAt     int64
	Frontmatter   string
}

// UpsertPage 写入或覆盖一行 page。
//
// 用 INSERT ... ON CONFLICT(id) DO UPDATE 保证 reindex 幂等。
// pages_fts 由 trigger 自动同步。
func UpsertPage(ctx context.Context, db *DB, row *PageRow) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	if row == nil || row.ID == "" {
		return fmt.Errorf("page row must have non-empty id")
	}
	const q = `INSERT INTO pages (
    id, type, path, title, body, confidence, status, schema_ver,
    created_by, updated_by, created_at, updated_at, frontmatter
) VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, NULLIF(?, ''))
ON CONFLICT(id) DO UPDATE SET
    type        = excluded.type,
    path        = excluded.path,
    title       = excluded.title,
    body        = excluded.body,
    confidence  = excluded.confidence,
    status      = excluded.status,
    schema_ver  = excluded.schema_ver,
    created_by  = excluded.created_by,
    updated_by  = excluded.updated_by,
    created_at  = excluded.created_at,
    updated_at  = excluded.updated_at,
    frontmatter = excluded.frontmatter`

	var confidence interface{}
	if row.Confidence.Valid {
		confidence = row.Confidence.Float64
	}

	if _, err := db.SQL().ExecContext(ctx, q,
		row.ID, row.Type, row.Path, row.Title, row.Body,
		confidence, row.Status, row.SchemaVersion,
		row.CreatedBy, row.UpdatedBy,
		row.CreatedAt, row.UpdatedAt,
		row.Frontmatter,
	); err != nil {
		return fmt.Errorf("upsert page %s: %w", row.ID, err)
	}
	return nil
}

// ListPages 返回 pages 表全量记录；typeFilter 非空时按 type 过滤。
//
// 按 type, id 排序输出，方便 CLI 分组打印。
func ListPages(ctx context.Context, db *DB, typeFilter string) ([]*PageRow, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	var (
		rows *sql.Rows
		err  error
	)
	base := `SELECT id, type, path, title, body,
       confidence, COALESCE(status, ''), schema_ver,
       COALESCE(created_by, ''), COALESCE(updated_by, ''),
       COALESCE(created_at, 0), COALESCE(updated_at, 0),
       COALESCE(frontmatter, '')
FROM pages`
	if typeFilter != "" {
		rows, err = db.SQL().QueryContext(ctx, base+" WHERE type = ? ORDER BY type, id", typeFilter)
	} else {
		rows, err = db.SQL().QueryContext(ctx, base+" ORDER BY type, id")
	}
	if err != nil {
		return nil, fmt.Errorf("query pages: %w", err)
	}
	defer rows.Close()

	var out []*PageRow
	for rows.Next() {
		var p PageRow
		var conf sql.NullFloat64
		if err := rows.Scan(
			&p.ID, &p.Type, &p.Path, &p.Title, &p.Body,
			&conf, &p.Status, &p.SchemaVersion,
			&p.CreatedBy, &p.UpdatedBy,
			&p.CreatedAt, &p.UpdatedAt,
			&p.Frontmatter,
		); err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		p.Confidence = conf
		out = append(out, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pages: %w", err)
	}
	return out, nil
}

// GetPageByID 取单条 page；未命中返回 (nil, nil)。
func GetPageByID(ctx context.Context, db *DB, id string) (*PageRow, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	const q = `SELECT id, type, path, title, body,
       confidence, COALESCE(status, ''), schema_ver,
       COALESCE(created_by, ''), COALESCE(updated_by, ''),
       COALESCE(created_at, 0), COALESCE(updated_at, 0),
       COALESCE(frontmatter, '')
FROM pages WHERE id = ? LIMIT 1`
	row := db.SQL().QueryRowContext(ctx, q, id)
	var p PageRow
	var conf sql.NullFloat64
	if err := row.Scan(
		&p.ID, &p.Type, &p.Path, &p.Title, &p.Body,
		&conf, &p.Status, &p.SchemaVersion,
		&p.CreatedBy, &p.UpdatedBy,
		&p.CreatedAt, &p.UpdatedAt,
		&p.Frontmatter,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query page %s: %w", id, err)
	}
	p.Confidence = conf
	return &p, nil
}
