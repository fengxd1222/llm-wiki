package mcp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/service"
	"github.com/fengxd1222/llm-wiki/internal/vault"
)

// ErrFormatUnsupported 表示客户端请求了 D8 尚未实现的 read_raw format
// （目前仅 "normalized" 触发，等 W2 D9 read_raw_anchor 一起上线 stage-2 parse）。
var ErrFormatUnsupported = errors.New("format unsupported")

// ErrRawIDOutsideRaw 表示 read_raw 的 raw_id 没指向 raw/ 子树。
//
// 严格只允许 raw/ 下访问——避免把 wiki/ 当 raw 读乱权限边界。
var ErrRawIDOutsideRaw = errors.New("raw_id must be under raw/")

// ErrPageNotFound 表示 read_page 在 pages 表（id 查）和 wiki/ 文件树
// （path 查）都没命中。
var ErrPageNotFound = errors.New("page not found")

// ErrRawNotFound 表示 read_raw 解析后路径在 vault 内但文件不存在。
var ErrRawNotFound = errors.New("raw file not found")

// daemonVersion 在 wiki_info 响应中返回；与 cmd/wikimind 的 version 解耦
// 一些——MCP 进程语义上是 daemon 角色（D10+ 由真正 daemon 接管前 staged）。
const daemonVersion = "0.1.0-w2"

// schemaVersion 是 MCP 协议宣称的 vault schema 版本——优先从 vault config
// 读真值；读取失败时退回此常量保证 wiki_info 总有响应。
const fallbackSchemaVersion = "1.0"

// historyNote / backlinksNote 给 read_page 的 staged 字段一个稳定的解释，
// 让 LLM agent 看到空数组不会以为是 bug。
const (
	historyNote   = "history requires git log integration (W2 D9+)"
	backlinksNote = "backlinks require page_links table (W2 D10+)"
)

// formatNormalizedNote 留给 read_raw 拒绝 normalized 时返回，提示用户
// 何时这条路径会通。
const formatNormalizedNote = "normalized format requires stage-2 parser (W2 D9 with read_raw_anchor)"

// vaultBackend 把 4 个 tool 需要的依赖收拢为一个结构，让 server 构造时
// 一次性传入；handler 通过闭包持有，避免每次 tool 调用走 context value 取值。
type vaultBackend struct {
	root string
	db   *index.DB
}

// handleWikiInfo 实现 mcp-tools.md §2 wiki_info。
//
// 直接 SQL COUNT() 读 sources / pages 各类型计数。SQLite 数据 < 10k 量级，
// 全量 COUNT(*) 一次 < 10 ms，无需缓存。
func (b *vaultBackend) handleWikiInfo(ctx context.Context, _ WikiInfoArgs) (WikiInfoResult, error) {
	result := WikiInfoResult{
		VaultRoot:     b.root,
		SchemaVersion: fallbackSchemaVersion,
		DaemonVersion: daemonVersion,
		Health: HealthBlock{
			Score:        100,
			DriftClaims:  0,
			LintWarnings: 0,
		},
	}
	if cfg, err := vault.LoadConfig(b.root); err == nil && cfg.SchemaVersion != "" {
		result.SchemaVersion = cfg.SchemaVersion
	}

	counts, err := countVault(ctx, b.db)
	if err != nil {
		return WikiInfoResult{}, err
	}
	result.Counts = counts
	return result, nil
}

// countVault 跑 5 条 COUNT() 凑出 wiki_info 的 counts block。
//
// reviews 表在 W3 D10+ 才上线，pending_reviews 暂归 0；不查那张可能不存在
// 的表，避免 SQL 错误污染响应。
func countVault(ctx context.Context, db *index.DB) (CountsBlock, error) {
	out := CountsBlock{}
	sqlDB := db.SQL()
	if sqlDB == nil {
		return out, index.ErrIndexUnavailable
	}
	if err := scanInt(ctx, sqlDB, `SELECT COUNT(*) FROM sources`, &out.RawSources); err != nil {
		return out, fmt.Errorf("count sources: %w", err)
	}
	if err := scanInt(ctx, sqlDB, `SELECT COUNT(*) FROM pages`, &out.WikiPages); err != nil {
		return out, fmt.Errorf("count pages: %w", err)
	}
	for _, t := range []struct {
		typ string
		dst *int
	}{
		{"claim", &out.Claims},
		{"entity", &out.Entities},
		{"concept", &out.Concepts},
	} {
		if err := scanInt(ctx, sqlDB, `SELECT COUNT(*) FROM pages WHERE type = ?`, t.dst, t.typ); err != nil {
			return out, fmt.Errorf("count %s: %w", t.typ, err)
		}
	}
	return out, nil
}

func scanInt(ctx context.Context, db *sql.DB, q string, dst *int, args ...interface{}) error {
	return db.QueryRowContext(ctx, q, args...).Scan(dst)
}

