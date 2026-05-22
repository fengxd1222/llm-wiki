package templates

import "testing"

func TestNamesReturnsDefaultTemplateSet(t *testing.T) {
	got := Names()
	want := []string{
		"AGENTS.md",
		"CLAUDE.md",
		"CODEX.md",
		"CURSOR.md",
		"HERMES.md",
		"lint-rules.md",
		"page-schemas.md",
	}
	if len(got) != len(want) {
		t.Fatalf("len(Names()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Names()[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	got[0] = "changed.md"
	if Names()[0] == "changed.md" {
		t.Fatal("Names returned mutable shared backing array")
	}
}

func TestReadFile(t *testing.T) {
	body, err := ReadFile("AGENTS.md")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(body) == 0 {
		t.Fatal("AGENTS.md embedded body is empty")
	}

	if _, err := ReadFile("unknown.md"); err == nil {
		t.Fatal("ReadFile(unknown.md) error = nil, want error")
	}
}
