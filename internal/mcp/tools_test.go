package mcp

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/service"
)

// --- wiki_info ---

// TestHandleWikiInfoEmptyVault 空 vault：sources=0、pages 只有 init 时
// 写的 index.md/log.md（不进 pages 表，要先 reindex）→ wiki_pages=0，
// claims/entities/concepts/pending_reviews=0。
func TestHandleWikiInfoEmptyVault(t *testing.T) {
	ctx := context.Background()
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)

	got, err := b.handleWikiInfo(ctx, WikiInfoArgs{})
	if err != nil {
		t.Fatalf("handleWikiInfo: %v", err)
	}
	if got.VaultRoot != b.root {
		t.Errorf("VaultRoot = %q, want %q", got.VaultRoot, b.root)
	}
	if got.SchemaVersion != "1.0" {
		t.Errorf("SchemaVersion = %q, want 1.0", got.SchemaVersion)
	}
	if got.DaemonVersion != daemonVersion {
		t.Errorf("DaemonVersion = %q, want %q", got.DaemonVersion, daemonVersion)
	}
	if got.Counts.RawSources != 0 || got.Counts.WikiPages != 0 ||
		got.Counts.Claims != 0 || got.Counts.Entities != 0 ||
		got.Counts.Concepts != 0 || got.Counts.PendingReviews != 0 {
		t.Errorf("empty vault counts not all 0: %+v", got.Counts)
	}
	if got.Health.Score != 100 {
		t.Errorf("Health.Score = %d, want 100", got.Health.Score)
	}
}

// TestHandleWikiInfoSeededVault 写 1 claim + 1 entity + 1 source，reindex
// 后跑 wiki_info，counts 应反映真实数据。
func TestHandleWikiInfoSeededVault(t *testing.T) {
	ctx := context.Background()
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)

	seedClaim(t, b.root, "cl-001", "wiki/claims/c.md", "Claim A", "supported", 0.9)
	seedClaim(t, b.root, "en-001", "wiki/entities/e.md", "Entity A", "", 0)
	seedClaim(t, b.root, "co-001", "wiki/concepts/p.md", "Concept A", "", 0)
	// reindex 让 pages 表填满。
	if _, err := service.ReindexWiki(ctx, b.db, b.root); err != nil {
		t.Fatalf("reindex: %v", err)
	}
	// 直接插一行 source 模拟 ingest（避开 fs 复制）。
	if err := index.InsertSource(ctx, b.db, &index.SourceRow{
		RawID: "raw/inbox/x.md", SHA256: "deadbeef", Size: 10, MTime: 1,
		Status: "pending", IngestedAt: 1,
	}); err != nil {
		t.Fatalf("InsertSource: %v", err)
	}

	got, err := b.handleWikiInfo(ctx, WikiInfoArgs{})
	if err != nil {
		t.Fatalf("handleWikiInfo: %v", err)
	}
	if got.Counts.Claims != 1 {
		t.Errorf("Claims = %d, want 1", got.Counts.Claims)
	}
	if got.Counts.Entities != 1 {
		t.Errorf("Entities = %d, want 1", got.Counts.Entities)
	}
	if got.Counts.Concepts != 1 {
		t.Errorf("Concepts = %d, want 1", got.Counts.Concepts)
	}
	if got.Counts.RawSources != 1 {
		t.Errorf("RawSources = %d, want 1", got.Counts.RawSources)
	}
	// index.md / log.md 也会被 ReindexWiki 收进 pages（type=unknown），所以
	// WikiPages 应该 ≥ 3（3 个 seed + 2 个初始 md）。
	if got.Counts.WikiPages < 3 {
		t.Errorf("WikiPages = %d, want >= 3", got.Counts.WikiPages)
	}
}

// --- read_page ---

