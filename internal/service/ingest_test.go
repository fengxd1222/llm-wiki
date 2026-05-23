package service

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/vault"
)

func TestIngestFileCopiesIntoInboxAndRecordsSource(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	srcPath := filepath.Join(t.TempDir(), "sample.md")
	body := []byte("# Sample\n\nHello\n")
	if err := os.WriteFile(srcPath, body, 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	res, err := IngestFile(ctx, db, vaultRoot, srcPath)
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	if res.Duplicate {
		t.Fatalf("first ingest unexpectedly marked duplicate")
	}
	if res.Source.RawID != "raw/inbox/sample.md" {
		t.Fatalf("RawID = %q, want raw/inbox/sample.md", res.Source.RawID)
	}
	if res.Source.Status != IngestStatusPending {
		t.Fatalf("Status = %q, want pending", res.Source.Status)
	}
	if res.Source.Size != int64(len(body)) {
		t.Fatalf("Size = %d, want %d", res.Source.Size, len(body))
	}
	if res.Source.SHA256 == "" {
		t.Fatalf("SHA256 empty")
	}

	// 文件已落 raw/inbox/。
	dest := filepath.Join(vaultRoot, "raw", "inbox", "sample.md")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("inbox copy missing: %v", err)
	}

	// 原文件未被破坏。
	if _, err := os.Stat(srcPath); err != nil {
		t.Fatalf("source file disturbed: %v", err)
	}

	// sources 表里能查到。
	found, err := index.FindSourceBySHA256(ctx, db, res.Source.SHA256)
	if err != nil {
		t.Fatalf("FindSourceBySHA256: %v", err)
	}
	if found == nil {
		t.Fatal("source row missing in sqlite")
	}
}

func TestIngestFileDeduplicatesSameContent(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	srcPath := filepath.Join(t.TempDir(), "dup.md")
	if err := os.WriteFile(srcPath, []byte("same content"), 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	first, err := IngestFile(ctx, db, vaultRoot, srcPath)
	if err != nil {
		t.Fatalf("first IngestFile: %v", err)
	}
	if first.Duplicate {
		t.Fatalf("first call should not be duplicate")
	}

	second, err := IngestFile(ctx, db, vaultRoot, srcPath)
	if err != nil {
		t.Fatalf("second IngestFile: %v", err)
	}
	if !second.Duplicate {
		t.Fatalf("second call should be duplicate")
	}
	if second.Source.RawID != first.Source.RawID {
		t.Fatalf("duplicate RawID = %q, want %q",
			second.Source.RawID, first.Source.RawID)
	}

	// raw/inbox/ 只应有一份文件。
	entries, err := os.ReadDir(filepath.Join(vaultRoot, "raw", "inbox"))
	if err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("inbox has %d entries, want 1: %v", len(entries), entries)
	}
}

func TestIngestFileLargeFileStreams(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	const size = 10 * 1024 * 1024 // 10 MiB
	srcPath := filepath.Join(t.TempDir(), "large.bin")
	if err := writeRandomFile(srcPath, size); err != nil {
		t.Fatalf("seed large file: %v", err)
	}

	// 基线内存读数，触发一次 GC 拿稳定数。
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	res, err := IngestFile(ctx, db, vaultRoot, srcPath)
	if err != nil {
		t.Fatalf("IngestFile large: %v", err)
	}
	if res.Source.Size != size {
		t.Fatalf("Size = %d, want %d", res.Source.Size, size)
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	// 流式 hash + copy 应保持工作集远低于文件大小（远低于 10 MiB）。
	// 给 4 MiB 余量覆盖测试框架自身波动，仍能验证不是"全文件读进内存"。
	delta := int64(after.Alloc) - int64(before.Alloc)
	if delta > 4*1024*1024 {
		t.Fatalf("memory delta = %d bytes, want streaming (<4MiB) for %d byte file",
			delta, size)
	}
}

func TestIngestFileMissing(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	_, err := IngestFile(ctx, db, vaultRoot, filepath.Join(t.TempDir(), "no-such.md"))
	if !errors.Is(err, ErrSourceMissing) {
		t.Fatalf("err = %v, want ErrSourceMissing", err)
	}
}

func TestIngestFileEmptyPath(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	_, err := IngestFile(ctx, db, vaultRoot, "")
	if !errors.Is(err, ErrSourceMissing) {
		t.Fatalf("err = %v, want ErrSourceMissing", err)
	}
}

func TestIngestFileDirectoryAsSource(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	dir := t.TempDir()
	_, err := IngestFile(ctx, db, vaultRoot, dir)
	if !errors.Is(err, ErrSourceUnreadable) {
		t.Fatalf("err = %v, want ErrSourceUnreadable", err)
	}
}

func TestIngestFileUnreadableSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses POSIX permissions")
	}
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	srcPath := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(srcPath, []byte("nope"), 0o600); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	if err := os.Chmod(srcPath, 0o000); err != nil {
		t.Fatalf("chmod 000: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(srcPath, 0o600) })

	_, err := IngestFile(ctx, db, vaultRoot, srcPath)
	if !errors.Is(err, ErrSourceUnreadable) {
		t.Fatalf("err = %v, want ErrSourceUnreadable", err)
	}
}

func TestIngestFileInvalidVaultRoot(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, newTestVault(t))

	srcPath := filepath.Join(t.TempDir(), "sample.md")
	if err := os.WriteFile(srcPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := IngestFile(ctx, db, "", srcPath)
	if !errors.Is(err, ErrInvalidVaultRoot) {
		t.Fatalf("err = %v, want ErrInvalidVaultRoot for empty vault root", err)
	}

	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed not-dir: %v", err)
	}
	_, err = IngestFile(ctx, db, notDir, srcPath)
	if !errors.Is(err, ErrInvalidVaultRoot) {
		t.Fatalf("err = %v, want ErrInvalidVaultRoot for file vault root", err)
	}
}

