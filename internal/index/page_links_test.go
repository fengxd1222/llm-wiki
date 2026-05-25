package index

import (
	"context"
	"testing"
)

func TestInsertAndQueryPageLinks(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	// Insert a page first (page_links references pages for orphan count)
	page := &PageRow{ID: "cl-a", Type: "claim", Path: "wiki/claims/a.md", Title: "Claim A"}
	if err := UpsertPage(ctx, db, page); err != nil {
		t.Fatalf("UpsertPage: %v", err)
	}
	page2 := &PageRow{ID: "en-b", Type: "entity", Path: "wiki/entities/b.md", Title: "Entity B"}
	if err := UpsertPage(ctx, db, page2); err != nil {
		t.Fatalf("UpsertPage: %v", err)
	}

	// Insert link cl-a -> en-b
	link := &PageLink{SourceID: "cl-a", TargetID: "en-b", LinkType: "ref"}
	if err := InsertPageLink(ctx, db, link); err != nil {
		t.Fatalf("InsertPageLink: %v", err)
	}

	// Idempotent insert
	if err := InsertPageLink(ctx, db, link); err != nil {
		t.Fatalf("InsertPageLink idempotent: %v", err)
	}

	// Query inbound for en-b
	inbound, err := InboundLinks(ctx, db, "en-b")
	if err != nil {
		t.Fatalf("InboundLinks: %v", err)
	}
	if len(inbound) != 1 || inbound[0].SourceID != "cl-a" {
		t.Fatalf("InboundLinks = %+v, want 1 from cl-a", inbound)
	}

	// Query outbound for cl-a
	outbound, err := OutboundLinks(ctx, db, "cl-a")
	if err != nil {
		t.Fatalf("OutboundLinks: %v", err)
	}
	if len(outbound) != 1 || outbound[0].TargetID != "en-b" {
		t.Fatalf("OutboundLinks = %+v, want 1 to en-b", outbound)
	}
}

func TestReplacePageLinks(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	// Setup pages
	for _, id := range []string{"src", "t1", "t2", "t3"} {
		p := &PageRow{ID: id, Type: "claim", Path: "wiki/claims/" + id + ".md", Title: id}
		if err := UpsertPage(ctx, db, p); err != nil {
			t.Fatalf("UpsertPage %s: %v", id, err)
		}
	}

	// Initial links: src -> t1, t2
	if err := ReplacePageLinks(ctx, db, "src", []string{"t1", "t2"}); err != nil {
		t.Fatalf("ReplacePageLinks: %v", err)
	}
	out, _ := OutboundLinks(ctx, db, "src")
	if len(out) != 2 {
		t.Fatalf("outbound = %d, want 2", len(out))
	}

	// Replace with src -> t2, t3 (t1 removed, t3 added)
	if err := ReplacePageLinks(ctx, db, "src", []string{"t2", "t3"}); err != nil {
		t.Fatalf("ReplacePageLinks 2: %v", err)
	}
	out, _ = OutboundLinks(ctx, db, "src")
	if len(out) != 2 {
		t.Fatalf("outbound after replace = %d, want 2", len(out))
	}
	targets := map[string]bool{}
	for _, l := range out {
		targets[l.TargetID] = true
	}
	if !targets["t2"] || !targets["t3"] {
		t.Fatalf("targets = %v, want t2 and t3", targets)
	}
	if targets["t1"] {
		t.Fatalf("t1 should have been removed")
	}
}

func TestCountOrphanPages(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	// Create 3 claim pages, 1 entity
	for _, p := range []*PageRow{
		{ID: "cl-1", Type: "claim", Path: "wiki/claims/1.md", Title: "C1"},
		{ID: "cl-2", Type: "claim", Path: "wiki/claims/2.md", Title: "C2"},
		{ID: "cl-3", Type: "claim", Path: "wiki/claims/3.md", Title: "C3"},
		{ID: "en-1", Type: "entity", Path: "wiki/entities/1.md", Title: "E1"},
	} {
		if err := UpsertPage(ctx, db, p); err != nil {
			t.Fatalf("UpsertPage: %v", err)
		}
	}

	// No links → all 4 are orphans
	count, err := CountOrphanPages(ctx, db, []string{"claim", "entity", "concept"})
	if err != nil {
		t.Fatalf("CountOrphanPages: %v", err)
	}
	if count != 4 {
		t.Fatalf("orphans = %d, want 4", count)
	}

	// Add link to cl-1 → now 3 orphans
	if err := InsertPageLink(ctx, db, &PageLink{SourceID: "en-1", TargetID: "cl-1"}); err != nil {
		t.Fatalf("InsertPageLink: %v", err)
	}
	count, err = CountOrphanPages(ctx, db, []string{"claim", "entity", "concept"})
	if err != nil {
		t.Fatalf("CountOrphanPages: %v", err)
	}
	if count != 3 {
		t.Fatalf("orphans = %d, want 3", count)
	}
}

func TestDeleteAllPageLinks(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	if err := InsertPageLink(ctx, db, &PageLink{SourceID: "a", TargetID: "b"}); err != nil {
		t.Fatalf("InsertPageLink: %v", err)
	}
	if err := InsertPageLink(ctx, db, &PageLink{SourceID: "c", TargetID: "d"}); err != nil {
		t.Fatalf("InsertPageLink: %v", err)
	}

	if err := DeleteAllPageLinks(ctx, db); err != nil {
		t.Fatalf("DeleteAllPageLinks: %v", err)
	}

	out, _ := OutboundLinks(ctx, db, "a")
	if len(out) != 0 {
		t.Fatalf("outbound after delete all = %d, want 0", len(out))
	}
}
