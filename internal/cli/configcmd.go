package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"br4zz4/thepurge/config"
)

// cmdConfig shows the formatted config summary, the colored YAML file, or runs
// config subcommands (init|validate|orphans.enable|orphans.disable).
func cmdConfig(args []string, out, errOut io.Writer) (int, error) {
	if len(args) == 0 {
		cfg, _, err := loadConfig()
		if err != nil {
			return 1, err
		}
		fmt.Fprint(out, renderConfig(cfg))
		return 0, nil
	}

	switch args[0] {
	case "--help", "-h", "help":
		printConfigUsage(out)
		return 0, nil
	case "--file":
		return configFile(out, errOut)
	case "init":
		return configInit(out)
	case "validate":
		cfg, path, err := loadConfig()
		if err != nil {
			return 1, err
		}
		if err := config.Validate(cfg); err != nil {
			return 1, err
		}
		fmt.Fprintf(out, "config valid: %s\n", path)
		fmt.Fprintf(out, "  targets resolved: %d\n", countTargets(cfg))
		return 0, nil
	case "orphans.enable", "orphans.disable":
		return configOrphan(args[0] == "orphans.enable", args[1:], out, errOut)
	case "get":
		return configGet(args[1:], out, errOut)
	case "set":
		return configSet(args[1:], out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown config subcommand %q\n", args[0])
		printConfigUsage(errOut)
		return 2, nil
	}
}

func printConfigUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: thepurge config init|get|set|validate|orphans.enable|orphans.disable")
	fmt.Fprintln(w, "       thepurge config            show formatted config summary")
	fmt.Fprintln(w, "       thepurge config --file     show colored YAML of config.yml")
	fmt.Fprintln(w, "       thepurge config --help     this help")
}

// configFile prints the raw config file with ANSI-colored YAML.
func configFile(out, errOut io.Writer) (int, error) {
	path, err := configPath()
	if err != nil {
		return 1, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, fmt.Errorf("config not found at %s (run `thepurge config init`)", path)
		}
		return 1, err
	}
	fmt.Fprint(out, colorYAML(string(b)))
	return 0, nil
}

// configInit writes a default config to the standard path if it does not exist.
func configInit(out io.Writer) (int, error) {
	path, err := configPath()
	if err != nil {
		return 1, err
	}
	if _, err := os.Stat(path); err == nil {
		return 1, fmt.Errorf("config already exists at %s", path)
	}
	var cfg = config.Default()
	data, err := config.Marshal(cfg)
	if err != nil {
		return 1, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 1, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return 1, err
	}
	fmt.Fprintf(out, "config written: %s\n", path)
	return 0, nil
}

func configOrphan(enable bool, args []string, out, errOut io.Writer) (int, error) {
	flagMap, positional := splitFlags(args)
	if !flagMap["--path"] || len(positional) == 0 {
		fmt.Fprintln(errOut, "usage: thepurge config orphans.enable --path <PATH>")
		return 2, nil
	}
	path, err := configPath()
	if err != nil {
		return 1, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, fmt.Errorf("config not found at %s", path)
		}
		return 1, err
	}
	// positional[0] is the --path value (splitFlags collects non-flag args after --path).
	orphan := positional[len(positional)-1]

	if enable {
		cfg.EvictionTargets.EnabledOrphanApps = appendUnique(cfg.EvictionTargets.EnabledOrphanApps, orphan)
	} else {
		cfg.EvictionTargets.EnabledOrphanApps = removeStr(cfg.EvictionTargets.EnabledOrphanApps, orphan)
	}
	data, err := config.Marshal(cfg)
	if err != nil {
		return 1, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return 1, err
	}
	verb := "enabled"
	if !enable {
		verb = "disabled"
	}
	fmt.Fprintf(out, "orphan %s: %s\n", verb, orphan)
	return 0, nil
}

func configGet(args []string, out, errOut io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: thepurge config get <key>")
		return 2, nil
	}
	cfg, _, err := loadConfig()
	if err != nil {
		return 1, err
	}
	val, err := getConfigValue(cfg, args[0])
	if err != nil {
		fmt.Fprintf(errOut, "config get: %v", err)
		fmt.Fprintln(errOut)
		return 1, nil
	}
	fmt.Fprintf(out, "%s: %s", args[0], val)
	fmt.Fprintln(out)
	return 0, nil
}

func configSet(args []string, out, errOut io.Writer) (int, error) {
	if len(args) < 2 {
		fmt.Fprintln(errOut, "usage: thepurge config set <key> <value>")
		return 2, nil
	}
	key := args[0]
	value := args[1]
	path, err := configPath()
	if err != nil {
		return 1, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, fmt.Errorf("config not found at %s", path)
		}
		return 1, err
	}
	if err := setConfigValue(cfg, key, value); err != nil {
		fmt.Fprintf(errOut, "config set: %v", err)
		fmt.Fprintln(errOut)
		return 1, nil
	}
	data, err := config.Marshal(cfg)
	if err != nil {
		return 1, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return 1, err
	}
	fmt.Fprintf(out, "set %s = %s", key, value)
	fmt.Fprintln(out)
	return 0, nil
}

