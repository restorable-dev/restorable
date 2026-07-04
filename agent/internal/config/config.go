// Package config loads and validates the agent's YAML configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

// DefaultPasswordEnv is the environment variable restic reads natively.
const DefaultPasswordEnv = "RESTIC_PASSWORD"

// Config is the agent.yaml schema.
type Config struct {
	// Repo is the restic repository (any restic-supported backend string).
	Repo string `yaml:"repo"`
	// PasswordEnv names the environment variable holding the repository
	// password. Defaults to RESTIC_PASSWORD. The password itself never
	// appears in config files.
	PasswordEnv string `yaml:"password_env"`
	// Schedule is a cron expression used by `restorable run`.
	Schedule string `yaml:"schedule"`
	// Recipes are paths to recipe YAML files, resolved relative to the
	// config file's directory.
	Recipes []string `yaml:"recipes"`
	// Sandbox configures where restore sandboxes are created.
	Sandbox SandboxConfig `yaml:"sandbox"`
}

// SandboxConfig controls sandbox placement and the disk-space pre-flight.
type SandboxConfig struct {
	// Dir is the base directory for sandboxes. Empty means the OS temp dir.
	Dir string `yaml:"dir"`
	// MinFreeSpace is a human-readable size ("2GiB", "500MB") that must be
	// free in Dir before a restore starts, in addition to the estimated
	// snapshot size.
	MinFreeSpace string `yaml:"min_free_space"`
}

var envNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Load reads, defaults, and validates a config file. Recipe paths are
// resolved relative to the config file's directory.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only file

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("config file %s is empty", path)
		}
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.PasswordEnv == "" {
		cfg.PasswordEnv = DefaultPasswordEnv
	}
	base := filepath.Dir(path)
	for i, r := range cfg.Recipes {
		if !filepath.IsAbs(r) {
			cfg.Recipes[i] = filepath.Join(base, r)
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Repo == "" {
		return errors.New("repo is required")
	}
	if !envNameRe.MatchString(c.PasswordEnv) {
		return fmt.Errorf("password_env %q is not a valid environment variable name", c.PasswordEnv)
	}
	if c.Schedule != "" {
		if _, err := cron.ParseStandard(c.Schedule); err != nil {
			return fmt.Errorf("invalid schedule %q: %w", c.Schedule, err)
		}
	}
	if c.Sandbox.MinFreeSpace != "" {
		if _, err := ParseSize(c.Sandbox.MinFreeSpace); err != nil {
			return fmt.Errorf("invalid sandbox.min_free_space: %w", err)
		}
	}
	for _, r := range c.Recipes {
		if _, err := os.Stat(r); err != nil {
			return fmt.Errorf("recipe file %s: %w", r, err)
		}
	}
	return nil
}

// MinFreeBytes returns sandbox.min_free_space in bytes (0 when unset).
func (c *Config) MinFreeBytes() uint64 {
	if c.Sandbox.MinFreeSpace == "" {
		return 0
	}
	n, err := ParseSize(c.Sandbox.MinFreeSpace)
	if err != nil {
		// validate() already rejected unparseable values at load time.
		return 0
	}
	return n
}

var sizeRe = regexp.MustCompile(`(?i)^\s*(\d+(?:\.\d+)?)\s*(b|kb|mb|gb|tb|kib|mib|gib|tib)?\s*$`)

// ParseSize parses human-readable sizes like "500MB" or "2GiB" into bytes.
// Decimal units (KB, MB, ...) are powers of 1000; binary units (KiB, MiB,
// ...) are powers of 1024. A bare number means bytes.
func ParseSize(s string) (uint64, error) {
	m := sizeRe.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("cannot parse size %q (examples: 500MB, 2GiB, 1048576)", s)
	}
	value, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse size %q: %w", s, err)
	}
	var mult float64
	switch strings.ToLower(m[2]) {
	case "", "b":
		mult = 1
	case "kb":
		mult = 1e3
	case "mb":
		mult = 1e6
	case "gb":
		mult = 1e9
	case "tb":
		mult = 1e12
	case "kib":
		mult = 1 << 10
	case "mib":
		mult = 1 << 20
	case "gib":
		mult = 1 << 30
	case "tib":
		mult = 1 << 40
	}
	bytes := value * mult
	if bytes < 0 || bytes > math.MaxUint64 {
		return 0, fmt.Errorf("size %q out of range", s)
	}
	return uint64(bytes), nil
}
