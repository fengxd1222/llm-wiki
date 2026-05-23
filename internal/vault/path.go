package vault

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrPathEscapeVault is returned when a candidate path resolves outside the vault root.
var ErrPathEscapeVault = errors.New("path escapes vault root")

// ErrInvalidFilename is returned when a filename violates vault naming rules.
var ErrInvalidFilename = errors.New("invalid filename")

// filenamePattern enforces the vault filename rule:
// lowercase ASCII letters / digits, single hyphen separators, .md suffix.
// The stem must start and end with [a-z0-9] (no leading / trailing hyphens).
// See cross-platform.md §1.1.
var filenamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?\.md$`)

// windowsReserved lists DOS device names that NTFS rejects regardless of
// extension. Vault filenames must avoid them even on POSIX hosts so vaults
// remain portable to Windows.
var windowsReserved = map[string]struct{}{
	"con": {}, "prn": {}, "aux": {}, "nul": {},
	"com1": {}, "com2": {}, "com3": {}, "com4": {}, "com5": {},
	"com6": {}, "com7": {}, "com8": {}, "com9": {},
	"lpt1": {}, "lpt2": {}, "lpt3": {}, "lpt4": {}, "lpt5": {},
	"lpt6": {}, "lpt7": {}, "lpt8": {}, "lpt9": {},
}

// NormalizePath rewrites p into the vault's canonical POSIX form.
//
// Internally WikiMind stores paths with forward slashes; system calls convert
// them back via filepath.FromSlash. NormalizePath also collapses redundant
// separators and "." segments via filepath.Clean while preserving relative vs
// absolute distinction (a leading "/" stays, a relative path stays relative).
//
// Backslashes are rewritten to "/" unconditionally so that Windows-flavored
// paths round-trip on POSIX hosts (and vice versa). This is intentional: we
// require backslashes to never appear in vault filenames (see IsValidFilename
// + cross-platform.md §1.1) so the substitution is safe.
func NormalizePath(p string) string {
	if p == "" {
		return ""
	}
	// 1) Force backslashes to forward slashes regardless of host OS. On
	//    Windows, filepath.ToSlash already does this; on POSIX it is a no-op,
	//    so we always rewrite manually for consistent behaviour.
	posix := strings.ReplaceAll(p, `\`, "/")

	// 2) Re-run ToSlash for symmetry on any future separator changes.
	posix = filepath.ToSlash(posix)

	// 3) Clean collapses "./", "//" and resolves "..". We operate the POSIX
	//    form by hand because filepath.Clean is OS-aware and would re-insert
	//    backslashes on Windows.
	cleaned := cleanPOSIX(posix)

	// 4) Clean trims trailing slashes; preserve a single trailing slash if
	//    the caller signalled directory intent (handy for display) — but only
	//    when the cleaned form has length > 1 to avoid emitting an empty path.
	if cleaned != "/" && strings.HasSuffix(posix, "/") && !strings.HasSuffix(cleaned, "/") {
		cleaned += "/"
	}
	return cleaned
}

// cleanPOSIX is filepath.Clean restricted to "/" separators, regardless of OS.
//
// We can't rely on filepath.Clean directly because on Windows it would emit
// "\" separators. Doing the equivalent walk by hand keeps NormalizePath's
// output identical on macOS / Linux / Windows.
func cleanPOSIX(p string) string {
	if p == "" {
		return ""
	}
	rooted := strings.HasPrefix(p, "/")

	// Walk the parts, dropping "" and "." and applying ".." against the stack
	// when possible.
	parts := strings.Split(p, "/")
	stack := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(stack) > 0 && stack[len(stack)-1] != ".." {
				stack = stack[:len(stack)-1]
				continue
			}
			if rooted {
				// "/" + ".." is "/" in POSIX semantics; drop the "..".
				continue
			}
			stack = append(stack, "..")
		default:
			stack = append(stack, part)
		}
	}

	cleaned := strings.Join(stack, "/")
	if rooted {
		cleaned = "/" + cleaned
	}
	if cleaned == "" {
		return "."
	}
	return cleaned
}

// ResolveInVault joins rel onto vaultRoot, follows symlinks, and ensures the
// result stays inside vaultRoot.
//
// It rejects:
//   - "" / vaultRoot inputs that yield an empty resolved path,
//   - traversal via "..",
//   - symlinks whose target leaves the vault.
//
// vaultRoot is normalized to an absolute, symlink-resolved canonical form
// before comparison so that ../ inside the vault tree behaves as expected.
func ResolveInVault(rel, vaultRoot string) (string, error) {
	if strings.TrimSpace(vaultRoot) == "" {
		return "", errors.New("vault root is required")
	}
	absRoot, err := filepath.Abs(vaultRoot)
	if err != nil {
		return "", fmt.Errorf("resolve vault root: %w", err)
	}
	canonRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		// vault root may legitimately not exist yet in some unit tests; fall
		// back to a cleaned absolute path so we can still enforce traversal.
		canonRoot = filepath.Clean(absRoot)
	}

	// Reject inputs that are themselves absolute or contain a Windows drive
	// letter — the caller asked for an in-vault relative path, so an
	// absolute target is unambiguously an escape attempt.
	cleanedRel := strings.ReplaceAll(rel, `\`, "/")
	if strings.HasPrefix(cleanedRel, "/") {
		return "", fmt.Errorf("%w: absolute path %q", ErrPathEscapeVault, rel)
	}
	if len(cleanedRel) >= 2 && cleanedRel[1] == ':' {
		// e.g. C:/Users/... or C:\Users\... — Windows drive-rooted path.
		return "", fmt.Errorf("%w: drive-rooted path %q", ErrPathEscapeVault, rel)
	}

	candidate := filepath.Join(absRoot, filepath.FromSlash(rel))
	candidate = filepath.Clean(candidate)

	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		// Allow non-existent leaf paths: walk up to the closest existing
		// ancestor, resolve symlinks there, then re-append the missing tail.
		resolved, err = resolveBestEffort(candidate)
		if err != nil {
			return "", fmt.Errorf("resolve path: %w", err)
		}
	}

	if !withinRoot(resolved, canonRoot) {
		return "", fmt.Errorf("%w: %s", ErrPathEscapeVault, rel)
	}
	return resolved, nil
}

// resolveBestEffort returns an absolute path whose ancestors have been
// canonicalised via EvalSymlinks; the missing tail (file that does not yet
// exist) is appended verbatim. This lets ResolveInVault evaluate proposed
// destinations before they are created.
func resolveBestEffort(path string) (string, error) {
	parent := path
	for {
		resolved, err := filepath.EvalSymlinks(parent)
		if err == nil {
			rel, relErr := filepath.Rel(parent, path)
			if relErr != nil {
				return "", relErr
			}
			return filepath.Clean(filepath.Join(resolved, rel)), nil
		}
		next := filepath.Dir(parent)
		if next == parent {
			return filepath.Clean(path), nil
		}
		parent = next
	}
}

func withinRoot(candidate, root string) bool {
	candidate = filepath.Clean(candidate)
	root = filepath.Clean(root)
	if candidate == root {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(candidate, root+sep)
}

// IsValidFilename checks a single filename (no separators) against the vault
// naming convention: lowercase ASCII kebab-case + ".md" suffix, with Windows
// reserved device names rejected to keep vaults portable across platforms.
func IsValidFilename(name string) error {
	if name == "" {
		return fmt.Errorf("%w: filename is empty", ErrInvalidFilename)
	}
	if strings.ContainsRune(name, '/') || strings.ContainsRune(name, '\\') {
		return fmt.Errorf("%w: filename must not contain path separators: %q", ErrInvalidFilename, name)
	}
	if !filenamePattern.MatchString(name) {
		return fmt.Errorf("%w: %q must match ^[a-z0-9][a-z0-9-]*\\.md$", ErrInvalidFilename, name)
	}
	stem := strings.TrimSuffix(name, ".md")
	if _, reserved := windowsReserved[strings.ToLower(stem)]; reserved {
		return fmt.Errorf("%w: %q is a Windows reserved device name", ErrInvalidFilename, name)
	}
	return nil
}