// TestHandleReadPageByID 用 reindex 后的 page id 查；命中 + frontmatter 透传。
func TestHandleReadPageByID(t *testing.T) {
	ctx := context.Background()
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)

	seedClaim(t, b.root, "cl-2026-05-21-001", "wiki/claims/wiki-compounding.md",
		"Wiki 是 compounding", "supported", 0.92)
	if _, err := service.ReindexWiki(ctx, b.db, b.root); err != nil {
		t.Fatalf("reindex: %v", err)
	}

	got, err := b.handleReadPage(ctx, ReadPageArgs{PageID: "cl-2026-05-21-001"})
	if err != nil {
		t.Fatalf("handleReadPage by id: %v", err)
	}
	if got.ID != "cl-2026-05-21-001" {
		t.Errorf("ID = %q, want cl-2026-05-21-001", got.ID)
	}
	if got.Type != "claim" {
		t.Errorf("Type = %q, want claim", got.Type)
	}
	if got.Title != "Wiki 是 compounding" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Confidence == nil || *got.Confidence != 0.92 {
		t.Errorf("Confidence = %v, want 0.92", got.Confidence)
	}
	if got.History == nil {
		t.Errorf("History should be empty slice, not nil")
	}
	if got.Backlinks == nil {
		t.Errorf("Backlinks should be empty slice, not nil")
	}
}

// TestHandleReadPageByPath path 形态绕过 SQLite 直接 ParsePage —— 验证
// looksLikePath 路由 + fs 读取通路。
func TestHandleReadPageByPath(t *testing.T) {
	ctx := context.Background()
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)

	seedClaim(t, b.root, "cl-by-path", "wiki/claims/by-path.md",
		"Hello path", "", 0)

	got, err := b.handleReadPage(ctx, ReadPageArgs{PageID: "wiki/claims/by-path.md"})
	if err != nil {
		t.Fatalf("handleReadPage by path: %v", err)
	}
	if got.ID != "cl-by-path" {
		t.Errorf("ID = %q, want cl-by-path", got.ID)
	}
	if !strings.Contains(got.Body, "# Hello path") {
		t.Errorf("Body missing heading: %q", got.Body)
	}
}

// TestHandleReadPageIncludeFlags 当 include_history/backlinks=true 应附带
// note，让 agent 看到 staged 行为。
func TestHandleReadPageIncludeFlags(t *testing.T) {
	ctx := context.Background()
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)

	seedClaim(t, b.root, "cl-flag", "wiki/claims/flag.md", "Flag", "", 0)
	if _, err := service.ReindexWiki(ctx, b.db, b.root); err != nil {
		t.Fatalf("reindex: %v", err)
	}
	got, err := b.handleReadPage(ctx, ReadPageArgs{
		PageID:           "cl-flag",
		IncludeHistory:   true,
		IncludeBacklinks: true,
	})
	if err != nil {
		t.Fatalf("handleReadPage: %v", err)
	}
	if got.HistoryNote == "" {
		t.Error("HistoryNote empty when IncludeHistory=true")
	}
	if got.BacklinksNote == "" {
		t.Error("BacklinksNote empty when IncludeBacklinks=true")
	}
}

// TestHandleReadPageNotFound id 与 path 两路 miss 都应返回 ErrPageNotFound。
func TestHandleReadPageNotFound(t *testing.T) {
	ctx := context.Background()
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)

	cases := []struct {
		name string
		id   string
	}{
		{"by id", "cl-no-such"},
		{"by path", "wiki/claims/no-such.md"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := b.handleReadPage(ctx, ReadPageArgs{PageID: tc.id})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, ErrPageNotFound) {
				t.Errorf("error = %v, want wraps ErrPageNotFound", err)
			}
		})
	}
}

// TestHandleReadPageEmptyID 缺 page_id 友好报错（避免 SQL NULL 查询）。
func TestHandleReadPageEmptyID(t *testing.T) {
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)
	if _, err := b.handleReadPage(context.Background(), ReadPageArgs{}); err == nil {
		t.Fatal("expected error for empty page_id")
	}
}

// --- read_raw ---

