package config

import (
	"slices"
	"strings"
	"testing"
)

func TestLoadSpecFixture(t *testing.T) {
	cfg, err := Load("testdata/config.yml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// storage
	if cfg.Storage.Volume != "/" {
		t.Errorf("Storage.Volume = %q, want %q", cfg.Storage.Volume, "/")
	}
	if cfg.Storage.ThresholdType != "percent" {
		t.Errorf("Storage.ThresholdType = %q, want %q", cfg.Storage.ThresholdType, "percent")
	}
	if cfg.Storage.ThresholdValue != 15 {
		t.Errorf("Storage.ThresholdValue = %v, want %v", cfg.Storage.ThresholdValue, 15)
	}
	if cfg.Storage.SafetyMarginPercent != 5 {
		t.Errorf("Storage.SafetyMarginPercent = %v, want %v", cfg.Storage.SafetyMarginPercent, 5)
	}
	if cfg.Storage.CheckIntervalMinutes != 30 {
		t.Errorf("Storage.CheckIntervalMinutes = %v, want %v", cfg.Storage.CheckIntervalMinutes, 30)
	}
	if cfg.Storage.PollIntervalSeconds != 60 {
		t.Errorf("Storage.PollIntervalSeconds = %v, want %v", cfg.Storage.PollIntervalSeconds, 60)
	}
	if cfg.Storage.MaxBytesPerRun != "10GB" {
		t.Errorf("Storage.MaxBytesPerRun = %q, want %q", cfg.Storage.MaxBytesPerRun, "10GB")
	}

	// scope_v1
	if !slices.Equal(cfg.ScopeV1.DeleteTypes, []string{"caches", "logs"}) {
		t.Errorf("ScopeV1.DeleteTypes = %v", cfg.ScopeV1.DeleteTypes)
	}
	if !slices.Equal(cfg.ScopeV1.PriorityTypes, []string{"logs", "caches"}) {
		t.Errorf("ScopeV1.PriorityTypes = %v", cfg.ScopeV1.PriorityTypes)
	}

	// protection
	if !cfg.Protection.RespectRunningApps {
		t.Error("Protection.RespectRunningApps = false, want true")
	}

	// immutable
	if !cfg.Immutable.CheckImmutable {
		t.Error("Immutable.CheckImmutable = false, want true")
	}

	// logging
	if !strings.Contains(cfg.Logging.LogPath, "thepurge") {
		t.Errorf("Logging.LogPath = %q", cfg.Logging.LogPath)
	}
	if !cfg.Logging.NotifyOnFailure {
		t.Error("Logging.NotifyOnFailure = false, want true")
	}

	// eviction_targets.safe_caches
	if len(cfg.EvictionTargets.SafeCaches) != 7 {
		t.Fatalf("SafeCaches len = %d, want 7", len(cfg.EvictionTargets.SafeCaches))
	}
	claude := cfg.EvictionTargets.SafeCaches[0]
	if claude.Path != "~/Library/Application Support/Claude" {
		t.Errorf("SafeCaches[0].Path = %q", claude.Path)
	}
	if claude.Type != "cache" {
		t.Errorf("SafeCaches[0].Type = %q, want cache", claude.Type)
	}
	if slices.Contains(claude.Subfolders, "Local Storage") {
		t.Error("Claude subfolders must NOT contain Local Storage (user state, not cache)")
	}
	if slices.Contains(claude.Subfolders, "IndexedDB") {
		t.Error("Claude subfolders must NOT contain IndexedDB (user state, not cache)")
	}

	// eviction_targets.logs
	if len(cfg.EvictionTargets.Logs) != 2 {
		t.Errorf("Logs len = %d, want 2", len(cfg.EvictionTargets.Logs))
	}

	// eviction_targets.volatile_apps — Edge appears exactly once
	if len(cfg.EvictionTargets.VolatileApps) != 5 {
		t.Fatalf("VolatileApps len = %d, want 5", len(cfg.EvictionTargets.VolatileApps))
	}
	edgeCount := 0
	for _, v := range cfg.EvictionTargets.VolatileApps {
		if v.BundleID == "com.microsoft.edgemac" {
			edgeCount++
		}
	}
	if edgeCount != 1 {
		t.Errorf("Microsoft Edge appears %d time(s) in volatile_apps, want 1", edgeCount)
	}

	// eviction_targets.orphan_apps
	if len(cfg.EvictionTargets.OrphanApps) != 6 {
		t.Errorf("OrphanApps len = %d, want 6", len(cfg.EvictionTargets.OrphanApps))
	}
	if len(cfg.EvictionTargets.EnabledOrphanApps) != 0 {
		t.Errorf("EnabledOrphanApps len = %d, want 0", len(cfg.EvictionTargets.EnabledOrphanApps))
	}

	// retention
	if cfg.Retention.MinAgeDays != 7 {
		t.Errorf("Retention.MinAgeDays = %v, want 7", cfg.Retention.MinAgeDays)
	}
}
