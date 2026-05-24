package index

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrIndexEmpty 表示 pages 表为空——查询前提缺失，CLI 应提示 reindex。
var ErrIndexEmpty = errors.New("index empty")

// SearchHit 是一次检索命中。
//
// Score 在 FTS5 路径是 bm25() 返回值（越小越相关；负值在 SQLite ≥ 3.34 常见），
// LIKE / ripgrep 路径无 BM25 排序时统一记 0。
// Source 标注命中来源，便于 CLI debug 与 NDJSON 调用方分支。
type SearchHit struct {
	PageID  string
	Type    string
	Title   string
	Snippet string
	Score   float64
	Source  string // "fts5" | "like-fallback" | "ripgrep"
}

// SearchSourceFTS5 / SearchSourceLike / SearchSourceRipgrep 是 SearchHit.Source
// 的稳定标签，供路由层与测试断言。
const (
	SearchSourceFTS5    = "fts5"
	SearchSourceLike    = "like-fallback"
	SearchSourceRipgrep = "ripgrep"
)

// SearchFTS5 走 pages_fts trigram MATCH + BM25 排序。
//
// query 必须满足 trigram 最小长度（utf8.RuneCountInString >= 3），调用方负责路由。
// limit <= 0 时默认 20。
// snippet(..., -1, ...) 让 FTS5 自动挑命中所在列（title 或 body），保证标题命中
// 也能出现 «...» 标记；max_tokens=16 兼顾上下文与可读性（cjk-tokenizer.md §3）。
//
// pages 表空时返回 ErrIndexEmpty，避免静默"无命中"。
func SearchFTS5(ctx context.Context, db *DB, query string, limit int) ([]SearchHit, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if err := assertIndexNonEmpty(ctx, db); err != nil {
		return nil, err
	}

	const sqlText = `SELECT p.id, p.type, p.title,
       snippet(pages_fts, -1, '«', '»', '…', 16) AS snip,
       bm25(pages_fts) AS rank
FROM pages_fts JOIN pages p ON p.id = pages_fts.id
WHERE pages_fts MATCH ?
ORDER BY rank
LIMIT ?`

	rows, err := db.SQL().QueryContext(ctx, sqlText, q, limit)
	if err != nil {
		return nil, fmt.Errorf("fts5 search %q: %w", q, err)
	}
	defer rows.Close()

	var out []SearchHit
	for rows.Next() {
		var hit SearchHit
		if err := rows.Scan(&hit.PageID, &hit.Type, &hit.Title, &hit.Snippet, &hit.Score); err != nil {
			return nil, fmt.Errorf("scan fts5 hit: %w", err)
		}
		hit.Source = SearchSourceFTS5
		out = append(out, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fts5 hits: %w", err)
	}
	return out, nil
}

// SearchLike 是短查询 / FTS5 不可用 / ripgrep 缺失的兜底实现。
//
// 直接对 pages.title / pages.body 走 LIKE '%needle%'（大小写敏感——CJK 无大小写概念，
// 英文短查询走 trigram 即可，所以 LIKE 路径主要服务 CJK 短串）。
// snippet 在 Go 侧提取：抓 needle 第一次命中前后约 30 个 rune，加 « » 标记。
// 命中按 id 升序输出（无 BM25），保持确定性。
func SearchLike(ctx context.Context, db *DB, needle string, limit int) ([]SearchHit, error) {
	if db == nil || db.SQL() == nil {
		return nil, ErrIndexUnavailable
	}
	n := strings.TrimSpace(needle)
	if n == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if err := assertIndexNonEmpty(ctx, db); err != nil {
		return nil, err
	}

	// 显式 ESCAPE '\\'，把 escapeLikePattern 加的 \% / \_ 当字面量解释（SQLite LIKE
	// 默认无 escape，必须声明）。
	const sqlText = `SELECT id, type, title, body
FROM pages
WHERE title LIKE ? ESCAPE '\' OR body LIKE ? ESCAPE '\'
ORDER BY id
LIMIT ?`
	pattern := "%" + escapeLikePattern(n) + "%"
	rows, err := db.SQL().QueryContext(ctx, sqlText, pattern, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("like search %q: %w", n, err)
	}
	defer rows.Close()

	var out []SearchHit
	for rows.Next() {
		var (
			id, pageType, title, body string
		)
		if err := rows.Scan(&id, &pageType, &title, &body); err != nil {
			return nil, fmt.Errorf("scan like hit: %w", err)
		}
		out = append(out, SearchHit{
			PageID:  id,
			Type:    pageType,
			Title:   title,
			Snippet: extractSnippet(title, body, n),
			Source:  SearchSourceLike,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate like hits: %w", err)
	}
	return out, nil
}

// assertIndexNonEmpty 在跑搜索前确认 pages 表非空，返回 ErrIndexEmpty 让 CLI 友好提示。
func assertIndexNonEmpty(ctx context.Context, db *DB) error {
	var n int
	if err := db.SQL().QueryRowContext(ctx, `SELECT COUNT(*) FROM pages`).Scan(&n); err != nil {
		return fmt.Errorf("count pages: %w", err)
	}
	if n == 0 {
		return ErrIndexEmpty
	}
	return nil
}

// escapeLikePattern 转义 LIKE 元字符 % 与 _，避免 user 输入意外触发通配。
// 必须配合调用方的 SQL `ESCAPE '\'` 子句使用——SQLite LIKE 默认无 escape，
// 不声明 ESCAPE 时 `\%` / `\_` 会被当成两个字面字符（反斜杠 + 元字符）而非转义。
func escapeLikePattern(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)
	return r.Replace(s)
}

// snippetContextRunes 是 LIKE / ripgrep snippet 在命中前后保留的 rune 数。
const snippetContextRunes = 30

// extractSnippet 从 title 或 body 抽取包含 needle 的上下文片段。
// 优先在 body 中找；body 无命中则用 title。命中位置用 « » 包裹。
// 找不到任何命中（理论上 LIKE 已过滤）→ 返回 body 前若干 rune。
func extractSnippet(title, body, needle string) string {
	lowerNeedle := strings.ToLower(needle)
	if snip := snippetAround(body, needle, lowerNeedle); snip != "" {
		return snip
	}
	if snip := snippetAround(title, needle, lowerNeedle); snip != "" {
		return snip
	}
	return truncateRunes(body, snippetContextRunes*2)
}

// snippetAround 在 text 中找第一次出现 needle 的位置（大小写不敏感），
// 在两侧各保留 ~snippetContextRunes 个 rune；命中本体用 « » 包裹。
// 未命中返回空串。
func snippetAround(text, needle, lowerNeedle string) string {
	if text == "" || needle == "" {
		return ""
	}
	lowerText := strings.ToLower(text)
	idx := strings.Index(lowerText, lowerNeedle)
	if idx < 0 {
		return ""
	}
	// 把 byte 偏移转为 rune 偏移以便正确按字符截断。
	startByte := idx
	endByte := idx + len(needle)
	// 校正：strings.ToLower 对单 byte ASCII 长度不变；对 CJK 也 1:1，
	// 因此 byte 偏移与 lowerText/text 对齐。
	leftStart := backByteRunes(text, startByte, snippetContextRunes)
	rightEnd := forwardByteRunes(text, endByte, snippetContextRunes)

	var b strings.Builder
	if leftStart > 0 {
		b.WriteString("…")
	}
	b.WriteString(text[leftStart:startByte])
	b.WriteString("«")
	b.WriteString(text[startByte:endByte])
	b.WriteString("»")
	b.WriteString(text[endByte:rightEnd])
	if rightEnd < len(text) {
		b.WriteString("…")
	}
	return collapseWhitespace(b.String())
}

// backByteRunes 从 pos 向左退 n 个 rune，返回结果的 byte 偏移（夹紧到 0）。
func backByteRunes(s string, pos, n int) int {
	if pos <= 0 {
		return 0
	}
	count := 0
	i := pos
	for i > 0 && count < n {
		_, size := utf8.DecodeLastRuneInString(s[:i])
		if size == 0 {
			break
		}
		i -= size
		count++
	}
	return i
}

// forwardByteRunes 从 pos 向右前进 n 个 rune，返回结果的 byte 偏移（夹紧到 len(s)）。
func forwardByteRunes(s string, pos, n int) int {
	if pos >= len(s) {
		return len(s)
	}
	count := 0
	i := pos
	for i < len(s) && count < n {
		_, size := utf8.DecodeRuneInString(s[i:])
		if size == 0 {
			break
		}
		i += size
		count++
	}
	return i
}

// collapseWhitespace 把连续空白（含换行）压成单个空格，snippet 单行展示更整齐。
func collapseWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			if !prevSpace {
				b.WriteByte(' ')
			}
			prevSpace = true
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// truncateRunes 截断 s 到至多 maxRunes 个 rune，超长加 "…"。
func truncateRunes(s string, maxRunes int) string {
	if s == "" {
		return ""
	}
	count := 0
	for i := range s {
		if count >= maxRunes {
			return collapseWhitespace(s[:i]) + "…"
		}
		count++
	}
	return collapseWhitespace(s)
}

