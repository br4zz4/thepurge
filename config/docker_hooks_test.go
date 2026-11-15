package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDockerEnabledAndHooksEmpty(t *testing.T) {
	// arrange
	// act
	cfg := Default()

	// assert
	want := Docker{Enabled: true, DanglingImages: true, BuilderCache: true, UnusedImages: false}
	if cfg.Docker != want {
		t.Errorf("Docker = %+v, want %+v", cfg.Docker, want)
	}
	if len(cfg.Hooks.PreEviction)+len(cfg.Hooks.PostEviction)+len(cfg.Hooks.OnFailure) != 0 {
		t.Errorf("Hooks must be empty by default, got %+v", cfg.Hooks)
	}
}

func TestDockerHooksRoundTrip(t *testing.T) {
	// arrange
	cfg := Default()
	cfg.Docker.UnusedImages = true
	cfg.Hooks.PreEviction = []Hook{{Command: "echo pre", TimeoutSeconds: 5}}
	cfg.Hooks.PostEviction = []Hook{{Path: "/tmp/post.sh"}}
	cfg.Hooks.OnFailure = []Hook{{Command: "echo fail"}}
	data, err := Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// act
	got, err := Load(p)

	// assert
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Docker != cfg.Docker {
		t.Errorf("Docker = %+v, want %+v", got.Docker, cfg.Docker)
	}
	if len(got.Hooks.PreEviction) != 1 || got.Hooks.PreEviction[0] != cfg.Hooks.PreEviction[0] {
		t.Errorf("PreEviction = %+v", got.Hooks.PreEviction)
	}
	if len(got.Hooks.PostEviction) != 1 || got.Hooks.PostEviction[0] != cfg.Hooks.PostEviction[0] {
		t.Errorf("PostEviction = %+v", got.Hooks.PostEviction)
	}
	if len(got.Hooks.OnFailure) != 1 || got.Hooks.OnFailure[0] != cfg.Hooks.OnFailure[0] {
		t.Errorf("OnFailure = %+v", got.Hooks.OnFailure)
	}
}

func TestMarshalUsesDockerYAMLKeys(t *testing.T) {
	// arrange
	cfg := Default()

	// act
	data, err := Marshal(cfg)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, key := range []string{"docker:", "enabled: true", "dangling_images: true", "builder_cache: true", "unused_images: false"} {
		if !strings.Contains(s, key) {
			t.Errorf("marshaled config missing %q; got:\n%s", key, s)
		}
	}
}

func TestValidateHooks(t *testing.T) {
	cases := []struct {
		name  string
		hooks Hooks
		ok    bool
	}{
		{"empty", Hooks{}, true},
		{"path only", Hooks{PreEviction: []Hook{{Path: "/bin/true"}}}, true},
		{"command only", Hooks{PostEviction: []Hook{{Command: "true"}}}, true},
		{"path and command", Hooks{PreEviction: []Hook{{Path: "/bin/true", Command: "true"}}}, false},
		{"neither", Hooks{PostEviction: []Hook{{}}}, false},
		{"neither on_failure", Hooks{OnFailure: []Hook{{TimeoutSeconds: 3}}}, false},
		{"negative timeout", Hooks{PreEviction: []Hook{{Command: "true", TimeoutSeconds: -1}}}, false},
		{"valid after invalid", Hooks{PreEviction: []Hook{{Command: "true"}, {Path: "a", Command: "b"}}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// arrange
			cfg := Default()
			cfg.Hooks = c.hooks

			// act
			err := Validate(cfg)

			// assert
			if c.ok && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
			if !c.ok && err == nil {
				t.Error("Validate() expected error, got nil")
			}
		})
	}
}
