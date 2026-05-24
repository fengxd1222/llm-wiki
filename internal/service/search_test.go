package service

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

// seedSearchPages 在 service 层重新写一份固定 fixture——index 层的同名 helper
// 不可跨包导出，复制 3 条同样的 page 保持两层测试断言独立可读。
func seedSearchPages(t *testing.T, db *index.DB) {
	t.Helper()
	ctx := context.Background()
	pages := []*index.PageRow{
		{
			ID: "cl-1", Type: "claim", Path: "wiki/claims/cl-1.md",
			Title:         "Wiki 是一个 compounding artifact",
			Body:          "每一次 ingest、每一次 query 都让 wiki 更值钱。wiki is wiki.",
			SchemaVersion: "1.0",
		},
		{
			ID: "cl-2", Type: "concept", Path: "wiki/concepts/cl-2.md",
			Title:         "TypeScript stack note",
			Body:          "使用 TypeScript 开发前端，后端 Go。",
			SchemaVersion: "1.0",
		},
		{
			ID: "cl-3", Type: "entity", Path: "wiki/entities/cl-3.md",
			Title:         "Karpathy",
			Body:          "Andrej Karpathy talks about wiki and software-as-knowledge.",
			SchemaVersion: "1.0",
		},
	}
	for _, p := range pages {
		if err := index.UpsertPage(ctx, db, p); err != nil {
			t.Fatalf("seed %s: %v", p.ID, err)
		}
	}
}

func TestSearchRoutesLongQueryToFTS5(t *testing.T) {
	// 长查询 (>= 3 runes) 默认走 FTS5——断言 Source 标签。
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)
	seedSearchPages(t, db)

	hits, err := Search(ctx, db, vaultRoot, "compounding", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one FTS5 hit")
	}
	if hits[0].Source != index.SearchSourceFTS5 {
		t.Fatalf("Source = %q, want %q", hits[0].Source, index.SearchSourceFTS5)
	}
}

func TestSearchRoutesShortQueryToLike(t *testing.T) {
	// 短查询 (< 3 runes) 必须 fallback LIKE——
	// "值钱" 在 trigram 上无法命中，但 LIKE 能从 body 抓到。
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)
	seedSearchPages(t, db)

	hits, err := Search(ctx, db, vaultRoot, "值钱", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one LIKE hit for 值钱")
	}
	if hits[0].Source != index.SearchSourceLike {
		t.Fatalf("Source = %q, want %q (LIKE fallback)", hits[0].Source, index.SearchSourceLike)
	}
	if hits[0].PageID != "cl-1" {
		t.Fatalf("PageID = %s, want cl-1", hits[0].PageID)
	}
}

func TestSearchEmptyQueryReturnsNil(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)
	seedSearchPages(t, db)

	hits, err := Search(ctx, db, vaultRoot, "   ", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if hits != nil {
		t.Fatalf("expected nil for empty query, got %v", hits)
	}
}

func TestSearchNilDBReturnsErrIndexUnavailable(t *testing.T) {
	_, err := Search(context.Background(), nil, "/tmp/anywhere", "x", SearchOptions{})
	if !errors.Is(err, index.ErrIndexUnavailable) {
		t.Fatalf("err = %v, want ErrIndexUnavailable", err)
	}
}

func TestSearchHonorsLimit(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)
	seedSearchPages(t, db)

	hits, err := Search(ctx, db, vaultRoot, "wiki", SearchOptions{Limit: 1})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("limit not enforced: %d hits", len(hits))
	}
}

func TestSearchEmptyIndexReturnsErrIndexEmpty(t *testing.T) {
	// 空 vault → ErrIndexEmpty 透传到 service 层，CLI 能 errors.Is 检测。
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	_, err := Search(ctx, db, vaultRoot, "anything", SearchOptions{})
	if !errors.Is(err, ErrIndexEmpty) {
		t.Fatalf("err = %v, want ErrIndexEmpty", err)
	}
}

