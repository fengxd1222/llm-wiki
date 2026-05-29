package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

// Queue limit defaults.
const (
	DefaultSoftLimit     = 30
	DefaultHardLimit     = 50
	DefaultCriticalLimit = 100
)

// Queue errors.
var (
	ErrQueueBacklog  = errors.New("queue at hard limit: review enters backlog")
	ErrQueueCritical = errors.New("queue at critical limit: propose rejected")
)

// QueueLimits holds the configured queue thresholds.
type QueueLimits struct {
	Soft     int
	Hard     int
	Critical int
}

// DefaultQueueLimits returns the default queue limits.
func DefaultQueueLimits() QueueLimits {
	return QueueLimits{
		Soft:     DefaultSoftLimit,
		Hard:     DefaultHardLimit,
		Critical: DefaultCriticalLimit,
	}
}

// QueueState represents the current state of the review queue.
type QueueState struct {
	Active     int  `json:"active"`
	Backlog    int  `json:"backlog"`
	CanPropose bool `json:"can_propose"`
	AtSoft     bool `json:"at_soft_limit"`
	AtHard     bool `json:"at_hard_limit"`
	AtCritical bool `json:"at_critical_limit"`
}

// GetQueueState computes the current queue state against configured limits.
func GetQueueState(ctx context.Context, db *index.DB, limits QueueLimits) (*QueueState, error) {
	active, err := index.CountReviewsByStatus(ctx, db, "pending")
	if err != nil {
		return nil, err
	}
	// Fail closed: a backlog-count read failure must not silently undercount
	// the queue total. Undercounting would let a propose slip through the
	// quota gate when the queue is actually full (same class as F-012).
	backlog, err := index.CountReviewsByStatus(ctx, db, "backlog")
	if err != nil {
		return nil, fmt.Errorf("count backlog reviews: %w", err)
	}

	total := active + backlog
	state := &QueueState{
		Active:     active,
		Backlog:    backlog,
		CanPropose: total < limits.Critical,
		AtSoft:     total >= limits.Soft,
		AtHard:     total >= limits.Hard,
		AtCritical: total >= limits.Critical,
	}
	return state, nil
}

// CheckQueueForPropose checks if a new propose is allowed.
// Returns nil if OK, ErrQueueBacklog if at hard limit, ErrQueueCritical if at critical.
func CheckQueueForPropose(ctx context.Context, db *index.DB, limits QueueLimits) error {
	state, err := GetQueueState(ctx, db, limits)
	if err != nil {
		// Fail closed: a queue-state read failure must NOT silently allow a
		// propose through. Surface the error so the caller rejects the write
		// rather than bypassing the quota gate.
		return fmt.Errorf("check queue: %w", err)
	}
	if state.AtCritical {
		return ErrQueueCritical
	}
	if state.AtHard {
		return ErrQueueBacklog
	}
	return nil
}
