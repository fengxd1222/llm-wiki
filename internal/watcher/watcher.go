package watcher

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileEvent represents a debounced file system event.
type FileEvent struct {
	Path string
	Op   fsnotify.Op
	Ts   time.Time
}

// OpCreate is fsnotify.Create, exported for consumers.
const OpCreate = fsnotify.Create

// Watcher monitors directories for file changes with debouncing.
type Watcher struct {
	fsw        *fsnotify.Watcher
	events     chan FileEvent
	debounceMs time.Duration
	done       chan struct{}
	wg         sync.WaitGroup
	closed     atomic.Bool

	mu     sync.Mutex
	timers map[string]*time.Timer
}

// New creates a new Watcher with the specified debounce duration.
func New(debounceMs time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if debounceMs <= 0 {
		debounceMs = 200 * time.Millisecond
	}
	return &Watcher{
		fsw:        fsw,
		events:     make(chan FileEvent, 64),
		debounceMs: debounceMs,
		done:       make(chan struct{}),
		timers:     make(map[string]*time.Timer),
	}, nil
}

// Add adds a directory to the watch list.
func (w *Watcher) Add(dir string) error {
	return w.fsw.Add(dir)
}

// Events returns the channel of debounced file events.
func (w *Watcher) Events() <-chan FileEvent {
	return w.events
}

// Start begins processing fsnotify events in a background goroutine.
// Call Close to stop.
func (w *Watcher) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.done:
				return
			case ev, ok := <-w.fsw.Events:
				if !ok {
					return
				}
				// Only care about Create and Write events.
				if ev.Op&(fsnotify.Create|fsnotify.Write) == 0 {
					continue
				}
				w.debounce(ev.Name, ev.Op)
			case _, ok := <-w.fsw.Errors:
				if !ok {
					return
				}
				// Swallow errors for now; W3 will add error reporting.
			}
		}
	}()
}

// Close stops the watcher and waits for the goroutine to exit.
func (w *Watcher) Close() error {
	close(w.done)
	err := w.fsw.Close()
	w.wg.Wait()
	// Mark closed and stop pending timers under the same lock the debounce
	// callback uses, so no AfterFunc callback can send on the channel after
	// we close it (F-042: send-on-closed-channel race).
	w.mu.Lock()
	w.closed.Store(true)
	for _, t := range w.timers {
		t.Stop()
	}
	w.timers = nil
	w.mu.Unlock()
	close(w.events)
	return err
}

func (w *Watcher) debounce(path string, op fsnotify.Op) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed.Load() {
		return
	}
	if t, exists := w.timers[path]; exists {
		t.Stop()
	}
	w.timers[path] = time.AfterFunc(w.debounceMs, func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		// Guard against Close() racing with this callback: once closed, the
		// events channel is (or is about to be) closed — never send on it.
		if w.closed.Load() {
			return
		}
		delete(w.timers, path)

		select {
		case w.events <- FileEvent{Path: path, Op: op, Ts: time.Now()}:
		default:
			// Channel full; drop event (W3 will add backpressure).
		}
	})
}
