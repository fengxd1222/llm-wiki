package commit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextSeqAppendChangeLogAndReadEntry(t *testing.T) {
	root := t.TempDir()

	seq, err := NextSeq(root)
	if err != nil {
		t.Fatalf("NextSeq empty: %v", err)
	}
	if seq != 1 {
		t.Fatalf("NextSeq empty = %d, want 1", seq)
	}

	first := LogEntry{
		Seq:       1,
		Timestamp: "2026-05-24T00:00:00Z",
		Actor:     "user",
		Op:        "ingest",
		Summary:   "raw/inbox/a.md",
	}
	second := first
	second.Seq = 3
	second.Summary = "raw/inbox/c.md"

	if err := AppendChangeLog(root, first); err != nil {
		t.Fatalf("AppendChangeLog first: %v", err)
	}
	if err := AppendChangeLog(root, second); err != nil {
		t.Fatalf("AppendChangeLog second: %v", err)
	}

	seq, err = NextSeq(root)
	if err != nil {
		t.Fatalf("NextSeq populated: %v", err)
	}
	if seq != 4 {
		t.Fatalf("NextSeq populated = %d, want 4", seq)
	}

	got, err := ReadEntryBySeq(root, 3)
	if err != nil {
		t.Fatalf("ReadEntryBySeq hit: %v", err)
	}
	if got.Summary != second.Summary {
		t.Fatalf("Summary = %q, want %q", got.Summary, second.Summary)
	}

	_, err = ReadEntryBySeq(root, 2)
	if !errors.Is(err, ErrSeqNotFound) {
		t.Fatalf("ReadEntryBySeq miss err = %v, want ErrSeqNotFound", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(changeLogRelPath)))
	if err != nil {
		t.Fatalf("read change log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("change-log line count = %d, want 2", len(lines))
	}
	for _, line := range lines {
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("invalid jsonl line %q: %v", line, err)
		}
	}
}

func TestAppendLogMdCreatesHeaderAndSanitizesSummary(t *testing.T) {
	root := t.TempDir()
	entry := LogEntry{
		Seq:       1,
		Timestamp: "2026-05-24T00:00:00Z",
		Actor:     "user",
		Op:        "ingest",
		Summary:   "raw/inbox/a|b.md\nnext",
	}

	if err := AppendLogMd(root, entry); err != nil {
		t.Fatalf("AppendLogMd: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(logMdRelPath)))
	if err != nil {
		t.Fatalf("read log.md: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "| seq | ts | actor | op | summary |") {
		t.Fatalf("log.md missing header:\n%s", got)
	}
	if !strings.Contains(got, `raw/inbox/a\|b.md next`) {
		t.Fatalf("log.md did not sanitize summary:\n%s", got)
	}
}
