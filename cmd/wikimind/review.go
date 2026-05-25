package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/service"
	"github.com/spf13/cobra"
)

func newReviewCommand(stdout io.Writer, stdin io.Reader) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Manage review queue (list/show/diff/accept/reject)",
	}
	cmd.AddCommand(newReviewListCommand(stdout))
	cmd.AddCommand(newReviewShowCommand(stdout))
	cmd.AddCommand(newReviewDiffCommand(stdout))
	cmd.AddCommand(newReviewAcceptCommand(stdout, stdin))
	cmd.AddCommand(newReviewRejectCommand(stdout))
	return cmd
}

func newReviewListCommand(stdout io.Writer) *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List reviews by status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultRoot, err := resolveVaultFromCWD()
			if err != nil {
				return err
			}
			db, err := index.Open(vaultRoot)
			if err != nil {
				return fmt.Errorf("open index: %w", err)
			}
			defer db.Close()

			ctx := context.Background()
			var reviews []*index.ReviewRow
			if status == "all" {
				// List all statuses.
				for _, s := range []string{"pending", "accepted", "rejected"} {
					rows, err := index.ListReviewsByStatus(ctx, db, s)
					if err != nil {
						return err
					}
					reviews = append(reviews, rows...)
				}
			} else {
				reviews, err = index.ListReviewsByStatus(ctx, db, status)
				if err != nil {
					return err
				}
			}

			if len(reviews) == 0 {
				fmt.Fprintf(stdout, "No reviews with status=%s\n", status)
				return nil
			}

			fmt.Fprintf(stdout, "%-8s %-10s %-20s %-20s %-12s %-10s\n",
				"ID", "Status", "Op", "Target", "Agent", "Bundle")
			for _, r := range reviews {
				bundle := r.BundleID
				if bundle == "" {
					bundle = "—"
				}
				fmt.Fprintf(stdout, "%-8s %-10s %-20s %-20s %-12s %-10s\n",
					r.ID, r.Status, r.Op, r.TargetPageID, r.Agent, bundle)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "pending", "Filter by status (pending|accepted|rejected|all)")
	return cmd
}

func newReviewShowCommand(stdout io.Writer) *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "show <review_id>",
		Short: "Show review details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reviewID := args[0]
			vaultRoot, err := resolveVaultFromCWD()
			if err != nil {
				return err
			}
			db, err := index.Open(vaultRoot)
			if err != nil {
				return fmt.Errorf("open index: %w", err)
			}
			defer db.Close()

			ctx := context.Background()
			review, err := index.GetReviewByID(ctx, db, reviewID)
			if err != nil {
				return err
			}

			fmt.Fprintf(stdout, "id: %s\n", review.ID)
			fmt.Fprintf(stdout, "status: %s\n", review.Status)
			fmt.Fprintf(stdout, "op: %s\n", review.Op)
			fmt.Fprintf(stdout, "target: %s\n", review.TargetPageID)
			fmt.Fprintf(stdout, "agent: %s\n", review.Agent)
			fmt.Fprintf(stdout, "session: %s\n", review.SessionID)
			fmt.Fprintf(stdout, "created: %s\n", review.CreatedAt)
			if review.BundleID != "" {
				fmt.Fprintf(stdout, "bundle: %s\n", review.BundleID)
			}
			if review.DecidedAt != "" {
				fmt.Fprintf(stdout, "decided_at: %s\n", review.DecidedAt)
				fmt.Fprintf(stdout, "decided_by: %s\n", review.DecidedBy)
			}

			// Show patch preview.
			patchPath := filepath.Join(vaultRoot, "wiki", "_review", reviewID+".patch")
			if data, readErr := os.ReadFile(patchPath); readErr == nil {
				fmt.Fprintf(stdout, "---\n")
				if full {
					fmt.Fprintf(stdout, "%s", string(data))
				} else {
					lines := strings.Split(string(data), "\n")
					limit := 20
					if len(lines) < limit {
						limit = len(lines)
					}
					for _, l := range lines[:limit] {
						fmt.Fprintf(stdout, "%s\n", l)
					}
					if len(lines) > 20 {
						fmt.Fprintf(stdout, "... (%d more lines, use --full)\n", len(lines)-20)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Show full patch content")
	return cmd
}

func newReviewDiffCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "diff <review_id>",
		Short: "Show patch diff for a review",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reviewID := args[0]
			vaultRoot, err := resolveVaultFromCWD()
			if err != nil {
				return err
			}

			patchPath := filepath.Join(vaultRoot, "wiki", "_review", reviewID+".patch")
			data, err := os.ReadFile(patchPath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("patch file not found: %s", reviewID+".patch")
				}
				return err
			}
			fmt.Fprint(stdout, string(data))
			return nil
		},
	}
}

