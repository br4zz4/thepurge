package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"br4zz4/thepurge/config"
)

// configPath returns the config file to load. It honours THEPURGE_CONFIG and
// falls back to ~/.config/thepurge/config.yml. A missing file is not an error
// here; callers decide how to handle it.
func configPath() (string, error) {
	if p := os.Getenv("THEPURGE_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "thepurge", "config.yml"), nil
}

// loadConfig loads the config, returning a helpful error when absent or invalid.
func loadConfig() (*config.Config, string, error) {
	path, err := configPath()
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, path, fmt.Errorf("config not found at %s (run `thepurge config init`)", path)
		}
		return nil, path, fmt.Errorf("invalid config at %s: %v", path, err)
	}
	return cfg, path, nil
}
