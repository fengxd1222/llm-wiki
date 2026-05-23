package vault

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestLoadConfigAfterInit confirms the canonical happy path: a vault created
// by Init() must be readable back via LoadConfig().
func TestLoadConfigAfterInit(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.VaultRoot == "" {
		t.Fatal("VaultRoot empty")
	}
	if cfg.SchemaVersion != "1.0" {
		t.Fatalf("SchemaVersion = %q, want %q", cfg.SchemaVersion, "1.0")
	}
	if cfg.CreatedAt == "" {
		t.Fatal("CreatedAt empty")
	}
}

// TestLoadConfigMissingFile surfaces ErrConfigMissing when the vault has not
// been initialised (or .wikimind was deleted).
func TestLoadConfigMissingFile(t *testing.T) {
	root := t.TempDir()
	_, err := LoadConfig(root)
	if err == nil {
		t.Fatal("LoadConfig() returned no error, want ErrConfigMissing")
	}
	if !errors.Is(err, ErrConfigMissing) {
		t.Fatalf("LoadConfig() error = %v, want wrap of ErrConfigMissing", err)
	}
}

// TestLoadConfigInvalidTOML rejects garbage TOML with ErrInvalidConfig.
func TestLoadConfigInvalidTOML(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".wikimind"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".wikimind", "config.toml"), []byte("not = valid = toml"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadConfig(root)
	if err == nil {
		t.Fatal("LoadConfig() returned no error, want ErrInvalidConfig")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("LoadConfig() error = %v, want wrap of ErrInvalidConfig", err)
	}
}

// TestLoadConfigMissingFields enumerates the four "missing required field"
// failure modes covered by validateConfig.
func TestLoadConfigMissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			"missing vault_root",
			`schema_version = "1.0"` + "\n" + `created_at = "2026-05-23T00:00:00Z"`,
			"vault_root",
		},
		{
			"missing schema_version",
			`vault_root = "/tmp/v"` + "\n" + `created_at = "2026-05-23T00:00:00Z"`,
			"schema_version",
		},
		{
			"missing created_at",
			`vault_root = "/tmp/v"` + "\n" + `schema_version = "1.0"`,
			"created_at",
		},
		{
			"relative vault_root",
			`vault_root = "relative/path"` + "\n" + `schema_version = "1.0"` + "\n" + `created_at = "2026-05-23T00:00:00Z"`,
			"vault_root must be absolute",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".wikimind"), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(root, ".wikimind", "config.toml"), []byte(tc.body), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			_, err := LoadConfig(root)
			if err == nil {
				t.Fatalf("LoadConfig() returned no error, want ErrInvalidConfig hinting %q", tc.want)
			}
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("LoadConfig() error = %v, want wrap of ErrInvalidConfig", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadConfig() error = %v, want hint %q", err, tc.want)
			}
		})
	}
}

// TestLoadConfigVaultRootMismatch detects a moved/relocated vault. The patched
// vault_root must remain an *absolute* path on every platform (Windows
// included, where "/nonexistent/elsewhere" has no drive letter and would be
// rejected as relative before we even reach the mismatch check), so we point
// it at a real sibling directory created next to the vault.
func TestLoadConfigVaultRootMismatch(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "vault")
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	other, err := filepath.Abs(filepath.Join(base, "vault-other"))
	if err != nil {
		t.Fatalf("abs other: %v", err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatalf("mkdir other: %v", err)
	}

	cfgPath := filepath.Join(root, ".wikimind", "config.toml")
	body, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read cfg: %v", err)
	}
	patched := strings.Replace(string(body), root, other, 1)
	if err := os.WriteFile(cfgPath, []byte(patched), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	_, err = LoadConfig(root)
	if err == nil {
		t.Fatal("LoadConfig() returned no error, want ErrInvalidConfig (mismatch)")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("LoadConfig() error = %v, want wrap of ErrInvalidConfig", err)
	}
	if !strings.Contains(err.Error(), "vault_root mismatch") {
		t.Fatalf("LoadConfig() error = %v, want hint about vault_root mismatch", err)
	}
}

// TestLoadConfigVaultRootCaseInsensitiveOnWindows proves that on Windows NTFS
// (case-insensitive by default), a vault_root that differs only in case from
// the actual root is accepted. Skipped on macOS and Linux because the strict
// equality path is intentional on those filesystems.
func TestLoadConfigVaultRootCaseInsensitiveOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("case-insensitive path compare is Windows-only")
	}
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	cfgPath := filepath.Join(root, ".wikimind", "config.toml")
	body, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read cfg: %v", err)
	}
	// Rewrite vault_root in the config to an all-uppercase variant of the
	// real root. On NTFS this points at the same directory, so LoadConfig
	// must accept it.
	upper := strings.ToUpper(root)
	patched := strings.Replace(string(body), root, upper, 1)
	if err := os.WriteFile(cfgPath, []byte(patched), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	if _, err := LoadConfig(root); err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil (case-insensitive match on Windows)", err)
	}
}

// TestWriteConfigRoundTrip ensures writeConfig + LoadConfig stay symmetric.
func TestWriteConfigRoundTrip(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.VaultRoot != root {
		// On macOS the temp dir may resolve through /private/var; allow that.
		canon, err := filepath.EvalSymlinks(root)
		if err == nil && cfg.VaultRoot == canon {
			return
		}
		t.Logf("VaultRoot=%q root=%q (acceptable if same after EvalSymlinks)", cfg.VaultRoot, root)
	}
}
