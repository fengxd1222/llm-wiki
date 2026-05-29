package service

import (
	"context"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

func TestGetQueueStateEmpty(t *testing.T) {
	root, db := setupReviewVault(t)
	_ = root
	ctx := context.Background()

	state, err := GetQueueState(ctx, db, DefaultQueueLimits())
	if err != nil {
		t.Fatalf("GetQueueState: %v", err)
	}
	if state.Active != 0 || state.Backlog != 0 {
		t.Fatalf("state = %+v, want empty", state)
	}
	if !state.CanPropose {
		t.Fatalf("CanPropose should be true when empty")
	}
}

func TestCheckQueueForProposeAtHard(t *testing.T) {
	_, db := setupReviewVault(t)
	ctx := context.Background()

	// Insert enough reviews to hit hard limit.
	limits := QueueLimits{Soft: 2, Hard: 3, Critical: 5}
	for i := 1; i <= 3; i++ {
		row := &index.ReviewRow{
			ID:        index.ReviewID(i),
			Seq:       i,
			Agent:     "test",
			SessionID: "s1",
			Op:        "propose_page",
			Status:    "pending",
			CreatedAt: "2026-05-25T10:00:00Z",
		}
		if err := index.InsertReview(ctx, db, row); err != nil {
			t.Fatalf("InsertReview %d: %v", i, err)
		}
	}

	err := CheckQueueForPropose(ctx, db, limits)
	if err != ErrQueueBacklog {
		t.Fatalf("err = %v, want ErrQueueBacklog", err)
	}
}

func TestCheckQueueForProposeAtCritical(t *testing.T) {
	_, db := setupReviewVault(t)
	ctx := context.Background()

	limits := QueueLimits{Soft: 1, Hard: 2, Critical: 3}
	for i := 1; i <= 3; i++ {
		row := &index.ReviewRow{
			ID:        index.ReviewID(i),
			Seq:       i,
			Agent:     "test",
			SessionID: "s1",
			Op:        "propose_page",
			Status:    "pending",
			CreatedAt: "2026-05-25T10:00:00Z",
		}
		if err := index.InsertReview(ctx, db, row); err != nil {
			t.Fatalf("InsertReview %d: %v", i, err)
		}
	}

	err := CheckQueueForPropose(ctx, db, limits)
	if err != ErrQueueCritical {
		t.Fatalf("err = %v, want ErrQueueCritical", err)
	}
}

// TestCheckQueueForProposeFailsClosed verifies that a DB read failure is
// surfaced as an error instead of silently allowing the propose (F-012).
// A failed queue-state lookup must NOT report "queue not full".
func TestCheckQueueForProposeFailsClosed(t *testing.T) {
	_, db := setupReviewVault(t)
	ctx := context.Background()

	// Close the underlying DB so the queue-state query fails.
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	err := CheckQueueForPropose(ctx, db, DefaultQueueLimits())
	if err == nil {
		t.Fatal("CheckQueueForPropose returned nil on DB error, want fail-closed error")
	}
	// Must not be one of the normal quota-gate sentinels; it should be the
	// wrapped read error.
	if err == ErrQueueBacklog || err == ErrQueueCritical {
		t.Fatalf("err = %v, want wrapped check-queue error", err)
	}
}
