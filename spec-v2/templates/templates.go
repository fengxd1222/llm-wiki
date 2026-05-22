// Package templates embeds the default WikiMind vault schema templates.
package templates

import (
	"embed"
	"fmt"
)

//go:embed *.md
var files embed.FS

var names = []string{
	"AGENTS.md",
	"CLAUDE.md",
	"CODEX.md",
	"CURSOR.md",
	"HERMES.md",
	"lint-rules.md",
	"page-schemas.md",
}

// Names returns the default template filenames in deterministic write order.
func Names() []string {
	return append([]string(nil), names...)
}

// ReadFile reads a known default template from the embedded template set.
func ReadFile(name string) ([]byte, error) {
	for _, allowed := range names {
		if name == allowed {
			return files.ReadFile(name)
		}
	}
	return nil, fmt.Errorf("unknown schema template %q", name)
}
