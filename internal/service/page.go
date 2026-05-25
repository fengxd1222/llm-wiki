package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

// ErrInvalidFrontmatter 表示 yaml frontmatter 块格式损坏（未闭合、yaml 语法错）。
var ErrInvalidFrontmatter = errors.New("invalid frontmatter")

// ErrInvalidPage 表示 markdown 文件本身无法解析为 page（例如 ID 字段类型错）。
var ErrInvalidPage = errors.New("invalid page")

// Heading 描述一个 markdown heading。
type Heading struct {
	Level int    // 1..6
	Text  string // 纯文本（已去 markdown 内联）
}

// ParsedPage 是 ParsePage 的产物。
//
// Frontmatter 可能为空（兼容 index.md / log.md 这类无 frontmatter 的文件）。
// Body 不含 frontmatter 块。
// Outbounds 已按出现顺序去重。
type ParsedPage struct {
	Path        string         // 绝对路径
	Frontmatter map[string]any // 解码后的 yaml；无 frontmatter 时为 nil
	Body        string         // markdown 正文
	Headings    []Heading
	Outbounds   []string // [[id]] / [[id|alias]] 去 alias 后的 id 列表（去重）
}

// outboundRe 匹配 [[id]] / [[id|alias]] 形式的双链。
// 宽松：id 可含中文 / 任意字符（除 ]、| 外）。
var outboundRe = regexp.MustCompile(`\[\[([^\]\|]+)(?:\|[^\]]*)?\]\]`)

// frontmatterDelim 是 frontmatter 块的分隔行（"---"）。
const frontmatterDelim = "---"

// ParsePage 读取一个 markdown 文件，解析 frontmatter / body / heading / outbound 链接。
//
// 失败：
//   - 路径不可读 → 文件系统错误（fmt-wrapped）
//   - frontmatter 块未闭合 / yaml 解析失败 → ErrInvalidFrontmatter
func ParsePage(path string) (*ParsedPage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read page %s: %w", path, err)
	}

	fm, body, err := splitFrontmatter(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrInvalidFrontmatter, path, err)
	}

	page := &ParsedPage{
		Path:        path,
		Frontmatter: fm,
		Body:        string(body),
	}

	if err := parseMarkdownStructure(body, page); err != nil {
		return nil, fmt.Errorf("parse markdown %s: %w", path, err)
	}
	return page, nil
}

// splitFrontmatter 抓取开头 "---\n...\n---\n" 的 yaml frontmatter 块。
//
// 没有 frontmatter（首行非 ---）→ 返回 (nil, raw, nil)，兼容 index.md / log.md。
// 有起始 --- 但找不到结束 --- → ErrInvalidFrontmatter。
// yaml 解析失败 → ErrInvalidFrontmatter。
func splitFrontmatter(raw []byte) (map[string]any, []byte, error) {
	// 去 BOM 兼容 Windows / 编辑器
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}
	if !hasFrontmatterPrefix(raw) {
		return nil, raw, nil
	}

	// 找第二个 --- 行。按 \n 切，逐行扫描。
	lines := bytes.SplitN(raw, []byte("\n"), 2)
	if len(lines) < 2 {
		return nil, nil, errors.New("frontmatter delimiter not closed")
	}
	rest := lines[1]
	closeIdx := indexFrontmatterClose(rest)
	if closeIdx < 0 {
		return nil, nil, errors.New("frontmatter delimiter not closed")
	}
	yamlBlock := rest[:closeIdx]
	bodyStart := closeIdx + len(frontmatterDelim)
	// 跳过紧跟在 close --- 后的 \r 与 \n
	for bodyStart < len(rest) && (rest[bodyStart] == '\r' || rest[bodyStart] == '\n') {
		bodyStart++
		if bodyStart > 0 && rest[bodyStart-1] == '\n' {
			break
		}
	}
	body := rest[bodyStart:]

	var fm map[string]any
	if err := yaml.Unmarshal(yamlBlock, &fm); err != nil {
		return nil, nil, fmt.Errorf("yaml parse: %w", err)
	}
	if fm == nil {
		fm = map[string]any{}
	}
	return fm, body, nil
}

// hasFrontmatterPrefix 判断 raw 是否以 "---" + 换行起头。
func hasFrontmatterPrefix(raw []byte) bool {
	if len(raw) < len(frontmatterDelim) {
		return false
	}
	if string(raw[:len(frontmatterDelim)]) != frontmatterDelim {
		return false
	}
	// 紧跟 \r\n / \n / EOF 都算合法 frontmatter 头。
	after := raw[len(frontmatterDelim):]
	if len(after) == 0 {
		return false
	}
	return after[0] == '\n' || after[0] == '\r'
}

