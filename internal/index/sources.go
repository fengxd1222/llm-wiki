package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SourceRow 映射 sources 表的一行。
//
// raw_id 是 vault-relative POSIX 路径（如 "raw/inbox/karpathy-llm-wiki.md"）。
// parser / metadata 在 D3 留空（默认 NULL），D4+ parse 阶段会填。
type SourceRow struct {
	RawID      string
	SHA256     string
	Size       int64
	MTime      int64
	Status     string
	IngestedAt int64
	Parser     string
	Metadata   string
}

// FindSourceBySHA256 按 sha256 命中已入仓的 source；未命中返回 (nil, nil)。
//
// 用于 ingest 去重：同一文件内容（无论文件名）只入仓一次。
func FindSourceBySHA256(ctx context.Context, db *DB, sha256 string) (*SourceRow, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	const q = `SELECT raw_id, sha256, size, mtime, status,
       COALESCE(ingested_at, 0), COALESCE(parser, ''), COALESCE(metadata, '')
FROM sources WHERE sha256 = ? LIMIT 1`
	row := db.SQL().QueryRowContext(ctx, q, sha256)
	var s SourceRow
	if err := row.Scan(&s.RawID, &s.SHA256, &s.Size, &s.MTime, &s.Status,
		&s.IngestedAt, &s.Parser, &s.Metadata); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query sources by sha256: %w", err)
	}
	return &s, nil
}

// DeleteSourceByRawID 按 raw_id 删除一行 source。未命中视为成功（best-effort 回滚用）。
func DeleteSourceByRawID(ctx context.Context, db *DB, rawID string) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	if _, err := db.SQL().ExecContext(ctx, `DELETE FROM sources WHERE raw_id = ?`, rawID); err != nil {
		return fmt.Errorf("delete source %s: %w", rawID, err)
	}
	return nil
}

// InsertSource 写入一行 source。raw_id 冲突时返回错误（调用方应先去重）。
func InsertSource(ctx context.Context, db *DB, src *SourceRow) error {
	if db == nil || db.SQL() == nil {
		return ErrIndexUnavailable
	}
	const q = `INSERT INTO sources (raw_id, sha256, size, mtime, status, ingested_at, parser, metadata)
VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''))`
	if _, err := db.SQL().ExecContext(ctx, q,
		src.RawID, src.SHA256, src.Size, src.MTime, src.Status,
		src.IngestedAt, src.Parser, src.Metadata,
	); err != nil {
		return fmt.Errorf("insert source %s: %w", src.RawID, err)
	}
	return nil
}
