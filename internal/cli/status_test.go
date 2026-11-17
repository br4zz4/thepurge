package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"br4zz4/thepurge/internal/docker"
)

func statusEnv(t *testing.T, maxBytes string, docker bool) (string, []string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	dir := filepath.Join(home, "App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	var files []string
	for _, name := range []string{"a", "b", "c"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, make([]byte, 2048), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	yaml := fmt.Sprintf(`storage:
  volume: "/"
  threshold_type: "percent"
  threshold_value: 0.001
  max_bytes_per_run: %q
retention:
  min_age_days: 0
docker:
  enabled: %v
  dangling_images: true
  builder_cache: true
eviction_targets:
  safe_caches:
    - path: "~/App"
      type: "cache"
`, maxBytes, docker)
	cfgPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("THEPURGE_CONFIG", cfgPath)
	return home, files
}

func runStatus(t *testing.T) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := Run([]string{"status"}, &out, &errOut); code != 0 {
		t.Fatalf("status exit code = %d; err=%s", code, errOut.String())
	}
	return out.String()
}

func TestStatusShowsPotentialAboveThresholdWithoutDeleting(t *testing.T) {
	// arrange
	_, files := statusEnv(t, "10GB", false)

	// act
	got := runStatus(t)

	// assert
	if !strings.Contains(got, "potential:") || !strings.Contains(got, "total (does not depend on threshold)") {
		t.Errorf("missing potential line; got:\n%s", got)
	}
	if !strings.Contains(got, "files:") || !strings.Contains(got, "(3 files)") {
		t.Errorf("want files line with 3 files; got:\n%s", got)
	}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("status must not delete %s: %v", f, err)
		}
	}
}

func TestStatusPotentialIgnoresMaxBytesPerRun(t *testing.T) {
	// arrange
	statusEnv(t, "1KB", false)

	// act
	got := runStatus(t)

	// assert
	if !strings.Contains(got, "(3 files)") {
		t.Errorf("max_bytes_per_run must not cap the potential; got:\n%s", got)
	}
}

func TestStatusDockerMissingShowsSkipped(t *testing.T) {
	// arrange
	statusEnv(t, "10GB", true)

	// act
	got := runStatus(t)

	// assert
	if !strings.Contains(got, "docker:") || !strings.Contains(got, "skipped (") {
		t.Errorf("want docker skipped line; got:\n%s", got)
	}
}

type fakeStatusDocker struct{}

func (fakeStatusDocker) Probe(context.Context) (docker.Status, error) { return docker.StatusOK, nil }
func (fakeStatusDocker) Reclaimable(context.Context) (docker.Reclaimable, error) {
	return docker.Reclaimable{Images: 2048, BuildCache: 4096}, nil
}
func (fakeStatusDocker) DanglingImageIDs(context.Context) ([]string, error) { return nil, nil }
func (fakeStatusDocker) Prune(context.Context, docker.PruneKind) (docker.PruneResult, error) {
	return docker.PruneResult{}, fmt.Errorf("status must not prune")
}

func TestStatusWithFakeDockerShowsReclaimable(t *testing.T) {
	// arrange
	statusEnv(t, "", true)
	prev := statusDocker
	statusDocker = fakeStatusDocker{}
	t.Cleanup(func() { statusDocker = prev })

	// act
	got := runStatus(t)

	// assert
	if !strings.Contains(got, "docker:") || !strings.Contains(got, "(images ") || !strings.Contains(got, "+ build cache ") {
		t.Errorf("want docker line with images + build cache; got:\n%s", got)
	}
	if strings.Contains(got, "skipped (") {
		t.Errorf("docker must not be skipped with fake; got:\n%s", got)
	}
}
