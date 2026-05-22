package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
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
		"health: ok",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q:\n%s", want, got)
		}
	}
}

func TestStubCommands(t *testing.T) {
	for _, name := range []string{"ingest", "query", "review", "lint", "revert"} {
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
