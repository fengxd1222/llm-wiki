package schema

import (
	"fmt"
	"os"
	"path/filepath"

	schematemplates "github.com/fengxd1222/llm-wiki/spec-v2/templates"
)

// Version is the current vault schema version written by wikimind init.
const Version = "1.0"

// DefaultTemplateNames returns the filenames created under a new vault's schema directory.
func DefaultTemplateNames() []string {
	return schematemplates.Names()
}

// WriteDefaultTemplates writes the embedded default schema templates to dir.
func WriteDefaultTemplates(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create schema directory: %w", err)
	}

	for _, name := range schematemplates.Names() {
		body, err := schematemplates.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read embedded schema template %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			return fmt.Errorf("write schema template %s: %w", name, err)
		}
	}
	return nil
}