// TestHandleReadRawText 写一个 utf-8 markdown 到 raw/inbox/，读 raw 应
// 返回原文 + encoding 为空（不 base64）。
func TestHandleReadRawText(t *testing.T) {
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)

	body := "# Hello WikiMind\n\n中文也通。\n"
	mustWrite(t, filepath.Join(b.root, "raw/inbox/hello.md"), []byte(body))

	got, err := b.handleReadRaw(context.Background(), ReadRawArgs{
		RawID:  "raw/inbox/hello.md",
		Format: "raw",
	})
	if err != nil {
		t.Fatalf("handleReadRaw: %v", err)
	}
	if got.Content != body {
		t.Errorf("Content = %q, want %q", got.Content, body)
	}
	if got.Encoding != "" {
		t.Errorf("Encoding = %q, want empty for utf-8 text", got.Encoding)
	}
	if got.Bytes != len(body) {
		t.Errorf("Bytes = %d, want %d", got.Bytes, len(body))
	}
}

// TestHandleReadRawBinaryBase64 写一段 PNG magic header；嗅探为非 text
// → base64 encode + encoding="base64"。
func TestHandleReadRawBinaryBase64(t *testing.T) {
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)

	// PNG 文件头 + 几字节 binary。
	body := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01, 0x02}
	mustWrite(t, filepath.Join(b.root, "raw/attachments/pic.png"), body)

	got, err := b.handleReadRaw(context.Background(), ReadRawArgs{
		RawID:  "raw/attachments/pic.png",
		Format: "raw",
	})
	if err != nil {
		t.Fatalf("handleReadRaw: %v", err)
	}
	if got.Encoding != "base64" {
		t.Errorf("Encoding = %q, want base64", got.Encoding)
	}
	decoded, err := base64.StdEncoding.DecodeString(got.Content)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	if string(decoded) != string(body) {
		t.Errorf("decoded mismatch")
	}
}

// TestHandleReadRawDefaultFormatIsRaw 不指定 format → 默认 raw，避免
// 空 input 触发 ErrFormatUnsupported（Decision 决定不沿用 spec 的 normalized
// 默认，因为 D8 normalized 没实现）。
func TestHandleReadRawDefaultFormatIsRaw(t *testing.T) {
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)
	mustWrite(t, filepath.Join(b.root, "raw/inbox/x.md"), []byte("# x\n"))

	got, err := b.handleReadRaw(context.Background(), ReadRawArgs{RawID: "raw/inbox/x.md"})
	if err != nil {
		t.Fatalf("default format failed: %v", err)
	}
	if got.Format != "raw" {
		t.Errorf("Format = %q, want raw", got.Format)
	}
}

// TestHandleReadRawNormalizedUnsupported normalized 必须返回结构化错误，
// 不能 silently fallback——Decision 已记录。
func TestHandleReadRawNormalizedUnsupported(t *testing.T) {
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)
	mustWrite(t, filepath.Join(b.root, "raw/inbox/x.md"), []byte("# x\n"))

	_, err := b.handleReadRaw(context.Background(), ReadRawArgs{
		RawID: "raw/inbox/x.md", Format: "normalized",
	})
	if !errors.Is(err, ErrFormatUnsupported) {
		t.Fatalf("error = %v, want ErrFormatUnsupported", err)
	}
}

// TestHandleReadRawRejectsOutsideRaw 任何不指向 raw/ 的 raw_id 都拒绝——
// 避免 read_raw 被滥用读 wiki/。
func TestHandleReadRawRejectsOutsideRaw(t *testing.T) {
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)
	mustWrite(t, filepath.Join(b.root, "wiki/index.md"), []byte("# index\n"))

	_, err := b.handleReadRaw(context.Background(), ReadRawArgs{
		RawID: "wiki/index.md", Format: "raw",
	})
	if !errors.Is(err, ErrRawIDOutsideRaw) {
		t.Fatalf("error = %v, want ErrRawIDOutsideRaw", err)
	}
}

// TestHandleReadRawPathTraversal "../../etc/passwd" 等逃逸尝试必须拒绝。
// ResolveInVault 是兜底防线；这里 prefix check 通常先拒。
func TestHandleReadRawPathTraversal(t *testing.T) {
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)

	cases := []string{
		"raw/../../etc/passwd",
		"raw/inbox/../../wiki/index.md",
		"/etc/passwd",
	}
	for _, rel := range cases {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			_, err := b.handleReadRaw(context.Background(), ReadRawArgs{
				RawID: rel, Format: "raw",
			})
			if err == nil {
				t.Fatalf("path %q resolved without error", rel)
			}
		})
	}
}

