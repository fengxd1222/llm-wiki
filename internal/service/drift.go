package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

// Drift status constants.
const (
	DriftStatusVerified      = "verified"
	DriftStatusDrift         = "drift"
	DriftStatusAnchorMissing = "anchor_missing"
	DriftStatusRawMissing    = "raw_missing"
	DriftStatusUnknown       = "unknown"
)

// VerifyClaimSource checks if a claim source's quote_hash still matches the raw file.
// Returns the drift status.
func VerifyClaimSource(ctx context.Context, vaultRoot string, row *index.ClaimSourceRow) (string, error) {
	rawAbs := filepath.Join(vaultRoot, filepath.FromSlash(row.RawID))

	// Check raw file exists.
	if _, err := os.Stat(rawAbs); err != nil {
		if os.IsNotExist(err) {
			return DriftStatusRawMissing, nil
		}
		return "", fmt.Errorf("stat raw %s: %w", row.RawID, err)
	}

	// Read raw content.
	content, err := os.ReadFile(rawAbs)
	if err != nil {
		return "", fmt.Errorf("read raw %s: %w", row.RawID, err)
	}

	// Resolve anchor and compute hash.
	text, _, resolveErr := index.ResolveAnchor(content, row.Anchor)
	if resolveErr != nil {
		return DriftStatusAnchorMissing, nil
	}

	currentHash := index.QuoteHash(text)
	if currentHash == row.StoredQuoteHash {
		return DriftStatusVerified, nil
	}
	return DriftStatusDrift, nil
}

// ScanAllClaims verifies all claim sources and updates their cached status.
// Returns the count of claims with drift.
func ScanAllClaims(ctx context.Context, db *index.DB, vaultRoot string) (int, error) {
	// Get all claim pages.
	pages, err := index.ListPages(ctx, db, "claim")
	if err != nil {
		return 0, fmt.Errorf("list claim pages: %w", err)
	}

	driftClaims := 0
	now := time.Now().Unix()

	for _, page := range pages {
		sources, err := index.ListClaimSources(ctx, db, page.ID)
		if err != nil {
			continue
		}
		hasDrift := false
		for _, src := range sources {
			status, verifyErr := VerifyClaimSource(ctx, vaultRoot, src)
			if verifyErr != nil {
				continue
			}
			_ = index.UpdateClaimSourceStatus(ctx, db, src.ClaimID, src.RawID, src.Anchor, status, now)
			if status == DriftStatusDrift || status == DriftStatusAnchorMissing || status == DriftStatusRawMissing {
				hasDrift = true
			}
		}
		if hasDrift {
			driftClaims++
		}
	}
	return driftClaims, nil
}
