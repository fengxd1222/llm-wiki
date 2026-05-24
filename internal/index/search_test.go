package index

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSearchFTS5HitsChineseAndEnglish(t *testing.T) {
	// Verifies CJK trigram MATCH 与英文统一走 FTS5（cjk-tokenizer.md §3）。
	ctx := context.Background()
	db := openTempDB(t)
	seedSearchPages(t, db)

	cases := []struct {
		name string
		q    string
		want string // 期望命中的 page id（取第一条）
	}{
		{"chinese-3rune", "更值钱", "cl-1"},
		{"chinese-substring", "每一次", "cl-1"},
		{"english", "compounding", "cl-1"},
		{"english-body", "ingest", "cl-1"},
		{"mixed-en-token", "TypeScript", "cl-2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits, err := SearchFTS5(ctx, db, tc.q, 10)
			if err != nil {
				t.Fatalf("SearchFTS5(%q): %v", tc.q, err)
			}
			if len(hits) == 0 {
				t.Fatalf("SearchFTS5(%q) returned no hits", tc.q)
			}
			if hits[0].PageID != tc.want {
				t.Fatalf("SearchFTS5(%q) first hit = %s, want %s", tc.q, hits[0].PageID, tc.want)
			}
			if hits[0].Source != SearchSourceFTS5 {
				t.Fatalf("Source = %q, want %q", hits[0].Source, SearchSourceFTS5)
			}
			if hits[0].Snippet == "" {
				t.Fatalf("expected non-empty snippet for hit on %q", tc.q)
			}
		})
	}
}

func TestSearchFTS5SnippetMarksMatch(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)
	seedSearchPages(t, db)

	hits, err := SearchFTS5(ctx, db, "compounding", 5)
	if err != nil {
		t.Fatalf("SearchFTS5: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no hits")
	}
	if !strings.Contains(hits[0].Snippet, "«") || !strings.Contains(hits[0].Snippet, "»") {
		t.Fatalf("snippet missing «...» markers: %q", hits[0].Snippet)
	}
}

func TestSearchFTS5RankOrdersByBM25(t *testing.T) {
	// 同一关键词在多页出现，BM25 应让"更高密度页"排第一；这里用"wiki"做关键词：
	//   cl-1 body 含两次 "wiki"
	//   cl-3 body 只含一次 "wiki"
	// BM25 受体长 + 频次影响——cl-1 体长大但频次也高；最低限度断言两者都命中即可，
	// 并断言 rank 是单调（前者 rank <= 后者，bm25 越小越相关）。
	ctx := context.Background()
	db := openTempDB(t)
	seedSearchPages(t, db)

	hits, err := SearchFTS5(ctx, db, "wiki", 10)
	if err != nil {
		t.Fatalf("SearchFTS5: %v", err)
	}
	if len(hits) < 2 {
		t.Fatalf("expected >= 2 hits, got %d", len(hits))
	}
	for i := 1; i < len(hits); i++ {
		if hits[i-1].Score > hits[i].Score {
			t.Fatalf("hits not sorted by BM25 ascending: %+v", hits)
		}
	}
}

func TestSearchFTS5RespectsLimit(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)
	seedSearchPages(t, db)

	hits, err := SearchFTS5(ctx, db, "wiki", 1)
	if err != nil {
		t.Fatalf("SearchFTS5: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("limit not enforced: got %d hits", len(hits))
	}
}

func TestSearchFTS5EmptyQueryReturnsNil(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)
	seedSearchPages(t, db)

	hits, err := SearchFTS5(ctx, db, "   ", 5)
	if err != nil {
		t.Fatalf("SearchFTS5: %v", err)
	}
	if hits != nil {
		t.Fatalf("expected nil hits for empty query, got %v", hits)
	}
}

func TestSearchFTS5EmptyIndexReturnsErrIndexEmpty(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)
	_, err := SearchFTS5(ctx, db, "anything", 5)
	if !errors.Is(err, ErrIndexEmpty) {
		t.Fatalf("err = %v, want ErrIndexEmpty", err)
	}
}

func TestSearchLikeFindsShortCJKSubstring(t *testing.T) {
	// 短查询路径：trigram 无法命中 "值钱"（2 rune），LIKE 应捞到。
	ctx := context.Background()
	db := openTempDB(t)
	seedSearchPages(t, db)

	hits, err := SearchLike(ctx, db, "值钱", 10)
	if err != nil {
		t.Fatalf("SearchLike: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("LIKE returned no hits for 值钱")
	}
	if hits[0].Source != SearchSourceLike {
		t.Fatalf("Source = %q, want %q", hits[0].Source, SearchSourceLike)
	}
	if !strings.Contains(hits[0].Snippet, "«值钱»") {
		t.Fatalf("snippet missing «值钱» marker: %q", hits[0].Snippet)
	}
}

func TestSearchLikeMatchesTitleWhenBodyMisses(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)
	if err := UpsertPage(context.Background(), db, &PageRow{
		ID: "cl-title-only", Type: "claim", Path: "wiki/claims/x.md",
		Title: "Karpathy on LLM-as-OS", Body: "body has no match",
		SchemaVersion: "1.0",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	hits, err := SearchLike(ctx, db, "Karpathy", 5)
	if err != nil {
		t.Fatalf("SearchLike: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	if !strings.Contains(hits[0].Snippet, "«Karpathy»") {
		t.Fatalf("title snippet missing marker: %q", hits[0].Snippet)
	}
}

func TestSearchLikeEscapesPercentAndUnderscore(t *testing.T) {
	// "50%" 含 LIKE 元字符，必须转义为字面量。
	ctx := context.Background()
	db := openTempDB(t)
	if err := UpsertPage(ctx, db, &PageRow{
		ID: "cl-pct", Type: "claim", Path: "wiki/claims/pct.md",
		Title: "Discount", Body: "50% off only this week",
		SchemaVersion: "1.0",
	}); err != nil {
		t.Fatalf("upsert pct: %v", err)
	}
	if err := UpsertPage(ctx, db, &PageRow{
		ID: "cl-other", Type: "claim", Path: "wiki/claims/other.md",
		Title: "Other", Body: "50 off — no percent here",
		SchemaVersion: "1.0",
	}); err != nil {
		t.Fatalf("upsert other: %v", err)
	}

	hits, err := SearchLike(ctx, db, "50%", 5)
	if err != nil {
		t.Fatalf("SearchLike: %v", err)
	}
	if len(hits) != 1 || hits[0].PageID != "cl-pct" {
		t.Fatalf("LIKE percent escape failed: %+v", hits)
	}
}

func TestSearchLikeEmptyNeedleReturnsNil(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)
	seedSearchPages(t, db)
	hits, err := SearchLike(ctx, db, "", 5)
	if err != nil {
		t.Fatalf("SearchLike: %v", err)
	}
	if hits != nil {
		t.Fatalf("expected nil for empty needle, got %v", hits)
	}
}

func TestSearchLikeEmptyIndexReturnsErrIndexEmpty(t *testing.T) {
	ctx := context.Background()
	db := openTempDB(t)
	_, err := SearchLike(ctx, db, "anything", 5)
	if !errors.Is(err, ErrIndexEmpty) {
		t.Fatalf("err = %v, want ErrIndexEmpty", err)
	}
}

// seedSearchPages 写入 3 个固定 page，供多个测试复用，避免重复 fixture。
func seedSearchPages(t *testing.T, db *DB) {
	t.Helper()
	ctx := context.Background()
	pages := []*PageRow{
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
		if err := UpsertPage(ctx, db, p); err != nil {
			t.Fatalf("seed %s: %v", p.ID, err)
		}
	}
}