func newReviewAcceptCommand(stdout io.Writer, stdin io.Reader) *cobra.Command {
	var noConfirm bool
	cmd := &cobra.Command{
		Use:   "accept <review_id>",
		Short: "Accept a review and apply patch to main",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reviewID := args[0]
			vaultRoot, err := resolveVaultFromCWD()
			if err != nil {
				return err
			}
			db, err := index.Open(vaultRoot)
			if err != nil {
				return fmt.Errorf("open index: %w", err)
			}
			defer db.Close()

			if !noConfirm {
				if !confirmYes(stdout, stdin, fmt.Sprintf("Apply %s to main?", reviewID)) {
					fmt.Fprintf(stdout, "cancelled\n")
					return nil
				}
			}

			ctx := context.Background()
			result, err := service.AcceptReview(ctx, vaultRoot, db, service.AcceptOptions{
				ReviewID:   reviewID,
				AcceptedBy: "user",
			})
			if err != nil {
				return fmt.Errorf("accept %s: %w", reviewID, err)
			}

			fmt.Fprintf(stdout, "accepted: %s → applied to main (seq=%d, sha=%s)\n",
				result.ReviewID, result.Seq, result.GitSHA)
			for _, f := range result.Files {
				fmt.Fprintf(stdout, "  %s\n", f)
			}

			// D14: append to wiki/index.md after accept (best-effort).
			for _, f := range result.Files {
				if strings.HasPrefix(f, "wiki/") && strings.HasSuffix(f, ".md") {
					absPath := filepath.Join(vaultRoot, filepath.FromSlash(f))
					page, parseErr := service.ParsePage(absPath)
					if parseErr != nil {
						continue
					}
					id := ""
					pageType := ""
					title := ""
					if page.Frontmatter != nil {
						if v, ok := page.Frontmatter["id"].(string); ok {
							id = v
						}
						if v, ok := page.Frontmatter["type"].(string); ok {
							pageType = v
						}
						if v, ok := page.Frontmatter["title"].(string); ok {
							title = v
						}
					}
					if id == "" {
						id = strings.TrimSuffix(filepath.Base(f), ".md")
					}
					_ = service.AppendIndexEntry(ctx, vaultRoot, service.PageInfo{
						ID:    id,
						Type:  pageType,
						Title: title,
					})
				}
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&noConfirm, "no-confirm", false, "Skip confirmation prompt")
	return cmd
}

func newReviewRejectCommand(stdout io.Writer) *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "reject <review_id>",
		Short: "Reject a review (patch preserved for audit)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reviewID := args[0]
			vaultRoot, err := resolveVaultFromCWD()
			if err != nil {
				return err
			}
			db, err := index.Open(vaultRoot)
			if err != nil {
				return fmt.Errorf("open index: %w", err)
			}
			defer db.Close()

			ctx := context.Background()
			if err := service.RejectReview(ctx, db, reviewID, "user", reason); err != nil {
				return fmt.Errorf("reject %s: %w", reviewID, err)
			}

			fmt.Fprintf(stdout, "rejected: %s (reason: %s)\n", reviewID, reason)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Rejection reason (required, min 10 chars)")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}
