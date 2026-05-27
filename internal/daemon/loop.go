package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/lock"
	"github.com/fengxd1222/llm-wiki/internal/watcher"
)

// Config holds daemon configuration.
type Config struct {
	VaultRoot string
	LogPath   string // defaults to .wikimind/daemon.log
}

// Daemon is the main WikiMind daemon process.
type Daemon struct {
	cfg     Config
	db      *index.DB
	lockMgr *lock.Manager
	watcher *watcher.Watcher
	logger  *log.Logger
	logFile *os.File
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// New creates a new daemon instance.
func New(cfg Config) (*Daemon, error) {
	if cfg.LogPath == "" {
		cfg.LogPath = filepath.Join(cfg.VaultRoot, ".wikimind", "daemon.log")
	}

	db, err := index.Open(cfg.VaultRoot)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}

	logFile, err := os.OpenFile(cfg.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("open log: %w", err)
	}

	w, err := watcher.New(200 * time.Millisecond)
	if err != nil {
		logFile.Close()
		db.Close()
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	return &Daemon{
		cfg:     cfg,
		db:      db,
		lockMgr: lock.NewManager(),
		watcher: w,
		logFile: logFile,
		logger:  log.New(logFile, "[wikimindd] ", log.LstdFlags),
	}, nil
}

// Run starts the daemon main loop. Blocks until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	ctx, d.cancel = context.WithCancel(ctx)
	d.logger.Printf("starting daemon vault=%s", d.cfg.VaultRoot)

	// Start watcher on raw/inbox/.
	inboxDir := filepath.Join(d.cfg.VaultRoot, "raw", "inbox")
	if _, err := os.Stat(inboxDir); err == nil {
		if err := d.watcher.Add(inboxDir); err != nil {
			d.logger.Printf("warn: watch raw/inbox: %v", err)
		}
	}
	d.watcher.Start(ctx)

	// Lock reaper goroutine (every 30s).
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				reaped := d.lockMgr.Reap(now)
				if len(reaped) > 0 {
					d.logger.Printf("reaped %d expired locks", len(reaped))
				}
			}
		}
	}()

	// Watcher event consumer (log events).
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-d.watcher.Events():
				if !ok {
					return
				}
				d.logger.Printf("file event: %s %s", ev.Op, ev.Path)
			}
		}
	}()

	// Block until shutdown.
	<-ctx.Done()
	d.logger.Printf("shutting down")
	return d.Shutdown()
}

// Shutdown gracefully stops the daemon.
func (d *Daemon) Shutdown() error {
	if d.cancel != nil {
		d.cancel()
	}
	_ = d.watcher.Close()
	d.wg.Wait()
	d.db.Close()
	d.logger.Printf("stopped")
	if d.logFile != nil {
		_ = d.logFile.Close()
	}
	return nil
}

// LockManager returns the daemon's lock manager.
func (d *Daemon) LockManager() *lock.Manager {
	return d.lockMgr
}

// DB returns the daemon's database handle.
func (d *Daemon) DB() *index.DB {
	return d.db
}
