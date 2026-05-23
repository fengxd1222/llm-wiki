package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/vault"
)

func TestInitAndStatusCommands(t *testing.T) {
	root := filepath.Join(t.TempDir(), "knowledge")

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"init", root})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "initialized: "+root) {
		t.Fatalf("init output = %q, want initialized root", out.String())
	}

	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"status", filepath.Join(root, "wiki", "topics")})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status Execute() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"vault: " + root,
		"schema_version: 1.0",
		"raw_files: 0",
		"wiki_pages: 2",
		"claims: 0",
		"git_status: dirty",
		"config: ok",
		"health: ok",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q:\n%s", want, got)
		}
	}
}

func TestIngestCommand(t *testing.T) {
	tempDir := t.TempDir()
	vaultRoot := filepath.Join(tempDir, "vault")
	if _, err := vault.Init(vaultRoot); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}

	srcPath := filepath.Join(tempDir, "sample.md")
	srcBody := []byte("# Sample\n\nHello WikiMind.\n")
	if err := os.WriteFile(srcPath, srcBody, 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	// chdir into vault so vault.FindRoot picks it up; t.Chdir restores cwd.
	t.Chdir(vaultRoot)

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"ingest", srcPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ingest Execute() error = %v\nout=%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{
		"ingested: raw/inbox/sample.md",
		"sha256: ",
		"size: ",
		"status: pending",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ingest output missing %q:\n%s", want, got)
		}
	}

	// Copy must land in raw/inbox/.
	if _, err := os.Stat(filepath.Join(vaultRoot, "raw", "inbox", "sample.md")); err != nil {
		t.Fatalf("ingested file missing: %v", err)
	}

	// Second ingest of same file → duplicate marker, no second copy under a different name.
	out.Reset()
	cmd = newRootCommand(&out, &out)
	cmd.SetArgs([]string{"ingest", srcPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("second ingest error = %v\nout=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "duplicate: raw/inbox/sample.md") {
		t.Fatalf("second ingest output missing duplicate marker:\n%s", out.String())
	}
}

func TestStubCommands(t *testing.T) {
	for _, name := range []string{"query", "review", "lint", "revert"} {
		var out bytes.Buffer
		cmd := newRootCommand(&out, &out)
		cmd.SetArgs([]string{name})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s Execute() error = %v", name, err)
		}
		want := "wikimind " + name + ": D1 未实现\n"
		if out.String() != want {
			t.Fatalf("%s output = %q, want %q", name, out.String(), want)
		}
	}
}