// handleReadPage 实现 mcp-tools.md §3 read_page。
//
// page_id 路由：
//  1. 含 "/" 或 ".md" → 当 vault-relative path 处理，走 ParsePage（fs 读取）
//  2. 其它 → 当 page id，查 pages 表（GetPageByID）
//
// include_history / include_backlinks 返回空数组 + note，不报错——让 agent
// 看到 staged 行为而不是 NOT_IMPLEMENTED crash。
func (b *vaultBackend) handleReadPage(ctx context.Context, args ReadPageArgs) (ReadPageResult, error) {
	id := strings.TrimSpace(args.PageID)
	if id == "" {
		return ReadPageResult{}, fmt.Errorf("read_page: page_id is required")
	}

	var (
		result ReadPageResult
		err    error
	)
	if looksLikePath(id) {
		result, err = b.readPageFromPath(id)
	} else {
		result, err = b.readPageFromIndex(ctx, id)
	}
	if err != nil {
		return ReadPageResult{}, err
	}

	// staged: 始终返回空切片避免 JSON 输出 null（agent 友好）。
	result.History = []any{}
	result.Backlinks = []any{}
	if args.IncludeHistory {
		result.HistoryNote = historyNote
	}
	if args.IncludeBacklinks {
		result.BacklinksNote = backlinksNote
	}
	return result, nil
}

// looksLikePath 用启发式判断 page_id 是否其实是 vault-relative path：
// 含 "/" 或以 ".md" 结尾——这两种形态在 page id 命名规范里都不允许。
func looksLikePath(s string) bool {
	if strings.ContainsAny(s, "/\\") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(s), ".md")
}

// readPageFromIndex 直接走 SQLite pages 表，避免读 fs。
func (b *vaultBackend) readPageFromIndex(ctx context.Context, id string) (ReadPageResult, error) {
	row, err := index.GetPageByID(ctx, b.db, id)
	if err != nil {
		return ReadPageResult{}, fmt.Errorf("read_page: query index: %w", err)
	}
	if row == nil {
		return ReadPageResult{}, fmt.Errorf("%w: %s", ErrPageNotFound, id)
	}
	return pageRowToResult(row), nil
}

// readPageFromPath 通过 vault-relative path 走 fs：先 ResolveInVault 防 traversal,
// 再调 service.ParsePage 拿 frontmatter+body。
func (b *vaultBackend) readPageFromPath(rel string) (ReadPageResult, error) {
	abs, err := vault.ResolveInVault(rel, b.root)
	if err != nil {
		return ReadPageResult{}, fmt.Errorf("read_page: %w", err)
	}
	if _, statErr := os.Stat(abs); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return ReadPageResult{}, fmt.Errorf("%w: %s", ErrPageNotFound, rel)
		}
		return ReadPageResult{}, fmt.Errorf("read_page: stat: %w", statErr)
	}
	parsed, err := service.ParsePage(abs)
	if err != nil {
		return ReadPageResult{}, fmt.Errorf("read_page: parse: %w", err)
	}
	return parsedPageToResult(rel, parsed), nil
}

// pageRowToResult 把 SQLite pages 行映射为 MCP 响应。
func pageRowToResult(row *index.PageRow) ReadPageResult {
	res := ReadPageResult{
		ID:            row.ID,
		Type:          row.Type,
		Path:          row.Path,
		Title:         row.Title,
		Body:          row.Body,
		Status:        row.Status,
		SchemaVersion: row.SchemaVersion,
		Frontmatter:   row.Frontmatter,
	}
	if row.Confidence.Valid {
		v := row.Confidence.Float64
		res.Confidence = &v
	}
	return res
}

// parsedPageToResult 把 fs 解析结果映射为 MCP 响应。
//
// 与 pageRowToResult 不同，path 形态没经 reindex，所以 confidence / status
// 等字段从 frontmatter 直接抓——尽量贴近 pageRow 的语义。
// Frontmatter 字段也 JSON-marshal 一份保持与 by-id 路径输出 shape 一致。
func parsedPageToResult(rel string, p *service.ParsedPage) ReadPageResult {
	res := ReadPageResult{
		Path: filepath.ToSlash(rel),
		Body: p.Body,
	}
	if p.Frontmatter != nil {
		if v, ok := p.Frontmatter["id"].(string); ok {
			res.ID = v
		}
		if v, ok := p.Frontmatter["type"].(string); ok {
			res.Type = v
		}
		if v, ok := p.Frontmatter["title"].(string); ok {
			res.Title = v
		}
		if v, ok := p.Frontmatter["status"].(string); ok {
			res.Status = v
		}
		if v, ok := p.Frontmatter["schema_version"].(string); ok {
			res.SchemaVersion = v
		}
		switch c := p.Frontmatter["confidence"].(type) {
		case float64:
			res.Confidence = &c
		case int:
			f := float64(c)
			res.Confidence = &f
		}
		if raw, err := json.Marshal(p.Frontmatter); err == nil {
			res.Frontmatter = string(raw)
		}
	}
	if res.Title == "" {
		for _, h := range p.Headings {
			if h.Level == 1 && h.Text != "" {
				res.Title = h.Text
				break
			}
		}
	}
	return res
}

