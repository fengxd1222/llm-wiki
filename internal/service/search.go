package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

// SearchOptions 控制 service.Search 的路由与展示。
//
// NoIndex 强制走 ripgrep（不存在则降级 LIKE）；Regex 同上但 needle 按正则传入。
// Limit <= 0 时默认 20。
type SearchOptions struct {
	NoIndex bool
	Regex   bool
	Limit   int
}

const (
	// trigramMinRunes 是 FTS5 trigram tokenizer 的最小可匹配查询长度。
	// 低于此长度走 LIKE / ripgrep 兜底（cjk-tokenizer.md §3.3）。
	trigramMinRunes = 3

	// defaultSearchLimit 与 CLI flag default 对齐。
	defaultSearchLimit = 20

	// ripgrepExe 与 ripgrepNotFound 是 exec.LookPath 检查的常量句柄。
	ripgrepExe = "rg"
)

// ErrIndexEmpty 透传 index 层的同名 sentinel，让 CLI 在 service 层就能用 errors.Is 判定。
var ErrIndexEmpty = index.ErrIndexEmpty

// Search 是 query 命令的入口路由器。
//
// 路由优先级（cjk-tokenizer.md §3.3）：
//  1. NoIndex / Regex → ripgrep（不存在则 LIKE 兜底，silently 不报错）
//  2. RuneCount(query) < 3 → LIKE
//  3. 默认 → FTS5 trigram BM25
//
// 任何路径下空 query 返回 (nil, nil)。
func Search(
	ctx context.Context,
	db *index.DB,
	vaultRoot, query string,
	opts SearchOptions,
) ([]index.SearchHit, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: index handle is nil", index.ErrIndexUnavailable)
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	if opts.NoIndex || opts.Regex {
		hits, err := searchRipgrep(ctx, vaultRoot, q, opts.Regex, limit)
		if err == nil {
			return hits, nil
		}
		if !errors.Is(err, errRipgrepUnavailable) {
			return nil, err
		}
		// ripgrep 不可用 → 降级 LIKE。Regex 模式下 LIKE 无法等价正则，
		// 此时按字面量匹配兜底（与 ripgrep 不存在的妥协一致），
		// 调用方可见 hit.Source 区分。
		return index.SearchLike(ctx, db, q, limit)
	}

	if utf8.RuneCountInString(q) < trigramMinRunes {
		return index.SearchLike(ctx, db, q, limit)
	}
	return index.SearchFTS5(ctx, db, q, limit)
}

// errRipgrepUnavailable 是 service 内部 sentinel：ripgrep 不在 PATH，
// 让 Search 决定是否降级 LIKE（外部调用方不需要看见此错误）。
var errRipgrepUnavailable = errors.New("ripgrep not available")

