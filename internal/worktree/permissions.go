package worktree

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	ErrRawWriteForbidden      = errors.New("raw writes are forbidden in worktree")
	ErrSchemaWriteForbidden   = errors.New("schema writes are forbidden in worktree")
	ErrWorktreeWriteForbidden = errors.New("nested worktree writes are forbidden")
	ErrPathOutsideWorktree    = errors.New("path is outside worktree write scope")
)

// IsWorktreeWriteAllowed implements the agent-protocol §4.2 write matrix.
func IsWorktreeWriteAllowed(relPath string) error {
	rel := filepath.ToSlash(strings.TrimSpace(relPath))
	if rel == "" || filepath.IsAbs(relPath) {
		return fmt.Errorf("%w: %q", ErrPathOutsideWorktree, relPath)
	}
	clean := filepath.ToSlash(filepath.Clean(rel))
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return fmt.Errorf("%w: %q", ErrPathOutsideWorktree, relPath)
	}

	switch {
	case clean == "raw" || strings.HasPrefix(clean, "raw/"):
		return fmt.Errorf("%w: %s", ErrRawWriteForbidden, clean)
	case clean == "schema" || strings.HasPrefix(clean, "schema/"):
		return fmt.Errorf("%w: %s", ErrSchemaWriteForbidden, clean)
	case clean == "wiki/_worktrees" || strings.HasPrefix(clean, "wiki/_worktrees/") ||
		clean == "_worktrees" || strings.HasPrefix(clean, "_worktrees/"):
		return fmt.Errorf("%w: %s", ErrWorktreeWriteForbidden, clean)
	case clean == "wiki" || strings.HasPrefix(clean, "wiki/"):
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrPathOutsideWorktree, clean)
	}
}
