package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ErrInvalidThreshold is returned when storage.threshold_type or
// storage.threshold_value is not a valid, in-range value.
var ErrInvalidThreshold = errors.New("invalid storage threshold")

type Storage struct {
	Volume               string  `yaml:"volume"`
	ThresholdType        string  `yaml:"threshold_type"`
	ThresholdValue       float64 `yaml:"threshold_value"`
	SafetyMarginPercent  int     `yaml:"safety_margin_percent"`
	CheckIntervalMinutes int     `yaml:"check_interval_minutes"`
	PollIntervalSeconds  int     `yaml:"poll_interval_seconds"`
	MaxBytesPerRun       string  `yaml:"max_bytes_per_run"`
}

type ScopeV1 struct {
	DeleteTypes   []string `yaml:"delete_types"`
	PriorityTypes []string `yaml:"priority_types"`
}

type Protection struct {
	RespectRunningApps bool `yaml:"respect_running_apps"`
}

type Immutable struct {
	CheckImmutable bool `yaml:"check_immutable"`
}

type Logging struct {
	LogPath         string `yaml:"log_path"`
	NotifyOnFailure bool   `yaml:"notify_on_failure"`
}

type Target struct {
	Path       string   `yaml:"path"`
	Type       string   `yaml:"type"`
	Subfolders []string `yaml:"subfolders"`
	Pattern    string   `yaml:"pattern"`
	BundleID   string   `yaml:"bundle_id"`
}

type EvictionTargets struct {
	SafeCaches        []Target `yaml:"safe_caches"`
	Logs              []Target `yaml:"logs"`
	VolatileApps      []Target `yaml:"volatile_apps"`
	OrphanApps        []string `yaml:"orphan_apps"`
	EnabledOrphanApps []string `yaml:"enabled_orphan_apps"`
}

type Retention struct {
	MinAgeDays int `yaml:"min_age_days"`
}

// Docker configures the optional docker cleanup phase.
type Docker struct {
	Enabled        bool `yaml:"enabled"`
	DanglingImages bool `yaml:"dangling_images"`
	BuilderCache   bool `yaml:"builder_cache"`
	UnusedImages   bool `yaml:"unused_images"`
}

// Hook is one user command run around an eviction pass. Exactly one of Path
// or Command must be set.
type Hook struct {
	Path           string `yaml:"path"`
	Command        string `yaml:"command"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

// Hooks groups hooks by event.
type Hooks struct {
	PreEviction  []Hook `yaml:"pre_eviction"`
	PostEviction []Hook `yaml:"post_eviction"`
	OnFailure    []Hook `yaml:"on_failure"`
}

type Config struct {
	Storage         Storage         `yaml:"storage"`
	ScopeV1         ScopeV1         `yaml:"scope_v1"`
	Protection      Protection      `yaml:"protection"`
	Immutable       Immutable       `yaml:"immutable"`
	Logging         Logging         `yaml:"logging"`
	EvictionTargets EvictionTargets `yaml:"eviction_targets"`
	Retention       Retention       `yaml:"retention"`
	Docker          Docker          `yaml:"docker"`
	Hooks           Hooks           `yaml:"hooks"`
}

// Validate checks hook definitions: each needs exactly one of path/command and
// a non-negative timeout.
func Validate(c *Config) error {
	events := []struct {
		name  string
		hooks []Hook
	}{
		{"pre_eviction", c.Hooks.PreEviction},
		{"post_eviction", c.Hooks.PostEviction},
		{"on_failure", c.Hooks.OnFailure},
	}
	for _, ev := range events {
		for i, h := range ev.hooks {
			if (h.Path == "") == (h.Command == "") {
				return fmt.Errorf("hooks.%s[%d]: exactly one of path or command is required", ev.name, i)
			}
			if h.TimeoutSeconds < 0 {
				return fmt.Errorf("hooks.%s[%d]: timeout_seconds must not be negative", ev.name, i)
			}
		}
	}
	return nil
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Marshal serialises a Config back to YAML bytes (used when editing via CLI,
// e.g. enabling/disabling orphans).
func Marshal(c *Config) ([]byte, error) {
	return yaml.Marshal(c)
}
