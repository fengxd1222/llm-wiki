package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

const wikiIndexRelPath = "wiki/index.md"

const wikiIndexHeader = `# WikiMind Index

| ID | Type | Title | Sources | Confidence | Updated |
|---|---|---|---|---|---|
`

// PageInfo holds the minimal info needed to append an index entry.
type PageInfo struct {
	ID         string
	Type       string
	Title      string
	Sources    int
	Confidence string // formatted string like "0.92" or "—"
	Updated    string // date string like "2026-05-24"
}

// EnsureIndex creates wiki/index.md with the table header if it doesn't exist.
func EnsureIndex(ctx context.Context, vaultRoot string) error {
	absPath := filepath.Join(vaultRoot, wikiIndexRelPath)
	if _, err := os.Stat(absPath); err == nil {
		return nil // already exists
	}
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure wiki dir: %w", err)
	}
	if err := os.WriteFile(absPath, []byte(wikiIndexHeader), 0o644); err != nil {
		return fmt.Errorf("write wiki/index.md: %w", err)
	}
	return nil
}

// AppendIndexEntry appends a single row to wiki/index.md.
// Creates the file with header if it doesn't exist.
func AppendIndexEntry(ctx context.Context, vaultRoot string, info PageInfo) error {
	if err := EnsureIndex(ctx, vaultRoot); err != nil {
		return err
	}
	absPath := filepath.Join(vaultRoot, wikiIndexRelPath)
	f, err := os.OpenFile(absPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open wiki/index.md for append: %w", err)
	}
	defer f.Close()

	sources := "—"
	if info.Sources > 0 {
		sources = fmt.Sprintf("%d", info.Sources)
	}
	confidence := info.Confidence
	if confidence == "" {
		confidence = "—"
	}
	updated := info.Updated
	if updated == "" {
		updated = time.Now().UTC().Format("2006-01-02")
	}

	line := fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
		info.ID, info.Type, info.Title, sources, confidence, updated)
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("append to wiki/index.md: %w", err)
	}
	return nil
}

// RebuildIndex rebuilds wiki/index.md from the pages table.
func RebuildIndex(ctx context.Context, db *index.DB, vaultRoot string) error {
	absPath := filepath.Join(vaultRoot, wikiIndexRelPath)
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure wiki dir: %w", err)
	}

	pages, err := index.ListPages(ctx, db, "")
	if err != nil {
		return fmt.Errorf("list pages for index rebuild: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(wikiIndexHeader)

	for _, p := range pages {
		confidence := "—"
		if p.Confidence.Valid {
			confidence = fmt.Sprintf("%.2f", p.Confidence.Float64)
		}
		sources := "—"
		updated := "—"
		if p.UpdatedAt > 0 {
			updated = time.Unix(p.UpdatedAt, 0).UTC().Format("2006-01-02")
		} else if p.CreatedAt > 0 {
			updated = time.Unix(p.CreatedAt, 0).UTC().Format("2006-01-02")
		}
		// Count sources for this page (simplified: count outbound links from source pages)
		srcCount := countSourceLinks(ctx, db, p.ID)
		if srcCount > 0 {
			sources = fmt.Sprintf("%d", srcCount)
		}

		line := fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
			p.ID, p.Type, p.Title, sources, confidence, updated)
		sb.WriteString(line)
	}

	if err := os.WriteFile(absPath, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("write wiki/index.md: %w", err)
	}
	return nil
}

// countSourceLinks counts inbound links from source-type pages to the given page.
func countSourceLinks(ctx context.Context, db *index.DB, pageID string) int {
	if db == nil || db.SQL() == nil {
		return 0
	}
	const q = `SELECT COUNT(*) FROM page_links pl
JOIN pages p ON pl.source_id = p.id
WHERE pl.target_id = ? AND p.type = 'source'`
	var count int
	if err := db.SQL().QueryRowContext(ctx, q, pageID).Scan(&count); err != nil {
		if err == sql.ErrNoRows {
			return 0
		}
		return 0
	}
	return count
}