func getConfigValue(cfg *config.Config, key string) (string, error) {
	switch key {
	case "storage.volume":
		return cfg.Storage.Volume, nil
	case "storage.threshold_type":
		return cfg.Storage.ThresholdType, nil
	case "storage.threshold_value":
		return fmt.Sprintf("%v", cfg.Storage.ThresholdValue), nil
	case "storage.safety_margin_percent":
		return fmt.Sprintf("%d", cfg.Storage.SafetyMarginPercent), nil
	case "storage.check_interval_minutes":
		return fmt.Sprintf("%d", cfg.Storage.CheckIntervalMinutes), nil
	case "storage.poll_interval_seconds":
		return fmt.Sprintf("%d", cfg.Storage.PollIntervalSeconds), nil
	case "storage.max_bytes_per_run":
		return cfg.Storage.MaxBytesPerRun, nil
	case "protection.respect_running_apps":
		return fmt.Sprintf("%v", cfg.Protection.RespectRunningApps), nil
	case "immutable.check_immutable":
		return fmt.Sprintf("%v", cfg.Immutable.CheckImmutable), nil
	case "logging.log_path":
		return cfg.Logging.LogPath, nil
	case "logging.notify_on_failure":
		return fmt.Sprintf("%v", cfg.Logging.NotifyOnFailure), nil
	case "retention.min_age_days":
		return fmt.Sprintf("%d", cfg.Retention.MinAgeDays), nil
	case "docker.enabled":
		return fmt.Sprintf("%v", cfg.Docker.Enabled), nil
	case "docker.dangling_images":
		return fmt.Sprintf("%v", cfg.Docker.DanglingImages), nil
	case "docker.builder_cache":
		return fmt.Sprintf("%v", cfg.Docker.BuilderCache), nil
	case "docker.unused_images":
		return fmt.Sprintf("%v", cfg.Docker.UnusedImages), nil
	default:
		return "", fmt.Errorf("unknown key %q", key)
	}
}

func setConfigValue(cfg *config.Config, key, value string) error {
	switch key {
	case "storage.volume":
		cfg.Storage.Volume = value
	case "storage.threshold_type":
		if value != "percent" && value != "size" {
			return fmt.Errorf("threshold_type must be percent or size, got %q", value)
		}
		cfg.Storage.ThresholdType = value
	case "storage.threshold_value":
		var v float64
		if _, err := fmt.Sscanf(value, "%f", &v); err != nil {
			return fmt.Errorf("invalid threshold_value %q: %v", value, err)
		}
		cfg.Storage.ThresholdValue = v
	case "storage.safety_margin_percent":
		if _, err := fmt.Sscanf(value, "%d", &cfg.Storage.SafetyMarginPercent); err != nil {
			return fmt.Errorf("invalid safety_margin_percent %q", value)
		}
	case "storage.check_interval_minutes":
		if _, err := fmt.Sscanf(value, "%d", &cfg.Storage.CheckIntervalMinutes); err != nil {
			return fmt.Errorf("invalid check_interval_minutes %q", value)
		}
	case "storage.poll_interval_seconds":
		if _, err := fmt.Sscanf(value, "%d", &cfg.Storage.PollIntervalSeconds); err != nil {
			return fmt.Errorf("invalid poll_interval_seconds %q", value)
		}
	case "storage.max_bytes_per_run":
		cfg.Storage.MaxBytesPerRun = value
	case "protection.respect_running_apps":
		cfg.Protection.RespectRunningApps = value == "true"
	case "immutable.check_immutable":
		cfg.Immutable.CheckImmutable = value == "true"
	case "logging.log_path":
		cfg.Logging.LogPath = value
	case "logging.notify_on_failure":
		cfg.Logging.NotifyOnFailure = value == "true"
	case "docker.enabled":
		cfg.Docker.Enabled = value == "true"
	case "docker.dangling_images":
		cfg.Docker.DanglingImages = value == "true"
	case "docker.builder_cache":
		cfg.Docker.BuilderCache = value == "true"
	case "docker.unused_images":
		cfg.Docker.UnusedImages = value == "true"
	case "retention.min_age_days":
		if _, err := fmt.Sscanf(value, "%d", &cfg.Retention.MinAgeDays); err != nil {
			return fmt.Errorf("invalid min_age_days %q", value)
		}
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	return nil
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func removeStr(list []string, v string) []string {
	var out []string
	for _, x := range list {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func countTargets(cfg *config.Config) int {
	return len(cfg.EvictionTargets.SafeCaches) + len(cfg.EvictionTargets.Logs) +
		len(cfg.EvictionTargets.VolatileApps) + len(cfg.EvictionTargets.EnabledOrphanApps)
}
