package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/service"
	"github.com/fengxd1222/llm-wiki/internal/watcher"
	"github.com/spf13/cobra"
)

func newWatchCommand(stdout, stderr io.Writer) *cobra.Command {
	var autoIngest bool
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch raw/inbox/ for new files (Ctrl-C to stop)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultRoot, err := resolveVaultFromCWD()
			if err != nil {
				return err
			}

			inboxDir := filepath.Join(vaultRoot, "raw", "inbox")
			if _, err := os.Stat(inboxDir); err != nil {
				return fmt.Errorf("raw/inbox/ not found: %w", err)
			}

			w, err := watcher.New(200 * time.Millisecond)
			if err != nil {
				return fmt.Errorf("create watcher: %w", err)
			}
			defer w.Close()

			if err := w.Add(inboxDir); err != nil {
				return fmt.Errorf("watch raw/inbox/: %w", err)
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			w.Start(ctx)
			fmt.Fprintf(stderr, "watching %s (Ctrl-C to stop)\n", inboxDir)

			var db *index.DB
			if autoIngest {
				db, err = index.Open(vaultRoot)
				if err != nil {
					return fmt.Errorf("open index for auto-ingest: %w", err)
				}
				defer db.Close()
			}

			for {
				select {
				case <-ctx.Done():
					fmt.Fprintf(stderr, "\nstopped\n")
					return nil
				case ev, ok := <-w.Events():
					if !ok {
						return nil
					}
					fmt.Fprintf(stdout, "[%s] %s %s\n",
						ev.Ts.Format("15:04:05"), ev.Op, ev.Path)

					if autoIngest && ev.Op.Has(watcher.OpCreate) {
						if !service.IsSupportedFormat(ev.Path) {
							fmt.Fprintf(stderr, "  skip (unsupported format): %s\n", filepath.Base(ev.Path))
							continue
						}
						result, ingestErr := service.IngestFile(ctx, db, vaultRoot, ev.Path)
						if ingestErr != nil {
							fmt.Fprintf(stderr, "  ingest error: %v\n", ingestErr)
						} else if result.Duplicate {
							fmt.Fprintf(stderr, "  skip (duplicate): %s\n", result.Source.RawID)
						} else {
							fmt.Fprintf(stdout, "  ingested: %s (seq=%d)\n",
								result.Source.RawID, result.LogEntry.Seq)
						}
					}
				}
			}
		},
	}
	cmd.Flags().BoolVar(&autoIngest, "auto-ingest", false, "Automatically ingest new files")
	return cmd
}
