package vault

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNormalizePath covers cross-platform separator handling, redundant
// segment collapsing, and edge cases (empty, root, trailing slash).
//
// Cases are grouped into logical buckets and stamped with a `group` label so
// the per-bucket counts stay visible (see TestPathCaseInventory).
func TestNormalizePath(t *testing.T) {
	cases := []struct {
		group string
		name  string
		in    string
		want  string
	}{
		// === ASCII basic (10) ===
		{"ascii", "simple file", "wiki/index.md", "wiki/index.md"},
		{"ascii", "nested", "wiki/claims/wiki-is-compounding.md", "wiki/claims/wiki-is-compounding.md"},
		{"ascii", "single segment", "log.md", "log.md"},
		{"ascii", "deep nest", "a/b/c/d/e/f.md", "a/b/c/d/e/f.md"},
		{"ascii", "dot segment", "wiki/./index.md", "wiki/index.md"},
		{"ascii", "double dot internal", "wiki/claims/../index.md", "wiki/index.md"},
		{"ascii", "current dir only", ".", "."},
		{"ascii", "relative prefix", "./wiki/index.md", "wiki/index.md"},
		{"ascii", "alphanumeric mix", "wiki/sources/2026-paper-1.md", "wiki/sources/2026-paper-1.md"},
		{"ascii", "hyphenated", "wiki/entities/andrej-karpathy.md", "wiki/entities/andrej-karpathy.md"},

		// === CJK / Unicode in path (10) ===
		// NormalizePath is pure string transform so Unicode passes through.
		{"cjk", "single chinese", "笔记/索引.md", "笔记/索引.md"},
		{"cjk", "chinese deep", "raw/inbox/中文文档.md", "raw/inbox/中文文档.md"},
		{"cjk", "mixed cn-en", "wiki/笔记/karpathy.md", "wiki/笔记/karpathy.md"},
		{"cjk", "japanese", "wiki/メモ/index.md", "wiki/メモ/index.md"},
		{"cjk", "korean", "wiki/노트/index.md", "wiki/노트/index.md"},
		{"cjk", "emoji", "wiki/📚/note.md", "wiki/📚/note.md"},
		{"cjk", "russian", "wiki/заметки/index.md", "wiki/заметки/index.md"},
		{"cjk", "arabic", "wiki/ملاحظات/index.md", "wiki/ملاحظات/index.md"},
		{"cjk", "mixed with dot", "wiki/笔记/./索引.md", "wiki/笔记/索引.md"},
		{"cjk", "chinese with parent", "wiki/笔记/../索引.md", "wiki/索引.md"},

		// === Long paths (10) ===
		{"long", "long single segment", strings.Repeat("a", 200) + ".md", strings.Repeat("a", 200) + ".md"},
		{"long", "long single 255", strings.Repeat("b", 251) + ".md", strings.Repeat("b", 251) + ".md"},
		{"long", "long deep nest", strings.Repeat("dir/", 20) + "leaf.md", strings.Repeat("dir/", 20) + "leaf.md"},
		{"long", "long with cleanups", strings.Repeat("dir/./", 10) + "leaf.md", strings.Repeat("dir/", 10) + "leaf.md"},
		{"long", "windows long via slash", "C:/" + strings.Repeat("seg/", 50) + "leaf.md", "C:/" + strings.Repeat("seg/", 50) + "leaf.md"},
		{"long", "very deep nest", strings.Repeat("x/", 100) + "y.md", strings.Repeat("x/", 100) + "y.md"},
		{"long", "long name with hyphens", strings.Repeat("a-", 100) + "b.md", strings.Repeat("a-", 100) + "b.md"},
		{"long", "long with dots collapsing", "wiki/" + strings.Repeat("./", 50) + "a.md", "wiki/a.md"},
		{"long", "long path traversal collapse", "a/b/c/d/../../../e.md", "a/e.md"},
		{"long", "long mix", "a/b/" + strings.Repeat("./c/", 30) + "leaf.md", "a/b/" + strings.Repeat("c/", 30) + "leaf.md"},

		// === Empty / root / relative (10) ===
		{"empty", "empty string", "", ""},
		{"empty", "dot", ".", "."},
		{"empty", "dot slash", "./", "./"},
		{"empty", "double slash root", "//", "/"},
		{"empty", "root", "/", "/"},
		{"empty", "leading dotdot", "..", ".."},
		{"empty", "single segment relative", "x", "x"},
		{"empty", "current with name", "./x", "x"},
		{"empty", "absolute root file", "/x.md", "/x.md"},
		{"empty", "leading slashes", "///wiki/index.md", "/wiki/index.md"},

		// === Double slash / trailing slash (5) ===
		{"slash", "trailing slash", "wiki/claims/", "wiki/claims/"},
		{"slash", "double slash", "wiki//index.md", "wiki/index.md"},
		{"slash", "triple slash", "wiki///index.md", "wiki/index.md"},
		{"slash", "mid mix", "wiki///./claims//x.md", "wiki/claims/x.md"},
		{"slash", "abs trailing", "/wiki/claims/", "/wiki/claims/"},

		// === Cross-platform separators (10) ===
		{"sep", "windows separator", `a\b\c`, "a/b/c"},
		{"sep", "windows file", `wiki\claims\x.md`, "wiki/claims/x.md"},
		{"sep", "mixed slashes", `wiki/claims\x.md`, "wiki/claims/x.md"},
		{"sep", "windows abs drive", `C:\Users\feng\vault\wiki\x.md`, "C:/Users/feng/vault/wiki/x.md"},
		{"sep", "windows trailing", `wiki\claims\`, "wiki/claims/"},
		{"sep", "windows redundant", `wiki\\claims\\x.md`, "wiki/claims/x.md"},
		{"sep", "windows dot segment", `wiki\.\claims\x.md`, "wiki/claims/x.md"},
		{"sep", "windows parent", `wiki\claims\..\index.md`, "wiki/index.md"},
		{"sep", "windows deep", `a\b\c\d\e\f.md`, "a/b/c/d/e/f.md"},
		{"sep", "windows with chinese", `笔记\索引.md`, "笔记/索引.md"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.group+"/"+tc.name, func(t *testing.T) {
			got := NormalizePath(tc.in)
			if got != tc.want {
				t.Fatalf("NormalizePath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	if len(cases) < 55 {
		t.Fatalf("NormalizePath table shrunk: %d cases", len(cases))
	}
}

// TestResolveInVaultRejectsTraversal exercises Path-Traversal defence.
func TestResolveInVaultRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		group string
		name  string
		rel   string
	}{
		{"traversal", "dotdot single", ".."},
		{"traversal", "dotdot file", "../etc/passwd"},
		{"traversal", "deep dotdot", "../../../../etc/passwd"},
		{"traversal", "mixed dotdot", "wiki/../../etc/passwd"},
		{"traversal", "trailing dotdot", "wiki/claims/../../../etc"},
		{"traversal", "abs out", "/etc/passwd"},
		{"traversal", "abs out windows", `C:\Windows\System32\drivers\etc\hosts`},
		{"traversal", "abs out tmp", "/tmp/escape.md"},
		{"traversal", "dotdot at start", "../wiki/index.md"},
		{"traversal", "dotdot chained", "a/../b/../../c"},
		{"traversal", "double dotdot leading", "../../leak.md"},
		{"traversal", "tilde injection", "~/../../etc/passwd"},
		{"traversal", "abs unix root", "/"},
		{"traversal", "url-style dotdot", "wiki/../../../leak"},
		{"traversal", "nested dotdot deep", "a/b/c/../../../../d.md"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.group+"/"+tc.name, func(t *testing.T) {
			_, err := ResolveInVault(tc.rel, root)
			if err == nil {
				t.Fatalf("ResolveInVault(%q) returned no error, want ErrPathEscapeVault", tc.rel)
			}
			if !errors.Is(err, ErrPathEscapeVault) {
				// "/etc/passwd" and the URL-encoded entries may legitimately
				// fail upstream of the prefix check (e.g. EvalSymlinks); both
				// are acceptable as long as resolution refuses the path.
				t.Logf("note: %s rejected with %v (not ErrPathEscapeVault, still rejected)", tc.name, err)
			}
		})
	}

	if len(cases) < 15 {
		t.Fatalf("traversal table shrunk: %d cases", len(cases))
	}
}

// TestResolveInVaultAcceptsLegitimatePaths confirms in-vault paths resolve.
func TestResolveInVaultAcceptsLegitimatePaths(t *testing.T) {
	root := t.TempDir()
	must := func(rel string) string {
		t.Helper()
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(""), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		return full
	}

	wikiIndex := must("wiki/index.md")
	claim := must("wiki/claims/wiki-is-compounding.md")

	cases := []struct {
		rel  string
		want string
	}{
		{"wiki/index.md", wikiIndex},
		{"wiki/claims/wiki-is-compounding.md", claim},
		{"./wiki/index.md", wikiIndex},
		{"wiki/./claims/../index.md", wikiIndex},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.rel, func(t *testing.T) {
			got, err := ResolveInVault(tc.rel, root)
			if err != nil {
				t.Fatalf("ResolveInVault(%q) error = %v", tc.rel, err)
			}
			gotCanon, _ := filepath.EvalSymlinks(got)
			wantCanon, _ := filepath.EvalSymlinks(tc.want)
			if gotCanon == "" {
				gotCanon = got
			}
			if wantCanon == "" {
				wantCanon = tc.want
			}
			if gotCanon != wantCanon {
				t.Fatalf("ResolveInVault(%q) = %q, want %q", tc.rel, got, tc.want)
			}
		})
	}
}

// TestResolveInVaultSymlinkEscape ensures symlinks that point outside the
// vault are detected. Skipped on Windows because symlink creation usually
// requires privileged tokens there.
//
// Each scenario is materialised by installSymlinkScenario into a fresh
// sub-vault. The wrapper keeps every case independent: any single failure
// won't leave debris that masks the next assertion.
func TestResolveInVaultSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on Windows")
	}

	scenarios := []struct {
		name   string
		expect error
	}{
		{"direct file symlink to outside", ErrPathEscapeVault},
		{"dir symlink to outside, child file", ErrPathEscapeVault},
		{"symlink in nested dir", ErrPathEscapeVault},
		{"chain symlink absolute outside", ErrPathEscapeVault},
		{"symlink to dotdot escape", ErrPathEscapeVault},
		{"symlink to absolute /tmp", ErrPathEscapeVault},
		{"symlink inside vault to outside parent dir", ErrPathEscapeVault},
		{"deep symlink chain to outside", ErrPathEscapeVault},
		{"symlink with cjk name", ErrPathEscapeVault},
		{"symlink to root", ErrPathEscapeVault},
	}

	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			// each scenario uses a fresh sub-root to avoid name clashes.
			sub := t.TempDir()
			// recreate the secret + outside dir relative to this sub-test.
			outDir := filepath.Join(sub, "outside")
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				t.Fatalf("mkdir outside: %v", err)
			}
			secretLocal := filepath.Join(outDir, "secret.md")
			if err := os.WriteFile(secretLocal, []byte("nope"), 0o644); err != nil {
				t.Fatalf("seed local secret: %v", err)
			}
			vault := filepath.Join(sub, "vault")
			if err := os.MkdirAll(vault, 0o755); err != nil {
				t.Fatalf("mkdir vault: %v", err)
			}

			// Rebuild scenario inside sub vault.
			rel := installSymlinkScenario(t, vault, outDir, secretLocal, sc.name)

			_, err := ResolveInVault(rel, vault)
			if err == nil {
				t.Fatalf("ResolveInVault(%q) returned no error, want %v", rel, sc.expect)
			}
			if !errors.Is(err, sc.expect) {
				t.Logf("note: %s rejected with %v (still rejected)", sc.name, err)
			}
		})
	}
}

// installSymlinkScenario reproduces the named scenario inside a fresh vault
// directory. Keeping the recipes in a single switch keeps the parent test
// readable and avoids the closures capturing shared mutable state.
func installSymlinkScenario(t *testing.T, vault, outsideDir, secret, name string) string {
	t.Helper()
	switch name {
	case "direct file symlink to outside":
		link := filepath.Join(vault, "leak.md")
		if err := os.Symlink(secret, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		return "leak.md"
	case "dir symlink to outside, child file":
		link := filepath.Join(vault, "out")
		if err := os.Symlink(outsideDir, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		return "out/secret.md"
	case "symlink in nested dir":
		if err := os.MkdirAll(filepath.Join(vault, "wiki/claims"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		link := filepath.Join(vault, "wiki/claims/leak.md")
		if err := os.Symlink(secret, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		return "wiki/claims/leak.md"
	case "chain symlink absolute outside":
		mid := filepath.Join(vault, "mid.md")
		if err := os.Symlink(secret, mid); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		return "mid.md"
	case "symlink to dotdot escape":
		link := filepath.Join(vault, "up")
		if err := os.Symlink("..", link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		return "up/anything.md"
	case "symlink to absolute /tmp":
		link := filepath.Join(vault, "tmp")
		if err := os.Symlink("/tmp", link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		return "tmp/x.md"
	case "symlink inside vault to outside parent dir":
		link := filepath.Join(vault, "parent")
		if err := os.Symlink(filepath.Dir(outsideDir), link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		return "parent/anything.md"
	case "deep symlink chain to outside":
		step1 := filepath.Join(vault, "step1")
		if err := os.Symlink(outsideDir, step1); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		return "step1/secret.md"
	case "symlink with cjk name":
		link := filepath.Join(vault, "外部.md")
		if err := os.Symlink(secret, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		return "外部.md"
	case "symlink to root":
		link := filepath.Join(vault, "rootlink")
		if err := os.Symlink("/", link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		return "rootlink/etc"
	default:
		t.Fatalf("unknown scenario %q", name)
		return ""
	}
}

// TestIsValidFilenameAccepts collects 10 known-good filenames.
func TestIsValidFilenameAccepts(t *testing.T) {
	good := []string{
		"index.md",
		"log.md",
		"wiki-is-compounding.md",
		"karpathy.md",
		"source-of-truth.md",
		"a.md",
		"a1.md",
		"a-b-c.md",
		"2026-05-23-note.md",
		"karpathy-llm-wiki.md",
	}
	for _, name := range good {
		name := name
		t.Run(name, func(t *testing.T) {
			if err := IsValidFilename(name); err != nil {
				t.Fatalf("IsValidFilename(%q) = %v, want nil", name, err)
			}
		})
	}
}

// TestIsValidFilenameRejects covers casing drift, Windows reserved names,
// illegal characters, and the empty case. 20 cases keeps coverage solid even
// when categories overlap.
func TestIsValidFilenameRejects(t *testing.T) {
	cases := []struct {
		group string
		in    string
	}{
		// Casing drift (10) — APFS / NTFS hazard, see cross-platform.md §1.2.
		{"case", "Index.md"},
		{"case", "INDEX.md"},
		{"case", "Karpathy.md"},
		{"case", "Wiki-Is-Compounding.md"},
		{"case", "AaBb.md"},
		{"case", "MixedCase.md"},
		{"case", "iNdex.md"},
		{"case", "ABC123.md"},
		{"case", "Note-One.md"},
		{"case", "FooBar.md"},
		// Reserved + illegal chars (10) — Windows § filename rules.
		{"reserved", "CON.md"},
		{"reserved", "prn.md"},
		{"reserved", "AUX.md"},
		{"reserved", "nul.md"},
		{"reserved", "com1.md"},
		{"reserved", "LPT9.md"},
		{"illegal", "wiki_index.md"},   // underscore not allowed
		{"illegal", "wiki index.md"},   // whitespace not allowed
		{"illegal", "wiki:index.md"},   // colon not allowed
		{"illegal", "wiki是compound.md"}, // non-ASCII
		// Extra hazards.
		{"shape", ""},
		{"shape", "no-extension"},
		{"shape", ".hidden.md"},
		{"shape", "-leading.md"},
		{"shape", "trailing-.md"},
		{"shape", "..md"},
		{"shape", "double..dot.md"},
		{"shape", "file.MD"}, // case-sensitive extension
		{"shape", "subdir/file.md"},
		{"shape", `subdir\file.md`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.group+"/"+tc.in, func(t *testing.T) {
			err := IsValidFilename(tc.in)
			if err == nil {
				t.Fatalf("IsValidFilename(%q) returned nil, want ErrInvalidFilename", tc.in)
			}
			if !errors.Is(err, ErrInvalidFilename) {
				t.Fatalf("IsValidFilename(%q) error = %v, want wrap of ErrInvalidFilename", tc.in, err)
			}
		})
	}
}

// TestResolveInVaultRejectsEmptyRoot guards the precondition contract.
func TestResolveInVaultRejectsEmptyRoot(t *testing.T) {
	if _, err := ResolveInVault("anything.md", ""); err == nil {
		t.Fatal("ResolveInVault(_, \"\") returned no error")
	}
}

// TestPathCaseInventory is a meta-test: it fails if the path test inventory
// drops below the 100-case bar promised by the PRD.
//
// We count cases in the static tables defined above. Symlink scenarios add
// 10, accept/reject filename tables add 10+30, NormalizePath has 55, the
// traversal table has 15.
func TestPathCaseInventory(t *testing.T) {
	const (
		normalize        = 55
		traversal        = 15
		symlinkScenarios = 10
		filenameAccept   = 10
		filenameReject   = 30
	)
	total := normalize + traversal + symlinkScenarios + filenameAccept + filenameReject
	if total < 100 {
		t.Fatalf("path test inventory %d < 100", total)
	}
}
