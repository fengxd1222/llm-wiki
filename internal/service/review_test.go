package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/commit"
	"github.com/fengxd1222/llm-wiki/internal/index"
)

// setupReviewVault creates a committed vault with a pending review and patch file.
func setupReviewVault(t *testing.T) (string, *index.DB) {
	t.Helper()
	root := t.TempDir()

	// Init vault structure.
	wikiDir := filepath.Join(root, "wiki", "claims")
	reviewDir := filepath.Join(root, "wiki", "_review")
	wikimindDir := filepath.Join(root, ".wikimind")
	for _, d := range []string{wikiDir, reviewDir, wikimindDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	// Write config.
	cfg := []byte("[vault]\nvault_root = \"" + root + "\"\nschema_version = \"1.0\"\n")
	if err := os.WriteFile(filepath.Join(wikimindDir, "config.toml"), cfg, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Init git + initial commit.
	ctx := context.Background()
	if err := commit.EnsureRepo(ctx, root); err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	if err := commit.GitAdd(ctx, root, "."); err != nil {
		t.Fatalf("GitAdd: %v", err)
	}
	if _, err := commit.GitCommit(ctx, root, "init vault"); err != nil {
		t.Fatalf("GitCommit: %v", err)
	}

	// Open DB.
	db, err := index.Open(root)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return root, db
}

func createPendingReview(t *testing.T, root string, db *index.DB, reviewID, targetPage, patchContent string) {
	t.Helper()
	ctx := context.Background()

	// Write patch file.
	patchDir := filepath.Join(root, "wiki", "_review")
	if err := os.MkdirAll(patchDir, 0o755); err != nil {
		t.Fatalf("mkdir _review: %v", err)
	}
	patchPath := filepath.Join(patchDir, reviewID+".patch")
	if err := os.WriteFile(patchPath, []byte(patchContent), 0o644); err != nil {
		t.Fatalf("write patch: %v", err)
	}

	// Insert review row.
	row := &index.ReviewRow{
		ID:           reviewID,
		Seq:          1,
		Agent:        "test-agent",
		SessionID:    "sess-1",
		Op:           "propose_page",
		TargetPageID: targetPage,
		PatchPath:    "wiki/_review/" + reviewID + ".patch",
		Status:       "pending",
		CreatedAt:    "2026-05-24T10:00:00Z",
	}
	if err := index.InsertReview(ctx, db, row); err != nil {
		t.Fatalf("InsertReview: %v", err)
	}
}

func TestAcceptReviewHappy(t *testing.T) {
	root, db := setupReviewVault(t)
	ctx := context.Background()

	// Create a target file that the patch will create.
	// The patch adds wiki/claims/test-claim.md.
	patch := `diff --git a/wiki/claims/test-claim.md b/wiki/claims/test-claim.md
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/wiki/claims/test-claim.md
@@ -0,0 +1,7 @@
+---
+id: test-claim
+type: claim
+title: "Test Claim"
+---
+
+# Test Claim
`
	createPendingReview(t, root, db, "r-0001", "test-claim", patch)

	result, err := AcceptReview(ctx, root, db, AcceptOptions{
		ReviewID:   "r-0001",
		AcceptedBy: "user",
	})
	if err != nil {
		t.Fatalf("AcceptReview: %v", err)
	}
	if result.ReviewID != "r-0001" {
		t.Fatalf("result.ReviewID = %s, want r-0001", result.ReviewID)
	}
	if result.Seq < 1 {
		t.Fatalf("result.Seq = %d, want >= 1", result.Seq)
	}
	if result.GitSHA == "" {
		t.Fatalf("result.GitSHA empty")
	}

	// Verify file exists on disk.
	claimPath := filepath.Join(root, "wiki", "claims", "test-claim.md")
	if _, err := os.Stat(claimPath); err != nil {
		t.Fatalf("claim file missing after accept: %v", err)
	}

	// Verify review status updated.
	review, err := index.GetReviewByID(ctx, db, "r-0001")
	if err != nil {
		t.Fatalf("GetReviewByID: %v", err)
	}
	if review.Status != "accepted" {
		t.Fatalf("review.Status = %s, want accepted", review.Status)
	}

	// Verify patch file deleted.
	patchPath := filepath.Join(root, "wiki", "_review", "r-0001.patch")
	if _, err := os.Stat(patchPath); !os.IsNotExist(err) {
		t.Fatalf("patch file should be deleted after accept")
	}
}

func TestAcceptReviewNotFound(t *testing.T) {
	root, db := setupReviewVault(t)
	ctx := context.Background()

	_, err := AcceptReview(ctx, root, db, AcceptOptions{ReviewID: "r-9999"})
	if err == nil {
		t.Fatalf("expected error for non-existent review")
	}
}

func TestAcceptReviewNotPending(t *testing.T) {
	root, db := setupReviewVault(t)
	ctx := context.Background()

	// Create and immediately reject a review.
	createPendingReview(t, root, db, "r-0002", "some-page", "dummy patch")
	_ = index.UpdateReviewStatus(ctx, db, "r-0002", "rejected", "user")

	_, err := AcceptReview(ctx, root, db, AcceptOptions{ReviewID: "r-0002"})
	if err == nil {
		t.Fatalf("expected ErrReviewNotPending")
	}
	if !contains(err.Error(), "not in pending") {
		t.Fatalf("error = %v, want 'not in pending'", err)
	}
}

func TestAcceptReviewPatchMissing(t *testing.T) {
	root, db := setupReviewVault(t)
	ctx := context.Background()

	// Insert review row but no patch file.
	row := &index.ReviewRow{
		ID:           "r-0003",
		Seq:          3,
		Agent:        "test",
		SessionID:    "s1",
		Op:           "propose_page",
		TargetPageID: "missing",
		PatchPath:    "wiki/_review/r-0003.patch",
		Status:       "pending",
		CreatedAt:    "2026-05-24T10:00:00Z",
	}
	if err := index.InsertReview(ctx, db, row); err != nil {
		t.Fatalf("InsertReview: %v", err)
	}

	_, err := AcceptReview(ctx, root, db, AcceptOptions{ReviewID: "r-0003"})
	if err == nil {
		t.Fatalf("expected ErrPatchMissing")
	}
	if !contains(err.Error(), "patch file missing") {
		t.Fatalf("error = %v, want 'patch file missing'", err)
	}
}

func TestAcceptReviewPatchApplyFail(t *testing.T) {
	root, db := setupReviewVault(t)
	ctx := context.Background()

	// Create a patch that won't apply (references non-existent file to modify).
	badPatch := `diff --git a/wiki/claims/nonexistent.md b/wiki/claims/nonexistent.md
index 1234567..abcdefg 100644
--- a/wiki/claims/nonexistent.md
+++ b/wiki/claims/nonexistent.md
@@ -1,3 +1,3 @@
-old line
+new line
`
	createPendingReview(t, root, db, "r-0004", "nonexistent", badPatch)

	_, err := AcceptReview(ctx, root, db, AcceptOptions{ReviewID: "r-0004"})
	if err == nil {
		t.Fatalf("expected patch apply failure")
	}

	// Verify review still pending (rollback).
	review, _ := index.GetReviewByID(ctx, db, "r-0004")
	if review.Status != "pending" {
		t.Fatalf("review.Status = %s after failed accept, want pending", review.Status)
	}

	// Verify patch file still exists.
	patchPath := filepath.Join(root, "wiki", "_review", "r-0004.patch")
	if _, err := os.Stat(patchPath); err != nil {
		t.Fatalf("patch file should still exist after failed accept")
	}
}

func TestRejectReviewHappy(t *testing.T) {
	_, db := setupReviewVault(t)
	ctx := context.Background()

	row := &index.ReviewRow{
		ID:        "r-0005",
		Seq:       5,
		Agent:     "test",
		SessionID: "s1",
		Op:        "propose_page",
		Status:    "pending",
		CreatedAt: "2026-05-24T10:00:00Z",
	}
	if err := index.InsertReview(ctx, db, row); err != nil {
		t.Fatalf("InsertReview: %v", err)
	}

	err := RejectReview(ctx, db, "r-0005", "user", "This claim is not supported by evidence")
	if err != nil {
		t.Fatalf("RejectReview: %v", err)
	}

	review, _ := index.GetReviewByID(ctx, db, "r-0005")
	if review.Status != "rejected" {
		t.Fatalf("review.Status = %s, want rejected", review.Status)
	}
}

func TestRejectReviewShortReason(t *testing.T) {
	_, db := setupReviewVault(t)
	ctx := context.Background()

	row := &index.ReviewRow{
		ID:        "r-0006",
		Seq:       6,
		Agent:     "test",
		SessionID: "s1",
		Op:        "propose_page",
		Status:    "pending",
		CreatedAt: "2026-05-24T10:00:00Z",
	}
	if err := index.InsertReview(ctx, db, row); err != nil {
		t.Fatalf("InsertReview: %v", err)
	}

	err := RejectReview(ctx, db, "r-0006", "user", "short")
	if err == nil {
		t.Fatalf("expected error for short reason")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
