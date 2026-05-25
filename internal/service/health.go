package service

import (
	"context"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

// HealthScore computes the vault health score.
//
// Formula:
//
//	score = 100
//	  - 5 * drift_claims_count (max -50)  [staged: always 0 until W3]
//	  - 1 * lint_warnings_count (max -30) [staged: always 0 until W3]
//	  - 2 * orphan_pages_count (max -20)
//	floor 0
type HealthScore struct {
	Score        int `json:"score"`
	DriftClaims  int `json:"drift_claims"`
	LintWarnings int `json:"lint_warnings"`
	OrphanPages  int `json:"orphan_pages"`
}

// ComputeHealth calculates the real health score based on page_links data.
func ComputeHealth(ctx context.Context, db *index.DB) (*HealthScore, error) {
	h := &HealthScore{
		Score:        100,
		DriftClaims:  0, // will be filled from claim_sources table
		LintWarnings: 0, // staged: W3 lint rules
	}

	// Count drift claims from claim_sources table.
	driftCount, driftErr := index.CountDriftClaims(ctx, db)
	if driftErr == nil {
		h.DriftClaims = driftCount
	}

	// Count orphan pages (claim, entity, concept with no inbound links)
	orphans, err := index.CountOrphanPages(ctx, db, []string{"claim", "entity", "concept"})
	if err != nil {
		return nil, err
	}
	h.OrphanPages = orphans

	// Apply penalties
	driftPenalty := h.DriftClaims * 5
	if driftPenalty > 50 {
		driftPenalty = 50
	}
	lintPenalty := h.LintWarnings * 1
	if lintPenalty > 30 {
		lintPenalty = 30
	}
	orphanPenalty := h.OrphanPages * 2
	if orphanPenalty > 20 {
		orphanPenalty = 20
	}

	h.Score = 100 - driftPenalty - lintPenalty - orphanPenalty
	if h.Score < 0 {
		h.Score = 0
	}
	return h, nil
}
