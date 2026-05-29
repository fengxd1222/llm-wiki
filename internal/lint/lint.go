// Package lint implements wiki vault health checks.
package lint

import (
	"context"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

// Severity levels for lint findings.
const (
	SeverityError = "error"
	SeverityWarn  = "warn"
	SeverityInfo  = "info"
)

// Finding represents a single lint issue.
type Finding struct {
	Rule            string `json:"rule"`
	PageID          string `json:"page_id,omitempty"`
	Severity        string `json:"severity"`
	Detail          string `json:"detail"`
	SuggestedAction string `json:"suggested_action,omitempty"`
}

// Summary holds aggregate lint results.
type Summary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
	Total    int `json:"total"`
}

// Rule is the interface all lint rules implement.
type Rule interface {
	Name() string
	Run(ctx context.Context, vaultRoot string, db *index.DB) []Finding
}

// AllRules returns the full set of built-in lint rules.
func AllRules() []Rule {
	return []Rule{
		&OrphanRule{},
		&BrokenLinkRule{},
		&SchemaViolationRule{},
		&UnverifiedClaimRule{},
		&MissingIndexEntryRule{},
	}
}

// RuleCount returns the number of built-in lint rules.
//
// The CLI lint banner uses this as the single source of truth for the rule
// count instead of hardcoding a number that drifts out of sync.
func RuleCount() int {
	return len(AllRules())
}

// RunRules executes the given rules and returns findings + summary.
func RunRules(ctx context.Context, vaultRoot string, db *index.DB, rules []Rule) ([]Finding, Summary) {
	var findings []Finding
	for _, r := range rules {
		findings = append(findings, r.Run(ctx, vaultRoot, db)...)
	}

	var s Summary
	for _, f := range findings {
		switch f.Severity {
		case SeverityError:
			s.Errors++
		case SeverityWarn:
			s.Warnings++
		case SeverityInfo:
			s.Infos++
		}
	}
	s.Total = len(findings)
	return findings, s
}