func TestIngestFileNilDB(t *testing.T) {
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	srcPath := filepath.Join(t.TempDir(), "sample.md")
	if err := os.WriteFile(srcPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := IngestFile(ctx, nil, vaultRoot, srcPath)
	if !errors.Is(err, index.ErrIndexUnavailable) {
		t.Fatalf("err = %v, want ErrIndexUnavailable", err)
	}
}

func TestIngestFileBasenameCollisionDifferentContent(t *testing.T) {
	// 同名不同内容 → 第二份应改名（追加 sha256 前 8 位），sources 表两行。
	ctx := context.Background()
	vaultRoot := newTestVault(t)
	db := openTestDB(t, vaultRoot)

	dirA := filepath.Join(t.TempDir(), "a")
	dirB := filepath.Join(t.TempDir(), "b")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatalf("mkdir a: %v", err)
	}
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatalf("mkdir b: %v", err)
	}
	srcA := filepath.Join(dirA, "note.md")
	srcB := filepath.Join(dirB, "note.md")
	if err := os.WriteFile(srcA, []byte("aaa"), 0o644); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	if err := os.WriteFile(srcB, []byte("bbb"), 0o644); err != nil {
		t.Fatalf("seed b: %v", err)
	}

	resA, err := IngestFile(ctx, db, vaultRoot, srcA)
	if err != nil {
		t.Fatalf("ingest a: %v", err)
	}
	resB, err := IngestFile(ctx, db, vaultRoot, srcB)
	if err != nil {
		t.Fatalf("ingest b: %v", err)
	}
	if resB.Duplicate {
		t.Fatalf("different content must not dedupe")
	}
	if resB.Source.RawID == resA.Source.RawID {
		t.Fatalf("collided RawIDs: %q == %q",
			resB.Source.RawID, resA.Source.RawID)
	}
	entries, err := os.ReadDir(filepath.Join(vaultRoot, "raw", "inbox"))
	if err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("inbox has %d entries, want 2", len(entries))
	}
}

// --- helpers ---

func newTestVault(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := vault.Init(root); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}
	return root
}

func openTestDB(t *testing.T, vaultRoot string) *index.DB {
	t.Helper()
	db, err := index.Open(vaultRoot)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func writeRandomFile(path string, size int) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	// 用 io.CopyN + crypto/rand 流式写入，避免一次性 alloc 10MiB。
	if _, err := copyRand(f, int64(size)); err != nil {
		return err
	}
	return nil
}

func copyRand(dst *os.File, n int64) (int64, error) {
	buf := make([]byte, 64*1024)
	var written int64
	for written < n {
		chunk := int64(len(buf))
		if remaining := n - written; remaining < chunk {
			chunk = remaining
		}
		if _, err := rand.Read(buf[:chunk]); err != nil {
			return written, err
		}
		w, err := dst.Write(buf[:chunk])
		written += int64(w)
		if err != nil {
			return written, err
		}
	}
	return written, nil
}
