package config

import "testing"

// TestDefaultConfig guards the initial configuration shipped by `config init`:
// it must be a usable, safe baseline (thresholds set, targets present, apply off).
func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	if cfg.Storage.Volume != "/" {
		t.Errorf("Storage.Volume = %q, want /", cfg.Storage.Volume)
	}
	if cfg.Storage.ThresholdType != "percent" {
		t.Errorf("Storage.ThresholdType = %q, want percent", cfg.Storage.ThresholdType)
	}
	if cfg.Storage.ThresholdValue != 15 {
		t.Errorf("Storage.ThresholdValue = %v, want 15", cfg.Storage.ThresholdValue)
	}
	if cfg.Storage.SafetyMarginPercent != 5 {
		t.Errorf("Storage.SafetyMarginPercent = %v, want 5", cfg.Storage.SafetyMarginPercent)
	}
	if cfg.Storage.CheckIntervalMinutes != 30 {
		t.Errorf("Storage.CheckIntervalMinutes = %v, want 30", cfg.Storage.CheckIntervalMinutes)
	}
	if cfg.Storage.PollIntervalSeconds != 60 {
		t.Errorf("Storage.PollIntervalSeconds = %v, want 60", cfg.Storage.PollIntervalSeconds)
	}
	if cfg.Storage.MaxBytesPerRun != "10GB" {
		t.Errorf("Storage.MaxBytesPerRun = %q, want 10GB", cfg.Storage.MaxBytesPerRun)
	}

	if cfg.Protection.RespectRunningApps != true {
		t.Error("Protection.RespectRunningApps must default to true")
	}
	if cfg.Immutable.CheckImmutable != true {
		t.Error("Immutable.CheckImmutable must default to true")
	}
	if cfg.Retention.MinAgeDays != 7 {
		t.Errorf("Retention.MinAgeDays = %v, want 7", cfg.Retention.MinAgeDays)
	}

	if len(cfg.EvictionTargets.SafeCaches) == 0 {
		t.Error("Default must include safe_caches targets")
	}
	if len(cfg.EvictionTargets.Logs) == 0 {
		t.Error("Default must include logs targets")
	}
	if len(cfg.EvictionTargets.VolatileApps) != 0 {
		t.Error("Default must NOT include volatile_apps (app folders — opt-in only)")
	}
	if len(cfg.EvictionTargets.OrphanApps) != 0 {
		t.Error("Default must NOT include orphan_apps (app folders — opt-in only)")
	}
	if len(cfg.EvictionTargets.EnabledOrphanApps) != 0 {
		t.Error("Default must NOT enable any orphan (opt-in)")
	}
}