// TestHandleReadRawNotFound 路径合法但文件不存在。
func TestHandleReadRawNotFound(t *testing.T) {
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)
	_, err := b.handleReadRaw(context.Background(), ReadRawArgs{
		RawID: "raw/inbox/never-exists.md", Format: "raw",
	})
	if !errors.Is(err, ErrRawNotFound) {
		t.Fatalf("error = %v, want ErrRawNotFound", err)
	}
}

// --- list_index ---

// TestHandleListIndexEmpty 空索引：total=0、items=[]，不返回 nil。
func TestHandleListIndexEmpty(t *testing.T) {
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)
	got, err := b.handleListIndex(context.Background(), ListIndexArgs{})
	if err != nil {
		t.Fatalf("handleListIndex: %v", err)
	}
	if got.Total != 0 {
		t.Errorf("Total = %d, want 0", got.Total)
	}
	if got.Items == nil {
		t.Error("Items should be empty slice, not nil")
	}
}

// TestHandleListIndexTypeFilter type=claim 只回 claim；entity / concept 被过滤掉。
func TestHandleListIndexTypeFilter(t *testing.T) {
	ctx := context.Background()
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)
	seedClaim(t, b.root, "cl-1", "wiki/claims/a.md", "A", "supported", 0.8)
	seedClaim(t, b.root, "en-1", "wiki/entities/a.md", "A", "", 0)
	seedClaim(t, b.root, "co-1", "wiki/concepts/a.md", "A", "", 0)
	if _, err := service.ReindexWiki(ctx, b.db, b.root); err != nil {
		t.Fatalf("reindex: %v", err)
	}

	got, err := b.handleListIndex(ctx, ListIndexArgs{Type: "claim"})
	if err != nil {
		t.Fatalf("handleListIndex: %v", err)
	}
	if got.Total != 1 || len(got.Items) != 1 {
		t.Fatalf("Total/items = %d/%d, want 1/1", got.Total, len(got.Items))
	}
	if got.Items[0].Type != "claim" || got.Items[0].ID != "cl-1" {
		t.Errorf("item = %+v, want cl-1/claim", got.Items[0])
	}
	if got.Items[0].Confidence == nil || *got.Items[0].Confidence != 0.8 {
		t.Errorf("Confidence = %v, want 0.8", got.Items[0].Confidence)
	}

	// type="all" 视同空过滤。
	gotAll, err := b.handleListIndex(ctx, ListIndexArgs{Type: "all"})
	if err != nil {
		t.Fatalf("type=all: %v", err)
	}
	if gotAll.Total < 3 {
		t.Errorf("type=all Total = %d, want >= 3", gotAll.Total)
	}
}

// TestHandleListIndexPrefix prefix 过滤命中目录前缀。
func TestHandleListIndexPrefix(t *testing.T) {
	ctx := context.Background()
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)
	seedClaim(t, b.root, "cl-1", "wiki/claims/foo.md", "Foo", "", 0)
	seedClaim(t, b.root, "cl-2", "wiki/claims/bar.md", "Bar", "", 0)
	seedClaim(t, b.root, "en-1", "wiki/entities/baz.md", "Baz", "", 0)
	if _, err := service.ReindexWiki(ctx, b.db, b.root); err != nil {
		t.Fatalf("reindex: %v", err)
	}

	got, err := b.handleListIndex(ctx, ListIndexArgs{Prefix: "wiki/claims/"})
	if err != nil {
		t.Fatalf("handleListIndex: %v", err)
	}
	if got.Total != 2 {
		t.Errorf("Total = %d, want 2 (claims only)", got.Total)
	}
	for _, item := range got.Items {
		if !strings.HasPrefix(item.Path, "wiki/claims/") {
			t.Errorf("item.Path = %q escapes prefix", item.Path)
		}
	}
}

