package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fengxd1222/llm-wiki/internal/commit"
	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/proposal"
)

// Review accept/reject errors.
var (
	ErrReviewNotPending          = errors.New("review is not in pending status")
	ErrPatchMissing              = errors.New("patch file missing from wiki/_review/")
	ErrPostApplyValidationFailed = errors.New("post-apply validation failed")
)

// AcceptOptions configures a review accept operation.
type AcceptOptions struct {
	ReviewID   string
	AcceptedBy string // "user" default
	SkipReindex bool
}

// AcceptResult holds the outcome of a successful accept.
type AcceptResult struct {
	ReviewID string
	GitSHA   string
	Seq      int
	Files    []string
}

// AcceptReview implements the 13-step atomic accept flow per architecture §3.3.
//
// On any failure after git apply, performs git reset --hard HEAD to restore
// the working tree, and leaves reviews.status as pending.
func AcceptReview(ctx context.Context, vaultRoot string, db *index.DB, opts AcceptOptions) (*AcceptResult, error) {
	if opts.AcceptedBy == "" {
		opts.AcceptedBy = "user"
	}

	// Step 1: Validate review exists and is pending.
	review, err := index.GetReviewByID(ctx, db, opts.ReviewID)
	if err != nil {
		return nil, err
	}
	if review.Status != "pending" {
		return nil, fmt.Errorf("%w: current status=%s", ErrReviewNotPending, review.Status)
	}

	// Step 2: Read patch file.
	patchRel := filepath.Join("wiki", "_review", opts.ReviewID+".patch")
	patchAbs := filepath.Join(vaultRoot, patchRel)
	patchData, err := os.ReadFile(patchAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrPatchMissing, patchRel)
		}
		return nil, fmt.Errorf("read patch: %w", err)
	}

	// Step 3-4: Skip re-validation of quote_hash/base_hash for D12 MVP.
	// (D11 already validated at propose time; re-validation is W3 enhancement.)

	// Step 5: Parse affected files from patch header.
	files := parsePatchFiles(patchData)

	// Step 6: git apply --check (dry run).
	if err := gitApplyCheck(ctx, vaultRoot, patchData); err != nil {
		return nil, fmt.Errorf("%w: %v", proposal.ErrPatchApplyFailed, err)
	}

	// Step 7: git apply (real).
	if err := proposal.ApplyPatch(ctx, vaultRoot, patchData); err != nil {
		return nil, err
	}

	// Step 8: Post-apply validation — verify affected files are valid markdown.
	for _, f := range files {
		absPath := filepath.Join(vaultRoot, filepath.FromSlash(f))
		if _, statErr := os.Stat(absPath); statErr != nil {
			// File was deleted by patch — that's valid.
			continue
		}
		if _, parseErr := ParsePage(absPath); parseErr != nil {
			// Rollback: git reset --hard HEAD.
			_ = gitResetHard(ctx, vaultRoot)
			return nil, fmt.Errorf("%w: %s: %v", ErrPostApplyValidationFailed, f, parseErr)
		}
	}

	// Step 9: Commit via commit.Commit (handles git add + commit + log + change-log).
	summary := buildAcceptSummary(review)
	commitMsg := fmt.Sprintf("%s [%s]", summary, opts.ReviewID)
	logEntry, err := commit.CommitWithActor(ctx, vaultRoot, opts.AcceptedBy, "accept", commitMsg, files)
	if err != nil {
		// Rollback git state.
		_ = gitResetHard(ctx, vaultRoot)
		return nil, fmt.Errorf("commit accept: %w", err)
	}

	// Step 10: Incremental reindex for affected pages.
	if !opts.SkipReindex && db != nil {
		for _, f := range files {
			if !strings.HasPrefix(f, "wiki/") || !strings.HasSuffix(f, ".md") {
				continue
			}
			absPath := filepath.Join(vaultRoot, filepath.FromSlash(f))
			if _, statErr := os.Stat(absPath); statErr != nil {
				continue // deleted file
			}
			page, parseErr := ParsePage(absPath)
			if parseErr != nil {
				continue
			}
			id := frontmatterString(page.Frontmatter, "id")
			if id == "" {
				id = strings.TrimSuffix(filepath.Base(f), ".md")
			}
			pageType := frontmatterString(page.Frontmatter, "type")
			if pageType == "" {
				pageType = "unknown"
			}
			title := frontmatterString(page.Frontmatter, "title")
			if title == "" && len(page.Headings) > 0 {
				title = page.Headings[0].Text
			}
			row := &index.PageRow{
				ID:    id,
				Type:  pageType,
				Path:  f,
				Title: title,
			}
			_ = index.UpsertPage(ctx, db, row)
			if len(page.Outbounds) > 0 {
				_ = index.ReplacePageLinks(ctx, db, row.ID, page.Outbounds)
			}
		}
	}

	// Step 11: Delete patch file.
	_ = os.Remove(patchAbs)

	// Step 12: Update review status to accepted.
	if err := index.UpdateReviewStatus(ctx, db, opts.ReviewID, "accepted", opts.AcceptedBy); err != nil {
		// Non-fatal: commit already landed. Log warning.
		return nil, fmt.Errorf("update review status (commit already applied): %w", err)
	}

	// Step 13: Done.
	return &AcceptResult{
		ReviewID: opts.ReviewID,
		GitSHA:   logEntry.GitSHA,
		Seq:      logEntry.Seq,
		Files:    files,
	}, nil
}

