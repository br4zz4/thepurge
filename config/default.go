package config

import "runtime"

// Default returns the baseline configuration shipped by `thepurge config init`
// and used when no config exists, for the running platform.
func Default() *Config { return DefaultFor(runtime.GOOS) }

// DefaultFor returns the baseline configuration for a specific GOOS. Targets are
// per-platform: macOS uses ~/Library, Linux uses the XDG dirs. It ships only
// caches and logs — no app folders. App-touching targets (volatile_apps,
// orphan_apps) are empty and opt-in only. Service defaults to dry-run; immutable
// checks and running-app protection are on.
func DefaultFor(goos string) *Config {
	cfg := &Config{
		Storage: Storage{
			Volume:               "/",
			ThresholdType:        "percent",
			ThresholdValue:       15,
			SafetyMarginPercent:  5,
			CheckIntervalMinutes: 30,
			PollIntervalSeconds:  60,
			MaxBytesPerRun:       "",
		},
		ScopeV1: ScopeV1{
			DeleteTypes:   []string{"caches", "logs"},
			PriorityTypes: []string{"logs", "caches"},
		},
		Protection: Protection{RespectRunningApps: true},
		Immutable:  Immutable{CheckImmutable: true},
		Logging: Logging{
			LogPath:         "~/Library/Logs/thepurge/thepurge.log",
			NotifyOnFailure: true,
		},
		Retention: Retention{MinAgeDays: 7},
		Docker:    Docker{Enabled: true, DanglingImages: true, BuilderCache: true},
	}

	if goos == "linux" {
		cfg.Logging.LogPath = "~/.local/state/thepurge/thepurge.log"
		cfg.EvictionTargets = linuxTargets()
		return cfg
	}

	cfg.EvictionTargets = darwinTargets()
	return cfg
}

func darwinTargets() EvictionTargets {
	return EvictionTargets{
		SafeCaches: []Target{
			{Path: "~/Library/Application Support/Claude", Type: "cache", Subfolders: []string{"Cache", "Code Cache", "GPUCache"}},
			{Path: "~/Library/Application Support/Arc", Type: "cache", Pattern: "*Cache*"},
			{Path: "~/Library/Application Support/Google/Chrome", Type: "cache", Pattern: "*Cache*"},
			{Path: "~/Library/Application Support/stremio-server", Type: "cache", Subfolders: []string{"stremio-cache"}},
			{Path: "~/Library/Containers/com.tinyspeck.slackmacgap", Type: "cache", Subfolders: []string{"Data/Library/Caches", "Data/Library/Application Support/Slack/Cache"}},
			{Path: "~/Library/Containers/com.microsoft.teams2", Type: "cache", Subfolders: []string{"Data/Library/Caches", "Data/Library/Application Support/Microsoft/Teams/Cache"}},
			{Path: "~/Library/Application Support/com.apple.wallpaper", Type: "cache", Subfolders: []string{"Aerials"}},
		},
		Logs: []Target{
			{Path: "~/Library/Logs", Type: "log", Pattern: "*"},
			{Path: "~/Library/Containers/com.tinyspeck.slackmacgap/Data/Library/Logs", Type: "log", Pattern: "*"},
		},
		VolatileApps:      []Target{},
		OrphanApps:        []string{},
		EnabledOrphanApps: []string{},
	}
}

func linuxTargets() EvictionTargets {
	return EvictionTargets{
		SafeCaches: []Target{
			{Path: "~/.cache", Type: "cache", Pattern: "*"},
			{Path: "~/.npm/_cacache", Type: "cache"},
			{Path: "~/.config/Slack/Cache", Type: "cache"},
			{Path: "~/.config/Slack/Code Cache", Type: "cache"},
			{Path: "~/.config/Code/Cache", Type: "cache"},
			{Path: "~/.config/Code/CachedData", Type: "cache"},
			{Path: "~/.config/discord/Cache", Type: "cache"},
		},
		Logs: []Target{
			{Path: "~/.npm/_logs", Type: "log"},
			{Path: "~/.config/Code/logs", Type: "log"},
		},
		VolatileApps:      []Target{},
		OrphanApps:        []string{},
		EnabledOrphanApps: []string{},
	}
}