// TestHandleListIndexLimitOffset 切片必须独立于 total——total 报全量过滤结果。
func TestHandleListIndexLimitOffset(t *testing.T) {
	ctx := context.Background()
	b, cleanup := newBackend(t)
	t.Cleanup(cleanup)
	for i := 0; i < 5; i++ {
		seedClaim(t, b.root,
			"cl-"+string(rune('a'+i)),
			"wiki/claims/c"+string(rune('a'+i))+".md",
			"T", "", 0)
	}
	if _, err := service.ReindexWiki(ctx, b.db, b.root); err != nil {
		t.Fatalf("reindex: %v", err)
	}

	limit := 2
	offset := 1
	got, err := b.handleListIndex(ctx, ListIndexArgs{
		Type: "claim", Limit: &limit, Offset: &offset,
	})
	if err != nil {
		t.Fatalf("handleListIndex: %v", err)
	}
	if got.Total != 5 {
		t.Errorf("Total = %d, want 5", got.Total)
	}
	if len(got.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(got.Items))
	}

	// offset 超界：返空切片，total 不变。
	bigOffset := 99
	got2, err := b.handleListIndex(ctx, ListIndexArgs{
		Type: "claim", Offset: &bigOffset,
	})
	if err != nil {
		t.Fatalf("handleListIndex offset overflow: %v", err)
	}
	if got2.Total != 5 || len(got2.Items) != 0 {
		t.Errorf("offset overflow: Total=%d, len=%d, want 5/0", got2.Total, len(got2.Items))
	}
}

// --- helpers ---

// newBackend 建一个 vault + opens index db，返回 backend + cleanup hook。
func newBackend(t *testing.T) (*vaultBackend, func()) {
	t.Helper()
	root := setupVault(t)
	db, err := index.Open(root)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	return &vaultBackend{root: root, db: db}, func() {
		_ = db.Close()
	}
}

// seedClaim 写一个最小可解析的 page markdown（type/id/title/optional
// confidence-status）到 vaultRoot 下相对路径。 confidence=0 → 不写
// confidence 行，让 frontmatter 不带这个字段。
func seedClaim(t *testing.T, root, id, rel, title, status string, confidence float64) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pageType := pageTypeFromPath(rel)
	var fm strings.Builder
	fm.WriteString("---\n")
	fm.WriteString("id: " + id + "\n")
	fm.WriteString("type: " + pageType + "\n")
	fm.WriteString("title: \"" + title + "\"\n")
	fm.WriteString("schema_version: \"1.0\"\n")
	if status != "" {
		fm.WriteString("status: " + status + "\n")
	}
	if confidence != 0 {
		fm.WriteString("confidence: " + strconvFloat(confidence) + "\n")
	}
	fm.WriteString("---\n\n# " + title + "\n")
	if err := os.WriteFile(abs, []byte(fm.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// pageTypeFromPath 把 wiki/claims/x.md → "claim"；让 seedClaim 复用同一 helper
// 同时写 entity / concept。
func pageTypeFromPath(rel string) string {
	switch {
	case strings.Contains(rel, "/claims/"):
		return "claim"
	case strings.Contains(rel, "/entities/"):
		return "entity"
	case strings.Contains(rel, "/concepts/"):
		return "concept"
	case strings.Contains(rel, "/sources/"):
		return "source"
	case strings.Contains(rel, "/topics/"):
		return "topic"
	default:
		return "unknown"
	}
}

// strconvFloat 是手写的极简 float→string，避免引 strconv import 只为单测。
// 仅覆盖 0.92 / 0.8 这种简单测试值。
func strconvFloat(f float64) string {
	if f == float64(int(f)) {
		return formatInt(int(f))
	}
	// 截 2 位小数足以覆盖测试用例（0.92, 0.8）。
	scaled := int(f*100 + 0.5)
	whole := scaled / 100
	frac := scaled % 100
	if frac%10 == 0 {
		frac /= 10
		return formatInt(whole) + "." + formatInt(frac)
	}
	out := formatInt(whole) + "."
	if frac < 10 {
		out += "0"
	}
	out += formatInt(frac)
	return out
}

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// mustWrite 写文件，必要时 mkdir parent。
func mustWrite(t *testing.T, abs string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
