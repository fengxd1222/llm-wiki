package vault

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/schema"
)

func TestInitCreatesVaultStructure(t *testing.T) {
	root := filepath.Join(t.TempDir(), "knowledge")

	result, err := Init(root)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if result.Root != root {
		t.Fatalf("Root = %q, want %q", result.Root, root)
	}
	if result.SchemaVersion != schema.Version {
		t.Fatalf("SchemaVersion = %q, want %q", result.SchemaVersion, schema.Version)
	}

	for _, rel := range []string{
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
		"schema",
		".wikimind/audit",
		".wikimind/locks",
		".git",
	} {
		assertDir(t, filepath.Join(root, rel))
	}

	for _, rel := range []string{
		".wikimind/config.toml",
		"wiki/index.md",
		"wiki/log.md",
	} {
		assertFile(t, filepath.Join(root, rel))
	}

	config, err := os.ReadFile(filepath.Join(root, ".wikimind", "config.toml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(config), `schema_version = "1.0"`) {
		t.Fatalf("config missing schema_version: %s", config)
	}
}

func TestInitWritesDefaultSchemaTemplates(t *testing.T) {
	root := filepath.Join(t.TempDir(), "knowledge")
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	repo := repoRoot(t)
	for _, name := range schema.DefaultTemplateNames() {
		got, err := os.ReadFile(filepath.Join(root, "schema", name))
		if err != nil {
			t.Fatalf("read written template %s: %v", name, err)
		}
		want, err := os.ReadFile(filepath.Join(repo, "spec-v2", "templates", name))
		if err != nil {
			t.Fatalf("read source template %s: %v", name, err)
		}
		if string(got) != string(want) {
			t.Fatalf("template %s does not match source", name)
		}
	}
}

func TestInitRejectsExistingNonEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.md"), []byte("already here"), 0o644); err != nil {
		t.Fatalf("seed non-empty dir: %v", err)
	}

	_, err := Init(root)
	if !errors.Is(err, ErrNonEmptyDirectory) {
		t.Fatalf("Init() error = %v, want ErrNonEmptyDirectory", err)
	}
}

func TestReadStatusFromNestedDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "knowledge")
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	nested := filepath.Join(root, "wiki", "topics")
	status, err := ReadStatus(nested)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}
	if status.Root != root {
		t.Fatalf("Root = %q, want %q", status.Root, root)
	}
	if status.SchemaVersion != schema.Version {
		t.Fatalf("SchemaVersion = %q, want %q", status.SchemaVersion, schema.Version)
	}
	if status.RawFiles != 0 {
		t.Fatalf("RawFiles = %d, want 0", status.RawFiles)
	}
	if status.WikiPages != 2 {
		t.Fatalf("WikiPages = %d, want 2", status.WikiPages)
	}
	if status.Claims != 0 {
		t.Fatalf("Claims = %d, want 0", status.Claims)
	}
	if !status.Git.Available {
		t.Fatal("Git.Available = false, want true")
	}
	if status.Git.Clean {
		t.Fatal("Git.Clean = true, want false for untracked initialized files")
	}
}

func assertDir(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat dir %s: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", path)
	}
}

func assertFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file %s: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("%s is a directory", path)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
