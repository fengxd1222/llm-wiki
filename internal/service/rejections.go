package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const rejectionsRelPath = ".wikimind/rejections.jsonl"

// Rejection records a user rejection for agent memory.
type Rejection struct {
	ReviewID string `json:"review_id"`
	Agent    string `json:"agent"`
	Page     string `json:"page"`
	Reason   string `json:"reason"`
	TS       string `json:"ts"`
}

// RecordRejection appends a rejection entry to .wikimind/rejections.jsonl.
func RecordRejection(ctx context.Context, vaultRoot string, r Rejection) error {
	_ = ctx
	if r.TS == "" {
		r.TS = time.Now().UTC().Format(time.RFC3339)
	}
	absPath := filepath.Join(vaultRoot, filepath.FromSlash(rejectionsRelPath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("mkdir for rejections: %w", err)
	}
	f, err := os.OpenFile(absPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open rejections.jsonl: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal rejection: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write rejection: %w", err)
	}
	return nil
}

// LoadRecentRejections reads the last N rejections (newest first).
func LoadRecentRejections(ctx context.Context, vaultRoot string, limit int) ([]Rejection, error) {
	_ = ctx
	absPath := filepath.Join(vaultRoot, filepath.FromSlash(rejectionsRelPath))
	f, err := os.Open(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open rejections.jsonl: %w", err)
	}
	defer f.Close()

	var all []Rejection
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var r Rejection
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue // skip malformed lines
		}
		all = append(all, r)
	}

	// Reverse for newest first.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}
	return all, nil
}