// searchRipgrep 在 vaultRoot/wiki/ 跑 ripgrep。
//
// fixed=false 时按正则传入；fixed=true 时加 --fixed-strings 把 needle 当字面量。
// ripgrep 二进制不存在 → 返回 errRipgrepUnavailable 让 caller 兜底。
// ripgrep 退出码 1（无匹配）不视为错误。
// 输出格式：--vimgrep（`file:line:col:text`），每行解析为一个 SearchHit。
//
// 返回的 SearchHit.PageID 是 vault-relative POSIX 路径（无 frontmatter id 时
// 与 service.buildPageRow 的 fallback 一致——避免与 SQLite 中 id 强绑定，
// ripgrep 路径上 page id 仅用于展示）。
func searchRipgrep(ctx context.Context, vaultRoot, needle string, regex bool, limit int) ([]index.SearchHit, error) {
	if _, err := exec.LookPath(ripgrepExe); err != nil {
		return nil, errRipgrepUnavailable
	}
	root, err := filepath.Abs(vaultRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve vault root: %w", err)
	}
	wikiDir := filepath.Join(root, "wiki")

	args := []string{"--type=md", "--vimgrep", "--no-heading", "--color=never"}
	if !regex {
		args = append(args, "--fixed-strings")
	}
	// --max-count 是 ripgrep 的"每文件命中上限"——不是全局总数限制。
	// 用 limit 兜底单文件爆炸（一个超长 markdown 不会吐出几百行），
	// 全局 limit 在 parseRipgrepVimgrep 中实施。
	args = append(args, "--max-count="+strconv.Itoa(limit))
	args = append(args, "--", needle, wikiDir)

	cmd := exec.CommandContext(ctx, ripgrepExe, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			// rg exit code 1 = no matches，不是错误。
			return nil, nil
		}
		return nil, fmt.Errorf("ripgrep: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	return parseRipgrepVimgrep(stdout.Bytes(), root, needle, limit), nil
}

// parseRipgrepVimgrep 解析 --vimgrep 输出。
//
// 行格式：absPath:line:col:matchedText
// 每行视为一次命中；同一文件多行各算一次（与 BM25 排序不同，确定性 = 文件遍历顺序）。
func parseRipgrepVimgrep(out []byte, vaultRoot, needle string, limit int) []index.SearchHit {
	var hits []index.SearchHit
	scanner := bufio.NewScanner(bytes.NewReader(out))
	// 默认 buffer 64KB 对长行（长 markdown 段落）可能不够；放宽到 1MB。
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if len(hits) >= limit {
			break
		}
		line := scanner.Text()
		if line == "" {
			continue
		}
		hit, ok := parseVimgrepLine(line, vaultRoot, needle)
		if !ok {
			continue
		}
		hits = append(hits, hit)
	}
	return hits
}

// parseVimgrepLine 解析单行 --vimgrep 输出。
//
// Windows 路径含盘符冒号（`C:\...:line:col:text`），需要从右往左切前 3 个 ":"。
func parseVimgrepLine(line, vaultRoot, needle string) (index.SearchHit, bool) {
	// 从右往左找 3 个 ":"：col / line / 之前的全是路径。
	idxColEnd := strings.LastIndex(line, ":")
	if idxColEnd <= 0 {
		return index.SearchHit{}, false
	}
	textPart := line[idxColEnd+1:]
	rest := line[:idxColEnd]

	idxLineEnd := strings.LastIndex(rest, ":")
	if idxLineEnd <= 0 {
		return index.SearchHit{}, false
	}
	rest = rest[:idxLineEnd]

	idxPathEnd := strings.LastIndex(rest, ":")
	if idxPathEnd <= 0 {
		return index.SearchHit{}, false
	}
	pathPart := rest[:idxPathEnd]

	relPath := pathPart
	if abs, err := filepath.Abs(pathPart); err == nil {
		if rel, err := filepath.Rel(vaultRoot, abs); err == nil {
			relPath = filepath.ToSlash(rel)
		}
	}

	// page id 用 basename（与 service.pageID 的 fallback 对齐——
	// reindex 后若有 frontmatter id 优先 SQLite 路径，这里 ripgrep 不读 frontmatter）。
	base := filepath.Base(relPath)
	pageID := strings.TrimSuffix(base, filepath.Ext(base))

	return index.SearchHit{
		PageID:  pageID,
		Type:    "ripgrep",
		Title:   relPath,
		Snippet: snippetFromText(textPart, needle),
		Source:  index.SearchSourceRipgrep,
	}, true
}

// snippetFromText 用 « » 标记 needle 在 text 中的命中，并裁剪首尾空白。
// 大小写不敏感；未命中（理论上 ripgrep 已过滤）→ 原文 trim 后返回。
func snippetFromText(text, needle string) string {
	t := strings.TrimSpace(text)
	if t == "" || needle == "" {
		return t
	}
	lowerT := strings.ToLower(t)
	lowerN := strings.ToLower(needle)
	idx := strings.Index(lowerT, lowerN)
	if idx < 0 {
		return t
	}
	end := idx + len(needle)
	return t[:idx] + "«" + t[idx:end] + "»" + t[end:]
}
