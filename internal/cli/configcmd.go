package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"br4zz4/thepurge/config"
)

// cmdConfig handles config init|get|set|validate and config orphans.enable|disable.
func cmdConfig(args []string, out, errOut io.Writer) (int, error) {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "usage: thepurge config init|get|set|validate|orphans.enable|orphans.disable")
		return 2, nil
	}

	switch args[0] {
	case "init":
		return configInit(out)
	case "validate":
		cfg, path, err := loadConfig()
		if err != nil {
			return 1, err
		}
		fmt.Fprintf(out, "config valid: %s\n", path)
		fmt.Fprintf(out, "  targets resolved: %d\n", countTargets(cfg))
		return 0, nil
	case "orphans.enable", "orphans.disable":
		return configOrphan(args[0] == "orphans.enable", args[1:], out, errOut)
	case "get":
		fmt.Fprintln(errOut, "thepurge config get: not implemented yet")
		return 1, nil
	case "set":
		fmt.Fprintln(errOut, "thepurge config set: not implemented yet")
		return 1, nil
	default:
		fmt.Fprintf(errOut, "unknown config subcommand %q\n", args[0])
		return 2, nil
	}
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
