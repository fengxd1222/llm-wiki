package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultTemplateNamesReturnsCopy(t *testing.T) {
	names := DefaultTemplateNames()
	if len(names) != 7 {
		t.Fatalf("len(DefaultTemplateNames()) = %d, want 7", len(names))
	}

	names[0] = "changed.md"
	if DefaultTemplateNames()[0] == "changed.md" {
		t.Fatal("DefaultTemplateNames returned mutable shared backing array")
	}
}

func TestWriteDefaultTemplates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "schema")
	if err := WriteDefaultTemplates(dir); err != nil {
		t.Fatalf("WriteDefaultTemplates() error = %v", err)
	}

	for _, name := range DefaultTemplateNames() {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read template %s: %v", name, err)
		}
		if len(body) == 0 {
			t.Fatalf("template %s is empty", name)
		}
	}
}