// indexFrontmatterClose 在 rest 中找首个独占一行的 "---" 行，返回其在 rest 中的起始 byte 偏移。
// 未找到返回 -1。
func indexFrontmatterClose(rest []byte) int {
	offset := 0
	for offset < len(rest) {
		lineEnd := bytes.IndexByte(rest[offset:], '\n')
		var line []byte
		if lineEnd < 0 {
			line = rest[offset:]
		} else {
			line = rest[offset : offset+lineEnd]
		}
		trimmed := bytes.TrimRight(line, "\r")
		if string(trimmed) == frontmatterDelim {
			return offset
		}
		if lineEnd < 0 {
			return -1
		}
		offset += lineEnd + 1
	}
	return -1
}

// parseMarkdownStructure 用 goldmark 解析 body，抽 heading + outbound [[id]]。
func parseMarkdownStructure(body []byte, page *ParsedPage) error {
	md := goldmark.New()
	reader := text.NewReader(body)
	doc := md.Parser().Parse(reader)

	seen := map[string]struct{}{}
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if h, ok := n.(*ast.Heading); ok {
			page.Headings = append(page.Headings, Heading{
				Level: h.Level,
				Text:  headingText(h, body),
			})
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return err
	}

	// outbound: 正则扫整段 body（goldmark 没有 [[wikilink]] 原生 AST，正则最简单可靠）
	matches := outboundRe.FindAllSubmatch(body, -1)
	for _, m := range matches {
		id := strings.TrimSpace(string(m[1]))
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		page.Outbounds = append(page.Outbounds, id)
	}
	return nil
}

// headingText 抽取 heading 节点的纯文本（拼接所有 Text 子节点）。
func headingText(h *ast.Heading, src []byte) string {
	var buf bytes.Buffer
	for c := h.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			buf.Write(v.Segment.Value(src))
		case *ast.String:
			buf.Write(v.Value)
		default:
			// 内联其它（code、emphasis 等）用其 raw segment 兜底。
			if t, ok := c.(interface{ Lines() *text.Segments }); ok {
				lines := t.Lines()
				if lines != nil {
					for i := 0; i < lines.Len(); i++ {
						seg := lines.At(i)
						buf.Write(seg.Value(src))
					}
				}
			}
		}
	}
	return strings.TrimSpace(buf.String())
}

// ReindexResult 报告 ReindexWiki 的结果。
type ReindexResult struct {
	Count   int      // 成功写入的 page 数
	Skipped []string // 跳过的文件（解析失败 / 路径）
}

// ReindexWiki 遍历 vaultRoot/wiki/**/*.md，把每个文件 UPSERT 进 pages 表。
//
// 跳过：
//   - 顶层 _ 前缀目录（_review / _worktrees / _reports）
//   - 解析失败的文件（记入 Skipped，不阻止整体）
//
// page id 解析顺序：
//  1. frontmatter "id" 字段
//  2. fallback：vault-relative POSIX path 的 basename（去 .md），如
//     "wiki/claims/wiki-is-compounding.md" → "wiki-is-compounding"
func ReindexWiki(ctx context.Context, db *index.DB, vaultRoot string) (*ReindexResult, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: index handle is nil", index.ErrIndexUnavailable)
	}
	if strings.TrimSpace(vaultRoot) == "" {
		return nil, fmt.Errorf("%w: vault root is empty", ErrInvalidVaultRoot)
	}
	absRoot, err := filepath.Abs(vaultRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve vault root: %w", err)
	}
	wikiDir := filepath.Join(absRoot, "wiki")
	info, err := os.Stat(wikiDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ReindexResult{}, nil
		}
		return nil, fmt.Errorf("stat wiki dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("wiki path is not a directory: %s", wikiDir)
	}

	// Clear stale page_links before full reindex.
	if err := index.DeleteAllPageLinks(ctx, db); err != nil {
		return nil, fmt.Errorf("clear page_links: %w", err)
	}

	res := &ReindexResult{}
	walkErr := filepath.WalkDir(wikiDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipDir(wikiDir, path, d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}

		page, err := ParsePage(path)
		if err != nil {
			res.Skipped = append(res.Skipped, path)
			return nil
		}
		row, err := buildPageRow(absRoot, path, page)
		if err != nil {
			res.Skipped = append(res.Skipped, path)
			return nil
		}
		if err := index.UpsertPage(ctx, db, row); err != nil {
			return fmt.Errorf("upsert %s: %w", path, err)
		}
		// Populate page_links from outbound [[...]] references.
		if len(page.Outbounds) > 0 {
			if err := index.ReplacePageLinks(ctx, db, row.ID, page.Outbounds); err != nil {
				return fmt.Errorf("page_links %s: %w", row.ID, err)
			}
		}
		res.Count++
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk wiki: %w", walkErr)
	}
	return res, nil
}

