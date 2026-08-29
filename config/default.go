package config

// Default returns the baseline configuration shipped by `thepurge config init`.
// It mirrors the spec's example: safe, conservative defaults — dry-run service,
// immutable checks on, running-app protection on, and orphans opt-in only.
func Default() *Config {
	return &Config{
		Storage: Storage{
			Volume:               "/",
			ThresholdType:        "percent",
			ThresholdValue:       15,
			SafetyMarginPercent:  5,
			CheckIntervalMinutes: 30,
			PollIntervalSeconds:  60,
			MaxBytesPerRun:       "10GB",
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
		Service: Service{Apply: false},
		EvictionTargets: EvictionTargets{
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
			VolatileApps: []Target{
				{Path: "~/Library/Application Support/Arc", BundleID: "company.thebrowser.Browser"},
				{Path: "~/Library/Application Support/Google/Chrome", BundleID: "com.google.Chrome"},
				{Path: "~/Library/Application Support/stremio-server", BundleID: "com.stremio.stremio"},
				{Path: "~/Library/Application Support/Microsoft Edge", BundleID: "com.microsoft.edgemac"},
				{Path: "~/Library/Application Support/Microsoft Office", BundleID: "com.microsoft.office"},
			},
			OrphanApps: []string{
				"~/Library/Application Support/Cursor",
				"~/Library/Application Support/Spotify",
				"~/Library/Application Support/discord",
				"~/Library/Application Support/Notion Calendar",
				"~/Library/Application Support/com.imobie.AnyTrans",
				"~/Library/Application Support/com.docker.install",
			},
			EnabledOrphanApps: []string{},
		},
		Retention: Retention{MinAgeDays: 7},
	}
}
