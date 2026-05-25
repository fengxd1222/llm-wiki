package lock

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Errors returned by LockManager operations.
var (
	ErrLockHeld    = errors.New("lock held by another session")
	ErrLockNotHeld = errors.New("page is not locked")
	ErrLockNotMine = errors.New("lock is held by a different session")
)

// DefaultTTL is the default lock TTL.
const DefaultTTL = 300 * time.Second

// MaxTTL is the maximum allowed TTL.
const MaxTTL = 3600 * time.Second

// GracePeriod is the time after TTL expiry before a lock can be reaped.
const GracePeriod = 60 * time.Second

// Lock represents an advisory lock on a page.
type Lock struct {
	PageID     string
	Holder     string // session token
	Agent      string
	AcquiredAt time.Time
	TTL        time.Duration
	LastSeenAt time.Time
}

// IsExpired returns true if the lock has exceeded TTL + grace period.
func (l *Lock) IsExpired(now time.Time) bool {
	return now.After(l.LastSeenAt.Add(l.TTL + GracePeriod))
}

// IsStale returns true if the lock has exceeded TTL (but may still be in grace).
func (l *Lock) IsStale(now time.Time) bool {
	return now.After(l.LastSeenAt.Add(l.TTL))
}

// ReapedLock records a lock that was cleaned up.
type ReapedLock struct {
	Lock
	ReapedAt time.Time
}

// Manager manages advisory page locks in memory.
type Manager struct {
	mu    sync.RWMutex
	locks map[string]*Lock // pageID -> Lock
}

// NewManager creates a new lock manager.
func NewManager() *Manager {
	return &Manager{
		locks: make(map[string]*Lock),
	}
}

// Acquire attempts to acquire a lock on a page.
// Returns ErrLockHeld if another session holds a non-expired lock.
func (m *Manager) Acquire(pageID, holder, agent string, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if ttl > MaxTTL {
		ttl = MaxTTL
	}

	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.locks[pageID]; ok {
		if existing.Holder == holder {
			// Same holder — refresh.
			existing.TTL = ttl
			existing.LastSeenAt = now
			return nil
		}
		if !existing.IsExpired(now) {
			return fmt.Errorf("%w: held by agent=%s since %s",
				ErrLockHeld, existing.Agent, existing.AcquiredAt.Format(time.RFC3339))
		}
		// Expired — allow takeover.
	}

	m.locks[pageID] = &Lock{
		PageID:     pageID,
		Holder:     holder,
		Agent:      agent,
		AcquiredAt: now,
		TTL:        ttl,
		LastSeenAt: now,
	}
	return nil
}

// Release releases a lock held by the given session.
func (m *Manager) Release(pageID, holder string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.locks[pageID]
	if !ok {
		return ErrLockNotHeld
	}
	if existing.Holder != holder {
		return ErrLockNotMine
	}
	delete(m.locks, pageID)
	return nil
}

// Touch refreshes the LastSeenAt timestamp for a lock.
func (m *Manager) Touch(pageID, holder string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.locks[pageID]
	if !ok {
		return ErrLockNotHeld
	}
	if existing.Holder != holder {
		return ErrLockNotMine
	}
	existing.LastSeenAt = time.Now()
	return nil
}

// ForceRelease removes a lock regardless of holder (admin operation).
func (m *Manager) ForceRelease(pageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.locks[pageID]; !ok {
		return ErrLockNotHeld
	}
	delete(m.locks, pageID)
	return nil
}

// IsHeldByOther checks if a page is locked by a different session.
// Returns the lock info if held by another, nil otherwise.
func (m *Manager) IsHeldByOther(pageID, holder string) *Lock {
	m.mu.RLock()
	defer m.mu.RUnlock()

	existing, ok := m.locks[pageID]
	if !ok {
		return nil
	}
	if existing.Holder == holder {
		return nil
	}
	if existing.IsExpired(time.Now()) {
		return nil
	}
	return existing
}

// Reap removes all expired locks and returns them.
func (m *Manager) Reap(now time.Time) []ReapedLock {
	m.mu.Lock()
	defer m.mu.Unlock()

	var reaped []ReapedLock
	for pageID, l := range m.locks {
		if l.IsExpired(now) {
			reaped = append(reaped, ReapedLock{Lock: *l, ReapedAt: now})
			delete(m.locks, pageID)
		}
	}
	return reaped
}

// Get returns the current lock for a page, or nil if not locked.
func (m *Manager) Get(pageID string) *Lock {
	m.mu.RLock()
	defer m.mu.RUnlock()
	l, ok := m.locks[pageID]
	if !ok {
		return nil
	}
	cp := *l
	return &cp
}
