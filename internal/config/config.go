package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Maintenance struct {
	MaxRecoveryPasses int `yaml:"max_recovery_passes" json:"max_recovery_passes"`
}

type Config struct {
	SchemaVersion      int         `yaml:"schema_version" json:"schema_version"`
	Initialized        bool        `yaml:"initialized" json:"initialized"`
	WikiPath           string      `yaml:"wiki_path" json:"wiki_path"`
	StatePath          string      `yaml:"state_path" json:"state_path"`
	ContextBudgetBytes int         `yaml:"context_budget_bytes" json:"context_budget_bytes"`
	IndexEntryLimit    int         `yaml:"index_entry_limit" json:"index_entry_limit"`
	LockWaitSeconds    int         `yaml:"lock_wait_seconds" json:"lock_wait_seconds"`
	SyncLeaseSeconds   int         `yaml:"sync_lease_seconds" json:"sync_lease_seconds"`
	Maintenance        Maintenance `yaml:"maintenance" json:"maintenance"`
}

func Default() Config {
	return Config{
		SchemaVersion:      1,
		WikiPath:           "docs/llm-wiki",
		StatePath:          ".llm-wiki-state",
		ContextBudgetBytes: 12 * 1024,
		IndexEntryLimit:    200,
		LockWaitSeconds:    5,
		SyncLeaseSeconds:   600,
		Maintenance:        Maintenance{MaxRecoveryPasses: 1},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	info, err := os.Lstat(path)
	if err != nil {
		return Config{}, err
	}
	if !info.Mode().IsRegular() {
		return Config{}, errors.New("llm-wiki config must be a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, err
	}
	for _, key := range []string{"schema_version", "initialized", "wiki_path", "state_path"} {
		if _, ok := raw[key]; !ok {
			return Config{}, errors.New("missing required config key: " + key)
		}
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}
	if cfg.SchemaVersion != 1 {
		return Config{}, errors.New("unsupported schema_version")
	}
	if err := validateRelative(cfg.WikiPath); err != nil {
		return Config{}, err
	}
	if err := validateRelative(cfg.StatePath); err != nil {
		return Config{}, err
	}
	cfg.WikiPath = filepath.ToSlash(filepath.Clean(cfg.WikiPath))
	cfg.StatePath = filepath.ToSlash(filepath.Clean(cfg.StatePath))
	if pathsOverlap(cfg.WikiPath, cfg.StatePath) {
		return Config{}, errors.New("wiki_path and state_path must not overlap")
	}
	if cfg.ContextBudgetBytes < 1024 ||
		cfg.IndexEntryLimit < 10 ||
		cfg.LockWaitSeconds < 0 || cfg.LockWaitSeconds > 60 ||
		cfg.SyncLeaseSeconds < 30 || cfg.SyncLeaseSeconds > 3600 ||
		cfg.Maintenance.MaxRecoveryPasses < 0 || cfg.Maintenance.MaxRecoveryPasses > 3 {
		return Config{}, errors.New("config value is outside the supported range")
	}
	return cfg, nil
}

var safeRepoPath = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

func validateRelative(path string) error {
	clean := filepath.Clean(path)
	slash := filepath.ToSlash(clean)
	if filepath.IsAbs(clean) || clean == "." || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) ||
		slash == ".git" || strings.HasPrefix(slash, ".git/") ||
		!safeRepoPath.MatchString(slash) {
		return errors.New("path must stay inside repository")
	}
	return nil
}

func pathsOverlap(first, second string) bool {
	a := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(first)), "/")
	b := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(second)), "/")
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