// shouldSkipDir 判断 wiki 子目录是否应跳过。
// 只跳过 wikiDir 直接子级的 _ 前缀目录（_review / _worktrees / _reports）。
func shouldSkipDir(wikiDir, path, name string) bool {
	if path == wikiDir {
		return false
	}
	parent := filepath.Dir(path)
	if parent != wikiDir {
		return false
	}
	return strings.HasPrefix(name, "_")
}

// buildPageRow 把 ParsedPage 转为 PageRow。
//
// 没有 frontmatter 的文件（index.md / log.md）也允许写入 pages 表，
// 用 path-derived id + type=unknown 兜底，避免 reindex 中途断流。
func buildPageRow(vaultRoot, absPath string, page *ParsedPage) (*index.PageRow, error) {
	rel, err := filepath.Rel(vaultRoot, absPath)
	if err != nil {
		return nil, fmt.Errorf("relpath: %w", err)
	}
	posixRel := filepath.ToSlash(rel)

	id := pageID(page.Frontmatter, posixRel)
	if id == "" {
		return nil, fmt.Errorf("%w: cannot derive id", ErrInvalidPage)
	}

	pageType := frontmatterString(page.Frontmatter, "type")
	if pageType == "" {
		pageType = "unknown"
	}

	title := frontmatterString(page.Frontmatter, "title")
	if title == "" {
		title = headingTitle(page.Headings, posixRel)
	}

	schemaVer := frontmatterString(page.Frontmatter, "schema_version")
	if schemaVer == "" {
		schemaVer = "0"
	}

	frontJSON, err := frontmatterJSON(page.Frontmatter)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal frontmatter: %v", ErrInvalidPage, err)
	}

	row := &index.PageRow{
		ID:            id,
		Type:          pageType,
		Path:          posixRel,
		Title:         title,
		Body:          page.Body,
		Status:        frontmatterString(page.Frontmatter, "status"),
		SchemaVersion: schemaVer,
		CreatedBy:     frontmatterString(page.Frontmatter, "created_by"),
		UpdatedBy:     frontmatterString(page.Frontmatter, "updated_by"),
		CreatedAt:     frontmatterTimestamp(page.Frontmatter, "created_at"),
		UpdatedAt:     frontmatterTimestamp(page.Frontmatter, "updated_at"),
		Frontmatter:   frontJSON,
	}
	if c, ok := frontmatterFloat(page.Frontmatter, "confidence"); ok {
		row.Confidence = sql.NullFloat64{Float64: c, Valid: true}
	}
	return row, nil
}

// pageID 优先用 frontmatter.id；否则用 path 的 basename（去 .md）。
func pageID(fm map[string]any, posixRel string) string {
	if id := frontmatterString(fm, "id"); id != "" {
		return id
	}
	base := filepath.Base(posixRel)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// headingTitle 取第一个 H1 作 title；无 H1 用 posixRel 兜底。
func headingTitle(hs []Heading, posixRel string) string {
	for _, h := range hs {
		if h.Level == 1 && h.Text != "" {
			return h.Text
		}
	}
	return posixRel
}

func frontmatterString(fm map[string]any, key string) string {
	if fm == nil {
		return ""
	}
	v, ok := fm[key]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return strings.TrimSpace(s)
	case fmt.Stringer:
		return strings.TrimSpace(s.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func frontmatterFloat(fm map[string]any, key string) (float64, bool) {
	if fm == nil {
		return 0, false
	}
	v, ok := fm[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// frontmatterTimestamp 把 created_at / updated_at 字段解释为 Unix 秒。
// 接受 ISO-8601 字符串、int / int64 / float64。失败返回 0。
func frontmatterTimestamp(fm map[string]any, key string) int64 {
	if fm == nil {
		return 0
	}
	v, ok := fm[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case time.Time:
		return n.Unix()
	case string:
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(n)); err == nil {
			return t.Unix()
		}
	}
	return 0
}

// frontmatterJSON 把 frontmatter map 序列化为稳定 JSON 字符串。
// 空 frontmatter → 空字符串（不写 NULL 之外的 "{}"，便于 LIKE 查询）。
func frontmatterJSON(fm map[string]any) (string, error) {
	if len(fm) == 0 {
		return "", nil
	}
	// yaml.v3 解码后嵌套 map 可能是 map[string]any，json.Marshal 直接吞。
	b, err := json.Marshal(fm)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