// TestSearchNoIndexFlagPrefersRipgrep 当 rg 可用时验证 NoIndex 路径返回
// ripgrep 来源的命中；rg 不可用时验证 silently 降级 LIKE（Source=like-fallback）。
// 一个测试覆盖两条互斥环境分支，避免 CI 矩阵漏掉无 rg 的机器。
func TestSearchNoIndexFlagPrefersRipgrep(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)
	seedSearchPages(t, db)

	hits, err := Search(ctx, db, vaultRoot, "compounding", SearchOptions{NoIndex: true})
	if err != nil {
		t.Fatalf("Search NoIndex: %v", err)
	}

	_, lookErr := exec.LookPath("rg")
	if lookErr == nil {
		// rg 可用：但 vault 是 t.TempDir() 下新建，wiki/ 里只有默认 index.md 和
		// log.md——seeded "compounding" 不会出现在磁盘 md 文件里。所以 rg 应该
		// 找不到，service 层不报错、返回空命中。
		for _, h := range hits {
			if h.Source != index.SearchSourceRipgrep {
				t.Fatalf("expected ripgrep source when rg available, got %q (hit %+v)",
					h.Source, h)
			}
		}
		return
	}

	// rg 不可用：降级 LIKE，hits 必须有内容且 Source=like-fallback。
	if len(hits) == 0 {
		t.Fatal("expected LIKE fallback hits when rg missing")
	}
	if hits[0].Source != index.SearchSourceLike {
		t.Fatalf("Source = %q, want %q (LIKE fallback)", hits[0].Source, index.SearchSourceLike)
	}
}

// TestSearchRegexFlagPrefersRipgrep mirrors NoIndex test for Regex 分支——
// Regex 必须走 rg；rg 缺失同样 LIKE 降级。
func TestSearchRegexFlagPrefersRipgrep(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)
	seedSearchPages(t, db)

	hits, err := Search(ctx, db, vaultRoot, "compoun.*", SearchOptions{Regex: true})
	if err != nil {
		t.Fatalf("Search Regex: %v", err)
	}

	if _, lookErr := exec.LookPath("rg"); lookErr != nil {
		// rg 缺失：LIKE 降级走字面量匹配 "compoun.*"——seeded body 不含此字面量，
		// 命中应该为空但不报错。
		_ = hits
		return
	}
	// rg 可用但磁盘 wiki/ 没有 seeded 内容——0 命中且不报错即合格。
	for _, h := range hits {
		if h.Source != index.SearchSourceRipgrep {
			t.Fatalf("Source = %q, want %q", h.Source, index.SearchSourceRipgrep)
		}
	}
}

// TestParseVimgrepLine 锁住 ripgrep --vimgrep 解析的关键边界：
// 1. 标准 unix 路径 → 3 个 ":" 都是分隔符
// 2. Windows 盘符 (C:\...:N:M:text) → 必须从右往左切，盘符冒号不能误吞
// 3. 缺字段 → 优雅返回 ok=false 不 panic
func TestParseVimgrepLine(t *testing.T) {
	cases := []struct {
		name        string
		line        string
		root        string
		needle      string
		wantOK      bool
		wantSnippet string
	}{
		{
			name:        "unix-path",
			line:        "/vault/wiki/claims/cl-1.md:12:5:Wiki is compounding",
			root:        "/vault",
			needle:      "compounding",
			wantOK:      true,
			wantSnippet: "Wiki is «compounding»",
		},
		{
			name:        "windows-path",
			line:        `C:\vault\wiki\claims\cl-1.md:7:3:some text here`,
			root:        `C:\vault`,
			needle:      "text",
			wantOK:      true,
			wantSnippet: "some «text» here",
		},
		{
			name:   "missing-fields",
			line:   "no-colons-here",
			root:   "/vault",
			needle: "x",
			wantOK: false,
		},
		{
			name:   "empty-line",
			line:   "",
			root:   "/vault",
			needle: "x",
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hit, ok := parseVimgrepLine(tc.line, tc.root, tc.needle)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (hit=%+v)", ok, tc.wantOK, hit)
			}
			if !ok {
				return
			}
			if tc.wantSnippet != "" && !strings.Contains(hit.Snippet, tc.wantSnippet) {
				t.Fatalf("snippet = %q, want contains %q", hit.Snippet, tc.wantSnippet)
			}
			if hit.Source != index.SearchSourceRipgrep {
				t.Fatalf("Source = %q, want %q", hit.Source, index.SearchSourceRipgrep)
			}
		})
	}
}

// TestSnippetFromText 锁定 « » 标记：大小写不敏感，未命中返回 trim 后原文。
func TestSnippetFromText(t *testing.T) {
	cases := []struct {
		name        string
		text        string
		needle      string
		want        string
	}{
		{"basic", "Wiki is compounding fast", "compounding", "Wiki is «compounding» fast"},
		{"case-insensitive", "WIKI is COMPOUNDING", "compounding", "WIKI is «COMPOUNDING»"},
		{"miss-returns-trim", "  no match here  ", "absent", "no match here"},
		{"empty-needle", "anything", "", "anything"},
		{"empty-text", "", "x", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := snippetFromText(tc.text, tc.needle)
			if got != tc.want {
				t.Fatalf("snippetFromText(%q,%q) = %q, want %q", tc.text, tc.needle, got, tc.want)
			}
		})
	}
}
