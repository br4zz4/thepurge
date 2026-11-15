package cli

import (
	"fmt"
	"strings"

	"br4zz4/thepurge/config"
)

// renderConfig prints a formatted digest of the active configuration.
func renderConfig(cfg *config.Config) string {
	var b strings.Builder

	fmt.Fprintln(&b, "storage:")
	fmt.Fprintf(&b, "  volume:          %s\n", orDefault(cfg.Storage.Volume, "/"))
	fmt.Fprintf(&b, "  threshold:       %v%s\n", cfg.Storage.ThresholdValue, percentOrSize(cfg.Storage.ThresholdType))
	fmt.Fprintf(&b, "  safety margin:   %d%%\n", cfg.Storage.SafetyMarginPercent)
	fmt.Fprintf(&b, "  check interval:  %d min\n", cfg.Storage.CheckIntervalMinutes)
	fmt.Fprintf(&b, "  poll interval:   %ds\n", cfg.Storage.PollIntervalSeconds)
	fmt.Fprintf(&b, "  max per run:     %s\n", orDefault(cfg.Storage.MaxBytesPerRun, "unlimited"))

	fmt.Fprintln(&b, "eviction:")
	fmt.Fprintf(&b, "  caches:  %d target(s)\n", len(cfg.EvictionTargets.SafeCaches))
	fmt.Fprintf(&b, "  logs:    %d target(s)\n", len(cfg.EvictionTargets.Logs))
	fmt.Fprintf(&b, "  volatile: %d target(s)\n", len(cfg.EvictionTargets.VolatileApps))
	fmt.Fprintf(&b, "  orphans enabled: %d\n", len(cfg.EvictionTargets.EnabledOrphanApps))

	fmt.Fprintln(&b, "safety:")
	fmt.Fprintf(&b, "  respect running apps: %v\n", cfg.Protection.RespectRunningApps)
	fmt.Fprintf(&b, "  check immutable:      %v\n", cfg.Immutable.CheckImmutable)
	fmt.Fprintf(&b, "  retention (min age):  %d days\n", cfg.Retention.MinAgeDays)

	fmt.Fprintln(&b, "docker:")
	fmt.Fprintf(&b, "  enabled:         %v\n", cfg.Docker.Enabled)
	fmt.Fprintf(&b, "  dangling images: %v\n", cfg.Docker.DanglingImages)
	fmt.Fprintf(&b, "  builder cache:   %v\n", cfg.Docker.BuilderCache)
	fmt.Fprintf(&b, "  unused images:   %v\n", cfg.Docker.UnusedImages)

	fmt.Fprintln(&b, "hooks:")
	fmt.Fprintf(&b, "  pre_eviction:  %d\n", len(cfg.Hooks.PreEviction))
	fmt.Fprintf(&b, "  post_eviction: %d\n", len(cfg.Hooks.PostEviction))
	fmt.Fprintf(&b, "  on_failure:    %d\n", len(cfg.Hooks.OnFailure))

	return b.String()
}

func percentOrSize(t string) string {
	if t == "size" {
		return " bytes"
	}
	return "%"
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
