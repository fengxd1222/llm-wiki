package lock

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireAndRelease(t *testing.T) {
	m := NewManager()

	if err := m.Acquire("page-1", "sess-a", "agent-a", DefaultTTL); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// Same holder can re-acquire (refresh).
	if err := m.Acquire("page-1", "sess-a", "agent-a", DefaultTTL); err != nil {
		t.Fatalf("re-Acquire same holder: %v", err)
	}

	// Different holder blocked.
	if err := m.Acquire("page-1", "sess-b", "agent-b", DefaultTTL); err == nil {
		t.Fatalf("expected ErrLockHeld for different holder")
	}

	// Release.
	if err := m.Release("page-1", "sess-a"); err != nil {
		t.Fatalf("Release: %v", err)
	}

	// Now sess-b can acquire.
	if err := m.Acquire("page-1", "sess-b", "agent-b", DefaultTTL); err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
}

func TestReleaseNotHeld(t *testing.T) {
	m := NewManager()
	if err := m.Release("page-x", "sess-a"); err != ErrLockNotHeld {
		t.Fatalf("err = %v, want ErrLockNotHeld", err)
	}
}

func TestReleaseNotMine(t *testing.T) {
	m := NewManager()
	_ = m.Acquire("page-1", "sess-a", "agent-a", DefaultTTL)
	if err := m.Release("page-1", "sess-b"); err != ErrLockNotMine {
		t.Fatalf("err = %v, want ErrLockNotMine", err)
	}
}

func TestTouch(t *testing.T) {
	m := NewManager()
	_ = m.Acquire("page-1", "sess-a", "agent-a", DefaultTTL)

	if err := m.Touch("page-1", "sess-a"); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	if err := m.Touch("page-1", "sess-b"); err != ErrLockNotMine {
		t.Fatalf("Touch other: %v, want ErrLockNotMine", err)
	}
	if err := m.Touch("page-x", "sess-a"); err != ErrLockNotHeld {
		t.Fatalf("Touch missing: %v, want ErrLockNotHeld", err)
	}
}

func TestReap(t *testing.T) {
	m := NewManager()
	_ = m.Acquire("page-1", "sess-a", "agent-a", 1*time.Millisecond)

	// Not yet expired (within grace).
	reaped := m.Reap(time.Now())
	if len(reaped) != 0 {
		t.Fatalf("reaped too early: %d", len(reaped))
	}

	// After TTL + grace.
	future := time.Now().Add(2*time.Millisecond + GracePeriod + time.Second)
	reaped = m.Reap(future)
	if len(reaped) != 1 {
		t.Fatalf("reaped = %d, want 1", len(reaped))
	}
	if reaped[0].PageID != "page-1" {
		t.Fatalf("reaped page = %s, want page-1", reaped[0].PageID)
	}

	// Lock gone — can acquire.
	if err := m.Acquire("page-1", "sess-b", "agent-b", DefaultTTL); err != nil {
		t.Fatalf("Acquire after reap: %v", err)
	}
}

func TestForceRelease(t *testing.T) {
	m := NewManager()
	_ = m.Acquire("page-1", "sess-a", "agent-a", DefaultTTL)

	if err := m.ForceRelease("page-1"); err != nil {
		t.Fatalf("ForceRelease: %v", err)
	}
	if err := m.ForceRelease("page-1"); err != ErrLockNotHeld {
		t.Fatalf("ForceRelease again: %v, want ErrLockNotHeld", err)
	}
}

func TestIsHeldByOther(t *testing.T) {
	m := NewManager()
	_ = m.Acquire("page-1", "sess-a", "agent-a", DefaultTTL)

	// Same holder → nil.
	if l := m.IsHeldByOther("page-1", "sess-a"); l != nil {
		t.Fatalf("IsHeldByOther same holder: %+v", l)
	}
	// Different holder → lock info.
	if l := m.IsHeldByOther("page-1", "sess-b"); l == nil {
		t.Fatalf("IsHeldByOther different holder: nil, want lock")
	}
	// Not locked → nil.
	if l := m.IsHeldByOther("page-x", "sess-a"); l != nil {
		t.Fatalf("IsHeldByOther unlocked: %+v", l)
	}
}

func TestExpiredLockTakeover(t *testing.T) {
	m := NewManager()
	// Acquire with very short TTL.
	_ = m.Acquire("page-1", "sess-a", "agent-a", 1*time.Millisecond)

	// Manually expire by setting LastSeenAt in the past.
	m.mu.Lock()
	m.locks["page-1"].LastSeenAt = time.Now().Add(-(GracePeriod + time.Second))
	m.mu.Unlock()

	// Another session can now take over.
	if err := m.Acquire("page-1", "sess-b", "agent-b", DefaultTTL); err != nil {
		t.Fatalf("Acquire expired: %v", err)
	}
}

func TestConcurrentAcquire(t *testing.T) {
	m := NewManager()
	const n = 100
	var successes atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			holder := fmt.Sprintf("sess-%d", id)
			if err := m.Acquire("contested-page", holder, "agent", DefaultTTL); err == nil {
				successes.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Fatalf("concurrent Acquire successes = %d, want 1", got)
	}
}
