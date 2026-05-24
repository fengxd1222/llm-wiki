package service

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SourcePageRelDir 是 source page 在 vault 内的相对目录（POSIX 风格）。
const SourcePageRelDir = "wiki/sources"

// SourcePageType 是 source page 的 frontmatter type 字段值。
const SourcePageType = "source"

// SourcePageResult 描述一次 EnsureSourcePage 调用的结果。
//
// RelPath / AbsPath 总是填充（即便 Existed=true）；Created=false 表示文件
// 已存在，本次调用未改写。
type SourcePageResult struct {
	RelPath string // vault-relative POSIX 路径，例如 "wiki/sources/sample.md"
	AbsPath string
	Created bool
	Title   string
}

// EnsureSourcePage 在 vaultRoot/wiki/sources/<raw-basename>.md 生成 source page；
// 文件已存在时跳过（幂等保证）。
//
// rawRelID 是 vault-relative POSIX 路径（"raw/inbox/<basename>"），与 ingest
// 写入 sources.raw_id 一致。
//
// title 解析顺序：
//  1. raw frontmatter "title"
//  2. raw 首个 H1 heading 的纯文本
//  3. basename 去扩展名
//
// 失败：
//   - rawRelID 不指向 raw/inbox/ 下的文件 → 路径错误
//   - raw 文件不可读 → 文件系统错误
//   - frontmatter 解析失败（既有 ParsePage 行为）→ ErrInvalidFrontmatter
func EnsureSourcePage(vaultRoot, rawRelID string) (*SourcePageResult, error) {
	if strings.TrimSpace(vaultRoot) == "" {
		return nil, fmt.Errorf("%w: vault root is empty", ErrInvalidVaultRoot)
	}
	rawRelID = strings.TrimSpace(rawRelID)
	if rawRelID == "" {
		return nil, errors.New("source page: rawRelID is empty")
	}
	// rawRelID 是 POSIX 风格 vault-relative；用 path 语义切 basename 避免
	// Windows 上 filepath.Base 把 "raw/inbox/x.md" 当单段路径。
	posixRel := filepath.ToSlash(rawRelID)
	base := posixBase(posixRel)
	if base == "" {
		return nil, fmt.Errorf("source page: cannot derive basename from %q", rawRelID)
	}

	rawAbs := filepath.Join(vaultRoot, filepath.FromSlash(posixRel))
	rawInfo, err := os.Stat(rawAbs)
	if err != nil {
		return nil, fmt.Errorf("source page: stat raw %s: %w", rawAbs, err)
	}
	if rawInfo.IsDir() {
		return nil, fmt.Errorf("source page: raw path is a directory: %s", rawAbs)
	}

	pageID := strings.TrimSuffix(base, filepath.Ext(base))
	relPath := SourcePageRelDir + "/" + pageID + ".md"
	absPath := filepath.Join(vaultRoot, filepath.FromSlash(relPath))

	// 幂等：source page 已存在 → 跳过创建，保留 user 可能的人手编辑。
	if _, err := os.Stat(absPath); err == nil {
		return &SourcePageResult{
			RelPath: relPath,
			AbsPath: absPath,
			Created: false,
			Title:   pageID,
		}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("source page: stat existing %s: %w", absPath, err)
	}

	title, err := sourcePageTitle(rawAbs, pageID)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, fmt.Errorf("source page: mkdir %s: %w", filepath.Dir(absPath), err)
	}
	body, err := renderSourcePage(pageID, title, posixRel)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(absPath, body, 0o644); err != nil {
		return nil, fmt.Errorf("source page: write %s: %w", absPath, err)
	}

	return &SourcePageResult{
		RelPath: relPath,
		AbsPath: absPath,
		Created: true,
		Title:   title,
	}, nil
}

// sourcePageTitle 按 frontmatter title → first H1 → basename 顺序解析 title。
//
// raw 不是 markdown / 解析空 → 返回 fallback（基于 basename）。
// frontmatter yaml 解析失败（ErrInvalidFrontmatter）→ 降级到 fallback，
// 不阻塞 ingest（W1 简化：raw 是用户/外部世界拷进来的，损坏的 frontmatter
// 不应该让整个 ingest 失败；真正不可读的文件已在 openSource 阶段拦截）。
// 其它解析错误透传（保留 stack 给上层 wrap）。
func sourcePageTitle(rawAbs, fallback string) (string, error) {
	// 非 markdown 文件直接返回 fallback——ParsePage 会读全文，对二进制无意义。
	if !strings.EqualFold(filepath.Ext(rawAbs), ".md") {
		return fallback, nil
	}
	parsed, err := ParsePage(rawAbs)
	if err != nil {
		if errors.Is(err, ErrInvalidFrontmatter) {
			return fallback, nil
		}
		return "", err
	}
	if t := frontmatterString(parsed.Frontmatter, "title"); t != "" {
		return t, nil
	}
	for _, h := range parsed.Headings {
		if h.Level == 1 && strings.TrimSpace(h.Text) != "" {
			return strings.TrimSpace(h.Text), nil
		}
	}
	return fallback, nil
}

// renderSourcePage 渲染 source page markdown 内容。
//
// 关键设计（prd ADR-lite）：body 仅占位，不复制 raw 全文——
// 保留 raw 作为 source of truth，避免双写漂移。
func renderSourcePage(id, title, sourcePath string) ([]byte, error) {
	fm := map[string]any{
		"id":           id,
		"type":         SourcePageType,
		"title":        title,
		"source_path":  sourcePath, // POSIX 风格，跨平台统一
		"ingested_at":  time.Now().UTC().Format(time.RFC3339),
	}
	var yamlBuf bytes.Buffer
	enc := yaml.NewEncoder(&yamlBuf)
	enc.SetIndent(2)
	if err := enc.Encode(fm); err != nil {
		return nil, fmt.Errorf("source page: encode frontmatter: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("source page: close yaml encoder: %w", err)
	}

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(yamlBuf.Bytes())
	out.WriteString("---\n\n")
	out.WriteString("# ")
	out.WriteString(title)
	out.WriteString("\n\n")
	out.WriteString("Source ingested from `")
	out.WriteString(sourcePath)
	out.WriteString("`. See raw file for full content.\n")
	return out.Bytes(), nil
}

// posixBase 返回 POSIX 风格路径的最后一段。
// 不依赖 filepath.Base，避免 Windows 上把 "raw/inbox/x.md" 当单段。
func posixBase(p string) string {
	idx := strings.LastIndexByte(p, '/')
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}
