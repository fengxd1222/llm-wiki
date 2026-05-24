package vault

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/fengxd1222/llm-wiki/internal/schema"
)

// ErrConfigMissing is returned when .wikimind/config.toml is absent or unreadable.
var ErrConfigMissing = errors.New("vault config is missing")

// ErrInvalidConfig is returned when .wikimind/config.toml fails validation.
var ErrInvalidConfig = errors.New("vault config is invalid")

// Config mirrors .wikimind/config.toml.
type Config struct {
	VaultRoot     string   `toml:"vault_root"`
	SchemaVersion string   `toml:"schema_version"`
	CreatedAt     string   `toml:"created_at"`
	AllowedAgents []string `toml:"allowed_agents"`
}

// DefaultAllowedAgents returns the D10 default agent whitelist.
func DefaultAllowedAgents() []string {
	return []string{"claude-code", "codex-cli", "cursor", "cline", "opencode"}
}

// LoadConfig reads and validates the vault config rooted at vaultRoot.
//
// vaultRoot must be the path returned by FindRoot (i.e. the directory that
// contains .wikimind/config.toml). LoadConfig cross-validates the on-disk
// vault_root against vaultRoot so that a moved or relocated vault surfaces
// as a clear ErrInvalidConfig.
func LoadConfig(vaultRoot string) (*Config, error) {
	absRoot, err := filepath.Abs(vaultRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve vault root: %w", err)
	}
	configPath := filepath.Join(absRoot, ".wikimind", "config.toml")

	body, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrConfigMissing, configPath)
		}
		return nil, fmt.Errorf("read vault config: %w", err)
	}

	cfg := &Config{}
	if _, err := toml.NewDecoder(bytes.NewReader(body)).Decode(cfg); err != nil {
		return nil, fmt.Errorf("%w: parse %s: %v", ErrInvalidConfig, configPath, err)
	}

	if err := validateConfig(cfg, absRoot); err != nil {
		return nil, err
	}
	return cfg, nil
}

func validateConfig(cfg *Config, vaultRoot string) error {
	if cfg.VaultRoot == "" {
		return fmt.Errorf("%w: missing field vault_root", ErrInvalidConfig)
	}
	if cfg.SchemaVersion == "" {
		return fmt.Errorf("%w: missing field schema_version", ErrInvalidConfig)
	}
	if cfg.CreatedAt == "" {
		return fmt.Errorf("%w: missing field created_at", ErrInvalidConfig)
	}

	if !filepath.IsAbs(cfg.VaultRoot) {
		return fmt.Errorf("%w: vault_root must be absolute, got %q", ErrInvalidConfig, cfg.VaultRoot)
	}

	declared, err := filepath.EvalSymlinks(cfg.VaultRoot)
	if err != nil {
		declared = filepath.Clean(cfg.VaultRoot)
	}
	actual, err := filepath.EvalSymlinks(vaultRoot)
	if err != nil {
		actual = filepath.Clean(vaultRoot)
	}
	if !pathsEqual(declared, actual) {
		return fmt.Errorf("%w: vault_root mismatch: config=%q actual=%q", ErrInvalidConfig, cfg.VaultRoot, vaultRoot)
	}
	return nil
}

// pathsEqual compares two filesystem paths with platform-appropriate case
// sensitivity. Windows NTFS is case-insensitive by default; macOS APFS and
// Linux ext4 may be either, but we keep strict equality there to match D2
// behavior and avoid false positives on case-sensitive filesystems.
func pathsEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func writeConfig(root string, createdAt time.Time) error {
	cfg := Config{
		VaultRoot:     root,
		SchemaVersion: schema.Version,
		CreatedAt:     createdAt.Format(time.RFC3339),
		AllowedAgents: DefaultAllowedAgents(),
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return fmt.Errorf("encode .wikimind/config.toml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".wikimind", "config.toml"), buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write .wikimind/config.toml: %w", err)
	}
	return nil
}

func readSchemaVersion(vaultRoot string) (string, error) {
	cfg, err := LoadConfig(vaultRoot)
	if err != nil {
		return "", err
	}
	return cfg.SchemaVersion, nil
}
