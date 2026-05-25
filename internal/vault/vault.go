package vault

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fengxd1222/llm-wiki/internal/schema"
)

// ErrNonEmptyDirectory is returned when init targets an existing non-empty directory.
var ErrNonEmptyDirectory = errors.New("vault directory already exists and is not empty")

// InitResult describes a newly initialized vault.
type InitResult struct {
	Root          string
	SchemaVersion string
}

// Status describes the current vault metadata surfaced by wikimind status.
type Status struct {
	Root          string
	SchemaVersion string
	RawFiles      int
	WikiPages     int
	Claims        int
	Git           GitStatus
}

// GitStatus is a small, CLI-oriented view of a vault git repository.
type GitStatus struct {
	Available bool
	Branch    string
	Clean     bool
	Detail    string
}

// Init creates a new WikiMind vault at root.
func Init(root string) (*InitResult, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("vault path is required")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve vault path: %w", err)
	}
	if err := prepareRoot(absRoot); err != nil {
		return nil, err
	}
	if err := createDirectories(absRoot); err != nil {
		return nil, err
	}
	if err := schema.WriteDefaultTemplates(filepath.Join(absRoot, "schema")); err != nil {
		return nil, err
	}
	if err := writeConfig(absRoot, time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := writeInitialWikiFiles(absRoot); err != nil {
		return nil, err
	}
	if err := writeVaultGitignore(absRoot); err != nil {
		return nil, err
	}
	if err := initGitIfNeeded(absRoot); err != nil {
		return nil, err
	}

	return &InitResult{Root: absRoot, SchemaVersion: schema.Version}, nil
}

// ReadStatus finds a vault from start and returns status metadata.
func ReadStatus(start string) (*Status, error) {
	root, err := FindRoot(start)
	if err != nil {
		return nil, err
	}
	version, err := readSchemaVersion(root)
	if err != nil {
		return nil, err
	}
	rawFiles, err := countRegularFiles(filepath.Join(root, "raw"))
	if err != nil {
		return nil, fmt.Errorf("count raw files: %w", err)
	}
	wikiPages, err := countMarkdownFiles(filepath.Join(root, "wiki"))
	if err != nil {
		return nil, fmt.Errorf("count wiki pages: %w", err)
	}
	claims, err := countMarkdownFiles(filepath.Join(root, "wiki", "claims"))
	if err != nil {
		return nil, fmt.Errorf("count claims: %w", err)
	}
	return &Status{
		Root:          root,
		SchemaVersion: version,
		RawFiles:      rawFiles,
		WikiPages:     wikiPages,
		Claims:        claims,
		Git:           readGitStatus(root),
	}, nil
}

// FindRoot walks upward from start until it finds .wikimind/config.toml.
func FindRoot(start string) (string, error) {
	if strings.TrimSpace(start) == "" {
		start = "."
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve status path: %w", err)
	}
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	for {
		if fileExists(filepath.Join(abs, ".wikimind", "config.toml")) {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no WikiMind vault found from %s", start)
		}
		abs = parent
	}
}

func prepareRoot(root string) error {
	info, err := os.Stat(root)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("vault path exists and is not a directory: %s", root)
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return fmt.Errorf("read existing vault directory: %w", err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("%w: %s", ErrNonEmptyDirectory, root)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect vault path: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create vault root: %w", err)
	}
	return nil
}

func createDirectories(root string) error {
	dirs := []string{
		"raw/inbox",
		"raw/imported",
		"raw/attachments",
		"raw/manifests",
		"wiki/claims",
		"wiki/entities",
		"wiki/concepts",
		"wiki/sources",
		"wiki/topics",
		"wiki/_review",
		"wiki/_reports",
		"wiki/_worktrees",
		"schema",
		".wikimind/audit",
		".wikimind/locks",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return fmt.Errorf("create vault directory %s: %w", dir, err)
		}
	}
	return nil
}

func writeInitialWikiFiles(root string) error {
	files := map[string]string{
		"wiki/index.md": "# WikiMind Index\n\nThis vault is ready for source ingestion.\n",
		"wiki/log.md":   "# WikiMind Log\n\n",
	}
	for rel, body := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
	}
	return nil
}

func writeVaultGitignore(root string) error {
	body := strings.Join([]string{
		".wikimind/index.db",
		".wikimind/index.db-*",
		".wikimind/*.bak",
		"wiki/_worktrees/",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(body), 0o644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}
	return nil
}

func initGitIfNeeded(root string) error {
	if ok, _ := isInsideGitWorkTree(root); ok {
		return nil
	}
	if _, err := runGit(root, "init", "--initial-branch=main"); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

func isInsideGitWorkTree(root string) (bool, error) {
	out, err := runGit(root, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

func readGitStatus(root string) GitStatus {
	if ok, err := isInsideGitWorkTree(root); !ok || err != nil {
		return GitStatus{Available: false, Detail: "not a git repository"}
	}

	branch, err := runGit(root, "branch", "--show-current")
	if err != nil || strings.TrimSpace(branch) == "" {
		branch, _ = runGit(root, "symbolic-ref", "--short", "HEAD")
	}
	branch = strings.TrimSpace(branch)
	if branch == "" || branch == "HEAD" {
		branch = "unknown"
	}

	shortStatus, err := runGit(root, "status", "--porcelain")
	if err != nil {
		return GitStatus{Available: true, Branch: branch, Clean: false, Detail: err.Error()}
	}
	return GitStatus{
		Available: true,
		Branch:    branch,
		Clean:     strings.TrimSpace(shortStatus) == "",
	}
}

func runGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), errors.New(msg)
	}
	return stdout.String(), nil
}

func countRegularFiles(root string) (int, error) {
	return countFiles(root, func(path string, d fs.DirEntry) bool {
		return d.Type().IsRegular()
	})
}

func countMarkdownFiles(root string) (int, error) {
	return countFiles(root, func(path string, d fs.DirEntry) bool {
		return d.Type().IsRegular() && strings.EqualFold(filepath.Ext(path), ".md")
	})
}

func countFiles(root string, include func(string, fs.DirEntry) bool) (int, error) {
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if include(path, d) {
			count++
		}
		return nil
	})
	return count, err
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
