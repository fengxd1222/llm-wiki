package proposal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

func TestValidatePath(t *testing.T) {
	cases := []struct {
		path     string
		pageType string
		ok       bool
	}{
		{"wiki/claims/a.md", "claim", true},
		{"wiki/entities/a.md", "entity", true},
		{"wiki/concepts/a.md", "concept", true},
		{"wiki/sources/a.md", "source", true},
		{"wiki/topics/a.md", "topic", true},
		{"wiki/entities/a.md", "claim", false},
		{"../wiki/claims/a.md", "claim", false},
		{"raw/inbox/a.md", "source", false},
	}
	for _, tc := range cases {
		err := ValidatePath(tc.path, tc.pageType)
		if tc.ok && err != nil {
			t.Fatalf("ValidatePath(%q,%q) = %v, want nil", tc.path, tc.pageType, err)
		}
		if !tc.ok && !errors.Is(err, ErrPathNotAllowed) {
			t.Fatalf("ValidatePath(%q,%q) = %v, want ErrPathNotAllowed", tc.path, tc.pageType, err)
		}
	}
}

func TestValidateFrontmatter(t *testing.T) {
	if err := ValidateFrontmatter(map[string]any{"type": "claim", "title": "A"}, "claim"); err != nil {
		t.Fatalf("ValidateFrontmatter valid: %v", err)
	}
	for _, fm := range []map[string]any{
		{"type": "claim"},
		{"type": "entity", "title": "A"},
	} {
		if err := ValidateFrontmatter(fm, "claim"); !errors.Is(err, ErrSchemaViolation) {
			t.Fatalf("ValidateFrontmatter(%v) = %v, want ErrSchemaViolation", fm, err)
		}
	}
}

func TestValidateClaimSources(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	rawRel := filepath.Join(root, "raw", "inbox")
	if err := os.MkdirAll(rawRel, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "# Source\n\nThe answer is forty two.\n"
	if err := os.WriteFile(filepath.Join(rawRel, "s.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	quote, _, err := index.ResolveAnchor([]byte(body), "#para-1")
	if err != nil {
		t.Fatalf("ResolveAnchor: %v", err)
	}
	src := ClaimSource{RawID: "raw/inbox/s.md", Anchor: "#para-1", QuoteHash: index.QuoteHash(quote)}
	if err := ValidateClaimSources(ctx, root, []ClaimSource{src}); err != nil {
		t.Fatalf("ValidateClaimSources valid: %v", err)
	}
	src.QuoteHash = "deadbeef"
	if err := ValidateClaimSources(ctx, root, []ClaimSource{src}); !errors.Is(err, ErrQuoteHashMismatch) {
		t.Fatalf("ValidateClaimSources mismatch = %v, want ErrQuoteHashMismatch", err)
	}
	src.RawID = "wiki/claims/x.md"
	if err := ValidateClaimSources(ctx, root, []ClaimSource{src}); !errors.Is(err, ErrProvenanceDepthExceeded) {
		t.Fatalf("ValidateClaimSources provenance = %v, want ErrProvenanceDepthExceeded", err)
	}
}

func TestPageContentHashStable(t *testing.T) {
	fm := map[string]any{"type": "claim", "title": "A"}
	h1 := PageContentHash(fm, "body\n")
	h2 := PageContentHash(fm, "body\n\n")
	if h1 != h2 {
		t.Fatalf("hash should ignore trailing blank lines: %s vs %s", h1, h2)
	}
	if len(h1) != 16 {
		t.Fatalf("hash length = %d, want 16", len(h1))
	}
}

func TestValidateBaseHash(t *testing.T) {
	ctx := context.Background()
	root := committedRepo(t)
	rel := "wiki/claims/base.md"
	if err := os.MkdirAll(filepath.Join(root, "wiki", "claims"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content, err := EncodePage(map[string]any{"type": "claim", "title": "Base"}, "# Base\n")
	if err != nil {
		t.Fatalf("EncodePage: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mustGit(t, root, "add", ".")
	mustGit(t, root, "-c", "user.name=WikiMind Test", "-c", "user.email=test@example.com", "commit", "-m", "base")
	hash, err := PageRawContentHash(content)
	if err != nil {
		t.Fatalf("PageRawContentHash: %v", err)
	}
	if err := ValidateBaseHash(ctx, root, rel, hash); err != nil {
		t.Fatalf("ValidateBaseHash valid: %v", err)
	}
	if err := ValidateBaseHash(ctx, root, rel, "bad"); !errors.Is(err, ErrBaseHashMismatch) {
		t.Fatalf("ValidateBaseHash mismatch = %v, want ErrBaseHashMismatch", err)
	}
}
