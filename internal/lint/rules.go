package lint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

// OrphanRule detects pages with no inbound or outbound links.
type OrphanRule struct{}

func (r *OrphanRule) Name() string { return "orphan" }
func (r *OrphanRule) Run(ctx context.Context, vaultRoot string, db *index.DB) []Finding {
	var findings []Finding
	pages, err := index.ListPages(ctx, db, "")
	if err != nil {
		return nil
	}
	for _, p := range pages {
		if p.Type == "unknown" || p.Type == "source" {
			continue // skip non-content pages
		}
		inbound, _ := index.InboundLinks(ctx, db, p.ID)
		outbound, _ := index.OutboundLinks(ctx, db, p.ID)
		if len(inbound) == 0 && len(outbound) == 0 {
			findings = append(findings, Finding{
				Rule:            "orphan",
				PageID:          p.ID,
				Severity:        SeverityWarn,
				Detail:          fmt.Sprintf("page %q has no inbound or outbound links", p.ID),
				SuggestedAction: "Add [[links]] to or from this page",
			})
		}
	}
	return findings
}

// BrokenLinkRule detects [[links]] pointing to non-existent pages.
type BrokenLinkRule struct{}

func (r *BrokenLinkRule) Name() string { return "broken_link" }
func (r *BrokenLinkRule) Run(ctx context.Context, vaultRoot string, db *index.DB) []Finding {
	var findings []Finding
	pages, err := index.ListPages(ctx, db, "")
	if err != nil {
		return nil
	}
	pageIDs := make(map[string]bool)
	for _, p := range pages {
		pageIDs[p.ID] = true
	}

	for _, p := range pages {
		outbound, _ := index.OutboundLinks(ctx, db, p.ID)
		for _, link := range outbound {
			if !pageIDs[link.TargetID] {
				findings = append(findings, Finding{
					Rule:            "broken_link",
					PageID:          p.ID,
					Severity:        SeverityError,
					Detail:          fmt.Sprintf("[[%s]] points to non-existent page", link.TargetID),
					SuggestedAction: "Create the target page or fix the link",
				})
			}
		}
	}
	return findings
}

// SchemaViolationRule checks frontmatter for required fields.
type SchemaViolationRule struct{}

func (r *SchemaViolationRule) Name() string { return "schema_violation" }
func (r *SchemaViolationRule) Run(ctx context.Context, vaultRoot string, db *index.DB) []Finding {
	var findings []Finding
	pages, err := index.ListPages(ctx, db, "")
	if err != nil {
		return nil
	}
	for _, p := range pages {
		// Skip system files that don't need type/title frontmatter.
		if isSystemPage(p.ID) {
			continue
		}
		if p.Type == "unknown" {
			findings = append(findings, Finding{
				Rule:     "schema_violation",
				PageID:   p.ID,
				Severity: SeverityError,
				Detail:   "page missing 'type' in frontmatter",
			})
		}
		if p.Title == "" || p.Title == p.ID {
			findings = append(findings, Finding{
				Rule:     "schema_violation",
				PageID:   p.ID,
				Severity: SeverityWarn,
				Detail:   "page missing explicit 'title' in frontmatter",
			})
		}
	}
	return findings
}

// UnverifiedClaimRule detects claims with status='unverified' for too long.
type UnverifiedClaimRule struct{}

func (r *UnverifiedClaimRule) Name() string { return "unverified_claim" }
func (r *UnverifiedClaimRule) Run(ctx context.Context, vaultRoot string, db *index.DB) []Finding {
	var findings []Finding
	pages, err := index.ListPages(ctx, db, "claim")
	if err != nil {
		return nil
	}
	for _, p := range pages {
		if p.Status == "unverified" {
			findings = append(findings, Finding{
				Rule:            "unverified_claim",
				PageID:          p.ID,
				Severity:        SeverityWarn,
				Detail:          fmt.Sprintf("claim %q has status 'unverified'", p.ID),
				SuggestedAction: "Verify or reject this claim",
			})
		}
	}
	return findings
}

// MissingIndexEntryRule checks if pages are listed in wiki/index.md.
type MissingIndexEntryRule struct{}

func (r *MissingIndexEntryRule) Name() string { return "missing_index_entry" }
func (r *MissingIndexEntryRule) Run(ctx context.Context, vaultRoot string, db *index.DB) []Finding {
	var findings []Finding
	indexPath := filepath.Join(vaultRoot, "wiki", "index.md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return nil // no index file → skip
	}
	indexContent := string(content)

	pages, err := index.ListPages(ctx, db, "")
	if err != nil {
		return nil
	}
	for _, p := range pages {
		if p.Type == "unknown" || isSystemPage(p.ID) {
			continue
		}
		if !strings.Contains(indexContent, p.ID) {
			findings = append(findings, Finding{
				Rule:            "missing_index_entry",
				PageID:          p.ID,
				Severity:        SeverityInfo,
				Detail:          fmt.Sprintf("page %q not found in wiki/index.md", p.ID),
				SuggestedAction: "Run 'wikimind reindex' to rebuild the index",
			})
		}
	}
	return findings
}

// isSystemPage returns true for vault system files that don't need frontmatter.
func isSystemPage(id string) bool {
	switch id {
	case "index", "log":
		return true
	}
	return false
}
