package eviction

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"br4zz4/thepurge/config"
	"br4zz4/thepurge/internal/docker"
)

func TestRunDiskUsageErrorPropagates(t *testing.T) {
	// arrange
	cfg := dockerCfg(config.Docker{})
	boom := errors.New("disk gone")
	opts := EngineOptions{HomeDir: t.TempDir(), DiskUsage: func(string) (int64, int64, error) { return 0, 0, boom }}

	// act
	_, err := Run(cfg, opts)

	// assert
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}

func TestRunInvalidThresholdTypeFails(t *testing.T) {
	// arrange
	cfg := dockerCfg(config.Docker{})
	cfg.Storage.ThresholdType = "bogus"
	opts := EngineOptions{HomeDir: t.TempDir(), DiskUsage: fakeDiskUsage(90, 1000)}

	// act
	_, err := Run(cfg, opts)

	// assert
	if !errors.Is(err, config.ErrInvalidThreshold) {
		t.Errorf("err = %v, want ErrInvalidThreshold", err)
	}
}

func TestRunPercentThresholdOutOfRangeFails(t *testing.T) {
	// arrange
	cfg := dockerCfg(config.Docker{})
	cfg.Storage.ThresholdValue = 150
	opts := EngineOptions{HomeDir: t.TempDir(), DiskUsage: fakeDiskUsage(90, 1000)}

	// act
	_, err := Run(cfg, opts)

	// assert
	if !errors.Is(err, config.ErrInvalidThreshold) {
		t.Errorf("err = %v, want ErrInvalidThreshold", err)
	}
}

func TestRunInvalidMaxBytesFails(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	cfg := dockerCfg(config.Docker{})
	cfg.Storage.MaxBytesPerRun = "lots"
	opts := EngineOptions{HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000)}

	// act
	_, err := Run(cfg, opts)

	// assert
	if err == nil {
		t.Error("err = nil, want invalid size error")
	}
}

func TestRunSizeThresholdUsesAbsoluteBytes(t *testing.T) {
	// arrange
	home, file := dockerHome(t)
	cfg := dockerCfg(config.Docker{})
	cfg.Storage.ThresholdType = "size"
	cfg.Storage.ThresholdValue = 500
	cfg.Storage.SafetyMarginPercent = 0
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(400, 1000)}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if res.UnderThreshold || res.FreedBytes != 100 {
		t.Errorf("UnderThreshold=%v FreedBytes=%d", res.UnderThreshold, res.FreedBytes)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("file should be deleted: %v", err)
	}
}

func TestRunDoesNotRunDockerAboveThreshold(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(900, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !res.UnderThreshold || res.Docker.Ran || len(fd.pruned) != 0 || fd.probed != 0 {
		t.Errorf("under=%v ran=%v pruned=%v probed=%d", res.UnderThreshold, res.Docker.Ran, fd.pruned, fd.probed)
	}
}

func TestRunIgnoreThresholdRunsEvenWhenAbove(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, BuilderCache: true})
	opts := EngineOptions{Apply: false, IgnoreThreshold: true, HomeDir: home, DiskUsage: fakeDiskUsage(900, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if res.UnderThreshold || len(res.Candidates) != 1 || !res.Docker.Ran {
		t.Errorf("under=%v cands=%d ran=%v", res.UnderThreshold, len(res.Candidates), res.Docker.Ran)
	}
}

func TestDockerUnusedImagesPrunesAllUnusedOnly(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, UnusedImages: true, BuilderCache: true})
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(fd.pruned, []docker.PruneKind{docker.PruneAllUnused, docker.PruneBuilder}) {
		t.Errorf("pruned = %v", fd.pruned)
	}
	if res.Docker.FreedBytes != 65 {
		t.Errorf("FreedBytes = %d, want 65", res.Docker.FreedBytes)
	}
	if !slices.Equal(res.Docker.Prunes, []string{"image prune -a", "builder prune"}) {
		t.Errorf("Prunes = %v", res.Docker.Prunes)
	}
}

func TestDockerPruneErrorIsRecordedAndOthersContinue(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	fd.pruneErr = map[docker.PruneKind]error{docker.PruneDangling: errors.New("boom")}
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Docker.Errors) != 1 || res.Docker.FreedBytes != 25 {
		t.Errorf("errors=%v freed=%d", res.Docker.Errors, res.Docker.FreedBytes)
	}
}

func TestDockerDryRunDfFailureRecordsError(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	fd.dfErr = errors.New("df broke")
	cfg := dockerCfg(config.Docker{Enabled: true, BuilderCache: true})
	opts := EngineOptions{HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Docker.Errors) != 1 || res.Docker.ReclaimableBytes != 0 || !res.Docker.Ran {
		t.Errorf("errors=%v reclaimable=%d ran=%v", res.Docker.Errors, res.Docker.ReclaimableBytes, res.Docker.Ran)
	}
}

func TestDockerDisabledOrNoKindsDoesNothing(t *testing.T) {
	cases := map[string]config.Docker{
		"disabled": {Enabled: false, DanglingImages: true},
		"no kinds": {Enabled: true},
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			// arrange
			home, _ := dockerHome(t)
			fd := newFakeDocker()
			opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

			// act
			res, err := Run(dockerCfg(d), opts)

			// assert
			if err != nil {
				t.Fatal(err)
			}
			if fd.probed != 0 || res.Docker.Ran {
				t.Errorf("probed=%d ran=%v", fd.probed, res.Docker.Ran)
			}
		})
	}
}

func TestRunOrphanStillInstalledIsSkipped(t *testing.T) {
	// arrange
	home := t.TempDir()
	dir := filepath.Join(home, "Orphan")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := writeFileIn(t, dir, "f", 100, time.Now())
	cfg := dockerCfg(config.Docker{})
	cfg.EvictionTargets = config.EvictionTargets{
		EnabledOrphanApps: []string{"~/Orphan"},
	}
	opts := EngineOptions{
		Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000),
		IsAppInstalled: func(string) (bool, string) { return true, "Orphan.app" },
	}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 || len(res.Skipped) != 1 {
		t.Errorf("cands=%d skipped=%v", len(res.Candidates), res.Skipped)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("file must remain: %v", err)
	}
}

func TestRunRunningAppDetectedByDerivedProcessName(t *testing.T) {
	// arrange
	home, file := dockerHome(t)
	cfg := dockerCfg(config.Docker{})
	cfg.EvictionTargets = config.EvictionTargets{
		VolatileApps: []config.Target{{Path: "~/App", Type: "cache"}},
	}
	var asked []string
	opts := EngineOptions{
		Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000),
		IsProcessRunning: func(_, name string) bool { asked = append(asked, name); return name == "App" },
	}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if len(res.AppSkipped) != 1 || len(res.Candidates) != 0 {
		t.Errorf("AppSkipped=%v cands=%d asked=%v", res.AppSkipped, len(res.Candidates), asked)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("file must remain: %v", err)
	}
}

func TestDeriveProcessNames(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		{"/Users/x/Library/Application Support/Google/Chrome", []string{"Chrome"}},
		{"/Users/x/Library/Application Support/Cursor/Data", []string{"Cursor"}},
		{"~/Library/Containers", nil},
		{"/Users/x/Library/com.foo.bar", []string{"x"}},
		{"", nil},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			// act
			got := deriveProcessNames(tc.path)

			// assert
			if !slices.Equal(got, tc.want) {
				t.Errorf("deriveProcessNames(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
