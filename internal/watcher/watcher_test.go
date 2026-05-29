package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherCreateEvent(t *testing.T) {
	dir := t.TempDir()

	w, err := New(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	if err := w.Add(dir); err != nil {
		t.Fatalf("Add: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	// Create a file.
	testFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(testFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wait for debounced event.
	select {
	case ev := <-w.Events():
		if ev.Path != testFile {
			t.Fatalf("event.Path = %s, want %s", ev.Path, testFile)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for event")
	}
}

func TestWatcherDebounce(t *testing.T) {
	dir := t.TempDir()

	w, err := New(200 * time.Millisecond)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	if err := w.Add(dir); err != nil {
		t.Fatalf("Add: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	// Rapid writes to same file.
	testFile := filepath.Join(dir, "rapid.md")
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(testFile, []byte("write"), 0o644); err != nil {
			t.Fatalf("WriteFile %d: %v", i, err)
		}
		time.Sleep(30 * time.Millisecond)
	}

	// Should get exactly 1 debounced event.
	select {
	case ev := <-w.Events():
		if ev.Path != testFile {
			t.Fatalf("event.Path = %s, want %s", ev.Path, testFile)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for debounced event")
	}

	// No second event within a reasonable window.
	select {
	case ev := <-w.Events():
		t.Fatalf("unexpected second event: %+v", ev)
	case <-time.After(500 * time.Millisecond):
		// Good — no extra event.
	}
}

func TestWatcherClose(t *testing.T) {
	dir := t.TempDir()

	w, err := New(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := w.Add(dir); err != nil {
		t.Fatalf("Add: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	cancel()

	// Close should not hang.
	done := make(chan struct{})
	go func() {
		_ = w.Close()
		close(done)
	}()

	select {
	case <-done:
		// Good.
	case <-time.After(3 * time.Second):
		t.Fatalf("Close hung")
	}
}

// TestWatcherCloseRaceWithPendingTimer exercises the F-042 window: a debounce
// timer is armed and about to fire when Close() runs. The atomic guard must
// prevent any send on the closed events channel. Run with `go test -race`.
func TestWatcherCloseRaceWithPendingTimer(t *testing.T) {
	dir := t.TempDir()

	w, err := New(20 * time.Millisecond)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := w.Add(dir); err != nil {
		t.Fatalf("Add: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	// Arm several debounce timers by creating files, then immediately Close
	// so the AfterFunc callbacks race with channel close.
	for i := 0; i < 10; i++ {
		f := filepath.Join(dir, "race-"+string(rune('a'+i))+".md")
		_ = os.WriteFile(f, []byte("x"), 0o644)
	}

	// Close right inside the debounce window — must not panic on closed channel.
	time.Sleep(5 * time.Millisecond)
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Give any stray timers a chance to fire post-close; guard must swallow them.
	time.Sleep(50 * time.Millisecond)
}

// TestWatcherDebounceAfterCloseNoPanic directly drives the close→callback
// ordering to ensure the guard holds.
func TestWatcherDebounceAfterCloseNoPanic(t *testing.T) {
	dir := t.TempDir()
	w, err := New(10 * time.Millisecond)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := w.Add(dir); err != nil {
		t.Fatalf("Add: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// debounce() after close must be a no-op (closed guard), not a panic.
	w.debounce(filepath.Join(dir, "late.md"), OpCreate)
}
