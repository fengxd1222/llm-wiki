package commit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitWritesSourceAndLogsInSingleCommit(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	rawRel := "raw/inbox/sample.md"
	if err := os.MkdirAll(filepath.Join(root, "raw", "inbox"), 0o755); err != nil {
		t.Fatalf("mkdir raw/inbox: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rawRel)), []byte("# Sample\n"), 0o644); err != nil {
		t.Fatalf("seed raw file: %v", err)
	}

	entry, err := Commit(context.Background(), root, "ingest", rawRel, []string{rawRel})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if entry.Seq != 1 {
		t.Fatalf("Seq = %d, want 1", entry.Seq)
	}
	if strings.TrimSpace(entry.GitSHA) == "" {
		t.Fatalf("GitSHA is empty")
	}

	subject, err := runGit(context.Background(), root, "log", "-1", "--format=%s")
	if err != nil {
		t.Fatalf("git log subject: %v", err)
	}
	wantSubject := "ingest: raw/inbox/sample.md (seq=1)"
	if strings.TrimSpace(subject) != wantSubject {
		t.Fatalf("commit subject = %q, want %q", strings.TrimSpace(subject), wantSubject)
	}

	logEntry, err := ReadEntryBySeq(root, 1)
	if err != nil {
		t.Fatalf("ReadEntryBySeq: %v", err)
	}
	if logEntry.GitSHA != "" {
		t.Fatalf("persisted GitSHA = %q, want empty ADR-lite marker", logEntry.GitSHA)
	}

	status, err := GitStatus(context.Background(), root)
	if err != nil {
		t.Fatalf("GitStatus: %v", err)
	}
	if len(status) != 0 {
		t.Fatalf("GitStatus = %v, want clean", status)
	}
}

func TestCommitCanFindCommitBySeq(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	rawRel := "raw/inbox/sample.md"
	if err := os.MkdirAll(filepath.Join(root, "raw", "inbox"), 0o755); err != nil {
		t.Fatalf("mkdir raw/inbox: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rawRel)), []byte("# Sample\n"), 0o644); err != nil {
		t.Fatalf("seed raw file: %v", err)
	}
	entry, err := Commit(context.Background(), root, "ingest", rawRel, []string{rawRel})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	found, err := FindCommitBySeq(context.Background(), root, 1)
	if err != nil {
		t.Fatalf("FindCommitBySeq: %v", err)
	}
	if found != entry.GitSHA {
		t.Fatalf("FindCommitBySeq = %q, want %q", found, entry.GitSHA)
	}
}