// handleReadRaw 实现 mcp-tools.md §4 read_raw。
//
// 严格限制：
//   - format=normalized → ErrFormatUnsupported（W2 D9 才上）
//   - raw_id 必须以 "raw/" 开头（normalize 后判断）—— 把 read_raw 锁在
//     raw/ 子树，wiki/ 用 read_page
//   - ResolveInVault 防 path traversal / symlink 逃逸
//   - 非 utf-8 文本（http.DetectContentType 嗅探 text/*）走 base64
func (b *vaultBackend) handleReadRaw(ctx context.Context, args ReadRawArgs) (ReadRawResult, error) {
	_ = ctx
	rawID := strings.TrimSpace(args.RawID)
	if rawID == "" {
		return ReadRawResult{}, fmt.Errorf("read_raw: raw_id is required")
	}
	format := strings.ToLower(strings.TrimSpace(args.Format))
	if format == "" {
		// spec 默认 normalized，但 D8 未实现；为不让空 input 直接报错，
		// 默认走 raw（content 字节）—— Decision §3 已记录。
		format = "raw"
	}
	if format == "normalized" {
		return ReadRawResult{}, fmt.Errorf("%w: %s", ErrFormatUnsupported, formatNormalizedNote)
	}
	if format != "raw" {
		return ReadRawResult{}, fmt.Errorf("read_raw: unknown format %q", args.Format)
	}

	posix := vault.NormalizePath(rawID)
	if !strings.HasPrefix(posix, "raw/") && posix != "raw" {
		return ReadRawResult{}, fmt.Errorf("%w: %s", ErrRawIDOutsideRaw, rawID)
	}

	abs, err := vault.ResolveInVault(posix, b.root)
	if err != nil {
		return ReadRawResult{}, fmt.Errorf("read_raw: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ReadRawResult{}, fmt.Errorf("%w: %s", ErrRawNotFound, rawID)
		}
		return ReadRawResult{}, fmt.Errorf("read_raw: stat: %w", err)
	}
	if info.IsDir() {
		return ReadRawResult{}, fmt.Errorf("read_raw: %s is a directory", rawID)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return ReadRawResult{}, fmt.Errorf("read_raw: read: %w", err)
	}

	res := ReadRawResult{
		RawID:  posix,
		Format: format,
		Bytes:  len(data),
	}
	if isTextual(data) {
		res.Content = string(data)
	} else {
		res.Content = base64.StdEncoding.EncodeToString(data)
		res.Encoding = "base64"
	}
	return res, nil
}

// isTextual 用 http.DetectContentType 嗅探前 512 字节；text/* 视为文本。
// 同时强校验 utf-8 合法性，防止把损坏的二进制当文本编返。
func isTextual(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	mime := http.DetectContentType(head)
	if !strings.HasPrefix(mime, "text/") && !strings.HasPrefix(mime, "application/json") &&
		!strings.HasPrefix(mime, "application/xml") {
		return false
	}
	return utf8.Valid(data)
}

// handleListIndex 实现 mcp-tools.md §7 list_index。
//
// 链路：ListPages（type filter，SQL 层）→ prefix filter（内存）→ slice
// limit/offset。pages 总数预计 < 10k，内存 filter + slice 完全够。
func (b *vaultBackend) handleListIndex(ctx context.Context, args ListIndexArgs) (ListIndexResult, error) {
	typeFilter := strings.TrimSpace(args.Type)
	if strings.EqualFold(typeFilter, "all") {
		typeFilter = ""
	}
	rows, err := index.ListPages(ctx, b.db, typeFilter)
	if err != nil {
		return ListIndexResult{}, fmt.Errorf("list_index: %w", err)
	}

	prefix := strings.TrimSpace(args.Prefix)
	if prefix != "" {
		filtered := rows[:0]
		for _, r := range rows {
			if strings.HasPrefix(r.Path, prefix) {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}
	total := len(rows)

	limit := 100
	if args.Limit != nil && *args.Limit > 0 {
		limit = *args.Limit
	}
	offset := 0
	if args.Offset != nil && *args.Offset > 0 {
		offset = *args.Offset
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	items := make([]*IndexItem, 0, end-offset)
	for _, r := range rows[offset:end] {
		item := &IndexItem{
			ID:     r.ID,
			Type:   r.Type,
			Path:   r.Path,
			Title:  r.Title,
			Status: r.Status,
		}
		if r.Confidence.Valid {
			v := r.Confidence.Float64
			item.Confidence = &v
		}
		items = append(items, item)
	}
	return ListIndexResult{Total: total, Items: items}, nil
}
