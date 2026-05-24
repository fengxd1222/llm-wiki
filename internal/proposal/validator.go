package proposal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/vault"
)

var (
	ErrPathNotAllowed          = errors.New("PATH_NOT_ALLOWED")
	ErrSchemaViolation         = errors.New("SCHEMA_VIOLATION")
	ErrBaseHashMismatch        = errors.New("BASE_HASH_MISMATCH")
	ErrQuoteHashMismatch       = errors.New("QUOTE_HASH_MISMATCH")
	ErrProvenanceDepthExceeded = errors.New("PROVENANCE_DEPTH_EXCEEDED")
)

var claimIDPattern = regexp.MustCompile(`^cl-\d{4}-\d{2}-\d{2}-\d{3}$`)

type ValidationResult struct {
	SchemaCheck    string   `json:"schema_check"`
	QuoteHashCheck string   `json:"quote_hash_check"`
	PathCheck      string   `json:"path_check"`
	BaseHashCheck  string   `json:"base_hash_check,omitempty"`
	Errors         []string `json:"errors,omitempty"`
}

type ClaimSource struct {
	RawID     string
	Anchor    string
	Quote     string
	QuoteHash string
	Span      []int
}

func ValidatePath(path, pageType string) error {
	normalized := vault.NormalizePath(path)
	if normalized == "." || strings.HasPrefix(normalized, "../") ||
		strings.HasPrefix(normalized, "/") || !strings.HasSuffix(normalized, ".md") {
		return fmt.Errorf("%w: %s", ErrPathNotAllowed, path)
	}
	allowed := map[string]string{
		"claim":   "wiki/claims/",
		"entity":  "wiki/entities/",
		"concept": "wiki/concepts/",
		"source":  "wiki/sources/",
		"topic":   "wiki/topics/",
	}
	prefix, ok := allowed[pageType]
	if !ok || !strings.HasPrefix(normalized, prefix) {
		return fmt.Errorf("%w: type=%s path=%s", ErrPathNotAllowed, pageType, path)
	}
	return nil
}

func ValidateClaimID(id string) error {
	if !claimIDPattern.MatchString(strings.TrimSpace(id)) {
		return fmt.Errorf("%w: invalid claim_id %q", ErrSchemaViolation, id)
	}
	return nil
}

func ValidateFrontmatter(fm map[string]any, pageType string) error {
	if fm == nil {
		return fmt.Errorf("%w: frontmatter required", ErrSchemaViolation)
	}
	title, _ := fm["title"].(string)
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("%w: title required", ErrSchemaViolation)
	}
	gotType, _ := fm["type"].(string)
	if gotType != pageType {
		return fmt.Errorf("%w: type=%q want %q", ErrSchemaViolation, gotType, pageType)
	}
	return nil
}

func ValidateBaseHash(ctx context.Context, vaultRoot, path, declaredBaseHash string) error {
	declaredBaseHash = strings.TrimSpace(declaredBaseHash)
	if declaredBaseHash == "" {
		return fmt.Errorf("%w: base_hash required", ErrBaseHashMismatch)
	}
	out, err := runGit(ctx, vaultRoot, "show", "main:"+filepath.ToSlash(path))
	if err != nil {
		return fmt.Errorf("%w: read base page %s: %v", ErrBaseHashMismatch, path, err)
	}
	current, err := PageRawContentHash([]byte(out))
	if err != nil {
		return fmt.Errorf("%w: hash base page %s: %v", ErrBaseHashMismatch, path, err)
	}
	if current != declaredBaseHash {
		return fmt.Errorf("%w: got %s want %s", ErrBaseHashMismatch, current, declaredBaseHash)
	}
	return nil
}

func ValidateClaimSources(ctx context.Context, vaultRoot string, sources []ClaimSource) error {
	if len(sources) == 0 {
		return fmt.Errorf("%w: sources required", ErrSchemaViolation)
	}
	for _, src := range sources {
		rawID := vault.NormalizePath(src.RawID)
		if !strings.HasPrefix(rawID, "raw/") {
			return fmt.Errorf("%w: %s", ErrProvenanceDepthExceeded, src.RawID)
		}
		abs, err := vault.ResolveInVault(rawID, vaultRoot)
		if err != nil {
			return fmt.Errorf("claim source path: %w", err)
		}
		body, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("read claim source %s: %w", rawID, err)
		}
		text, _, err := index.ResolveAnchor(body, src.Anchor)
		if err != nil {
			return fmt.Errorf("resolve claim source %s%s: %w", rawID, src.Anchor, err)
		}
		current := index.QuoteHash(text)
		if current != strings.TrimSpace(src.QuoteHash) {
			return fmt.Errorf("%w: %s%s stored=%s current=%s",
				ErrQuoteHashMismatch, rawID, src.Anchor, src.QuoteHash, current)
		}
	}
	return nil
}

func PageRawContentHash(raw []byte) (string, error) {
	fm, body, err := splitFrontmatter(raw)
	if err != nil {
		return "", err
	}
	return PageContentHash(fm, string(body)), nil
}

func PageContentHash(frontmatter map[string]any, body string) string {
	fmBytes, _ := json.Marshal(normalizeMap(frontmatter))
	normalizedBody := strings.TrimRight(strings.ReplaceAll(body, "\r\n", "\n"), "\n") + "\n"
	sum := sha256.Sum256([]byte(string(fmBytes) + "\n---\n" + normalizedBody))
	return hex.EncodeToString(sum[:])[:16]
}

func PageContentHashFromJSON(frontmatterJSON, body string) string {
	var fm map[string]any
	if strings.TrimSpace(frontmatterJSON) != "" {
		_ = json.Unmarshal([]byte(frontmatterJSON), &fm)
	}
	return PageContentHash(fm, body)
}

func EncodePage(frontmatter map[string]any, body string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(frontmatter); err != nil {
		return nil, fmt.Errorf("encode frontmatter: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("close frontmatter encoder: %w", err)
	}
	buf.WriteString("---\n\n")
	buf.WriteString(strings.TrimLeft(body, "\r\n"))
	if !strings.HasSuffix(buf.String(), "\n") {
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func splitFrontmatter(raw []byte) (map[string]any, []byte, error) {
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}
	if !bytes.HasPrefix(raw, []byte("---\n")) && !bytes.HasPrefix(raw, []byte("---\r\n")) {
		return nil, raw, nil
	}
	lines := bytes.SplitN(raw, []byte("\n"), 2)
	if len(lines) < 2 {
		return nil, nil, errors.New("frontmatter delimiter not closed")
	}
	rest := lines[1]
	offset := 0
	for offset < len(rest) {
		lineEnd := bytes.IndexByte(rest[offset:], '\n')
		var line []byte
		if lineEnd < 0 {
			line = rest[offset:]
		} else {
			line = rest[offset : offset+lineEnd]
		}
		if string(bytes.TrimRight(line, "\r")) == "---" {
			yamlBlock := rest[:offset]
			bodyStart := offset + len(line)
			for bodyStart < len(rest) && (rest[bodyStart] == '\r' || rest[bodyStart] == '\n') {
				bodyStart++
				if rest[bodyStart-1] == '\n' {
					break
				}
			}
			var fm map[string]any
			if err := yaml.Unmarshal(yamlBlock, &fm); err != nil {
				return nil, nil, err
			}
			return fm, rest[bodyStart:], nil
		}
		if lineEnd < 0 {
			break
		}
		offset += lineEnd + 1
	}
	return nil, nil, errors.New("frontmatter delimiter not closed")
}

func normalizeMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}