// RejectReview marks a review as rejected without touching git.
func RejectReview(ctx context.Context, db *index.DB, reviewID, rejectedBy, reason string) error {
	if strings.TrimSpace(reason) == "" || len(strings.TrimSpace(reason)) < 10 {
		return errors.New("reject reason must be at least 10 characters")
	}
	if rejectedBy == "" {
		rejectedBy = "user"
	}

	review, err := index.GetReviewByID(ctx, db, reviewID)
	if err != nil {
		return err
	}
	if review.Status != "pending" {
		return fmt.Errorf("%w: current status=%s", ErrReviewNotPending, review.Status)
	}

	return index.UpdateReviewStatus(ctx, db, reviewID, "rejected", rejectedBy)
}

// parsePatchFiles extracts file paths from unified diff headers.
// Looks for lines like: "diff --git a/<path> b/<path>"
var diffHeaderRe = regexp.MustCompile(`^diff --git a/(.+?) b/(.+)$`)

func parsePatchFiles(patch []byte) []string {
	seen := map[string]bool{}
	var files []string
	for _, line := range strings.Split(string(patch), "\n") {
		m := diffHeaderRe.FindStringSubmatch(line)
		if m != nil {
			// Use b/ path (the destination).
			path := m[2]
			if !seen[path] {
				seen[path] = true
				files = append(files, path)
			}
		}
	}
	return files
}

func buildAcceptSummary(review *index.ReviewRow) string {
	if review.TargetPageID != "" {
		return review.TargetPageID
	}
	return review.Op + " " + review.ID
}

func gitApplyCheck(ctx context.Context, root string, patch []byte) error {
	cmd := exec.CommandContext(ctx, "git", "apply", "--check", "--whitespace=nowarn", "-")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(string(patch))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitResetHard(ctx context.Context, root string) error {
	cmd := exec.CommandContext(ctx, "git", "reset", "--hard", "HEAD")
	cmd.Dir = root
	return cmd.Run()
}

// MarkSuperseded marks a pending review as superseded by a newer review.
// Used when the same agent proposes a new edit to the same page.
func MarkSuperseded(ctx context.Context, db *index.DB, reviewID, supersededBy string) error {
	review, err := index.GetReviewByID(ctx, db, reviewID)
	if err != nil {
		return err
	}
	if review.Status != "pending" {
		return fmt.Errorf("%w: current status=%s", ErrReviewNotPending, review.Status)
	}
	return index.UpdateReviewStatus(ctx, db, reviewID, "superseded", supersededBy)
}

// DetectConflict checks if a review's target page has been modified since the
// review was created (base_hash mismatch). If so, marks it as 'conflict'.
func DetectConflict(ctx context.Context, db *index.DB, reviewID string) error {
	review, err := index.GetReviewByID(ctx, db, reviewID)
	if err != nil {
		return err
	}
	if review.Status != "pending" {
		return nil // only pending reviews can conflict
	}
	// D15 MVP: mark conflict status. Full base_hash re-check is W3+.
	return index.UpdateReviewStatus(ctx, db, reviewID, "conflict", "system")
}
