package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

// IngestStatusPending 是新写入 source 的初始状态（D3 没有 parser，固定 pending）。
const IngestStatusPending = "pending"

// ErrSourceMissing 表示给定的源路径不存在。
var ErrSourceMissing = errors.New("ingest source missing")

// ErrSourceUnreadable 表示源路径存在但无法读取（权限 / 目录等）。
var ErrSourceUnreadable = errors.New("ingest source unreadable")

// ErrInvalidVaultRoot 表示 vault 路径无效（空、非目录等）。
var ErrInvalidVaultRoot = errors.New("invalid vault root")

// IngestResult 描述一次 ingest 调用的结果。
type IngestResult struct {
	Source    *index.SourceRow
	Duplicate bool // 命中已有 sha256，未新写入。
	WrittenTo string
}

// IngestFile 把外部文件复制到 vaultRoot/raw/inbox/<basename>，
// 算 sha256/mtime/size 三件套，并 UPSERT 到 sources 表。
//
// 去重：若同 sha256 已有 row，跳过文件复制和 INSERT，返回已有 row（Duplicate=true）。
// 复制策略：原文件保留不动（architecture §1 raw 只读哲学）。
func IngestFile(
	ctx context.Context,
	db *index.DB,
	vaultRoot, srcPath string,
) (*IngestResult, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: index handle is nil", index.ErrIndexUnavailable)
	}
	if err := validateVaultRoot(vaultRoot); err != nil {
		return nil, err
	}
	srcAbs, info, err := openSource(srcPath)
	if err != nil {
		return nil, err
	}

	// 1) 流式 sha256（O(1) 内存，PDF / 音频大文件友好）。
	sum, err := sha256File(srcAbs)
	if err != nil {
		return nil, fmt.Errorf("%w: hash %s: %v", ErrSourceUnreadable, srcAbs, err)
	}

	// 2) 去重命中：跳过文件复制 + INSERT。
	existing, err := index.FindSourceBySHA256(ctx, db, sum)
	if err != nil {
		return nil, fmt.Errorf("lookup existing source: %w", err)
	}
	if existing != nil {
		return &IngestResult{
			Source:    existing,
			Duplicate: true,
			WrittenTo: filepath.Join(vaultRoot, filepath.FromSlash(existing.RawID)),
		}, nil
	}

	// 3) 复制 srcAbs → vaultRoot/raw/inbox/<basename>，basename 冲突时加 sha256 前 8 位。
	rawID, destAbs, err := copyIntoInbox(srcAbs, vaultRoot, sum)
	if err != nil {
		return nil, err
	}

	row := &index.SourceRow{
		RawID:      rawID,
		SHA256:     sum,
		Size:       info.Size(),
		MTime:      info.ModTime().Unix(),
		Status:     IngestStatusPending,
		IngestedAt: time.Now().UTC().Unix(),
	}
	if err := index.InsertSource(ctx, db, row); err != nil {
		// 写入失败时回滚已复制文件（best-effort）。
		_ = os.Remove(destAbs)
		return nil, err
	}
	return &IngestResult{Source: row, WrittenTo: destAbs}, nil
}

// validateVaultRoot 确认 vaultRoot 指向一个已存在的目录。
func validateVaultRoot(vaultRoot string) error {
	if strings.TrimSpace(vaultRoot) == "" {
		return fmt.Errorf("%w: vault root is empty", ErrInvalidVaultRoot)
	}
	info, err := os.Stat(vaultRoot)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVaultRoot, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: not a directory: %s", ErrInvalidVaultRoot, vaultRoot)
	}
	return nil
}

// openSource 校验源路径并返回绝对路径 + FileInfo。
func openSource(srcPath string) (string, os.FileInfo, error) {
	if strings.TrimSpace(srcPath) == "" {
		return "", nil, fmt.Errorf("%w: source path is empty", ErrSourceMissing)
	}
	abs, err := filepath.Abs(srcPath)
	if err != nil {
		return "", nil, fmt.Errorf("%w: resolve path: %v", ErrSourceMissing, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, fmt.Errorf("%w: %s", ErrSourceMissing, abs)
		}
		if errors.Is(err, os.ErrPermission) {
			return "", nil, fmt.Errorf("%w: stat %s: %v", ErrSourceUnreadable, abs, err)
		}
		return "", nil, fmt.Errorf("%w: stat %s: %v", ErrSourceUnreadable, abs, err)
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("%w: %s is a directory", ErrSourceUnreadable, abs)
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("%w: %s is not a regular file", ErrSourceUnreadable, abs)
	}

	// 早开一次确认可读，让权限错误立刻冒出来。
	f, err := os.Open(abs)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return "", nil, fmt.Errorf("%w: open %s: %v", ErrSourceUnreadable, abs, err)
		}
		return "", nil, fmt.Errorf("%w: open %s: %v", ErrSourceUnreadable, abs, err)
	}
	_ = f.Close()
	return abs, info, nil
}

// sha256File 流式计算文件 SHA-256，O(1) 内存。
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyIntoInbox 把 srcAbs 复制到 vaultRoot/raw/inbox/<basename>，
// 返回 vault-relative POSIX raw_id 与目标绝对路径。
//
// basename 冲突（同名不同内容）时追加 "-<sha256[:8]>" 后缀避免覆盖。
// 不动原文件（architecture §1：raw 只读、不可变）。
func copyIntoInbox(srcAbs, vaultRoot, sum string) (string, string, error) {
	inboxAbs := filepath.Join(vaultRoot, "raw", "inbox")
	if err := os.MkdirAll(inboxAbs, 0o755); err != nil {
		return "", "", fmt.Errorf("create raw/inbox: %w", err)
	}

	base := filepath.Base(srcAbs)
	destAbs := filepath.Join(inboxAbs, base)
	if _, err := os.Stat(destAbs); err == nil {
		// 同名已存在：加 sha256 前 8 位避免覆盖。
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		base = fmt.Sprintf("%s-%s%s", stem, sum[:8], ext)
		destAbs = filepath.Join(inboxAbs, base)
	}

	if err := copyFile(srcAbs, destAbs); err != nil {
		return "", "", fmt.Errorf("copy to raw/inbox: %w", err)
	}

	rawID := "raw/inbox/" + base
	return rawID, destAbs, nil
}

// copyFile 复制 src → dst，使用流式 io.Copy（大文件友好）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}
