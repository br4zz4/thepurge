package eviction

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"br4zz4/thepurge/config"
	"br4zz4/thepurge/internal/docker"
)

type fakeDocker struct {
	status      docker.Status
	reclaimable docker.Reclaimable
	dangling    []string
	freed       map[docker.PruneKind]int64
	pruned      []docker.PruneKind
	probed      int
	pruneErr    map[docker.PruneKind]error
	dfErr       error
}

func (f *fakeDocker) Probe(context.Context) (docker.Status, error) {
	f.probed++
	return f.status, nil
}

func (f *fakeDocker) Reclaimable(context.Context) (docker.Reclaimable, error) {
	if f.dfErr != nil {
		return docker.Reclaimable{}, f.dfErr
	}
	return f.reclaimable, nil
}

func (f *fakeDocker) DanglingImageIDs(context.Context) ([]string, error) {
	return f.dangling, nil
}

func (f *fakeDocker) Prune(_ context.Context, k docker.PruneKind) (docker.PruneResult, error) {
	f.pruned = append(f.pruned, k)
	if err := f.pruneErr[k]; err != nil {
		return docker.PruneResult{}, err
	}
	return docker.PruneResult{FreedBytes: f.freed[k]}, nil
}

func newFakeDocker() *fakeDocker {
	return &fakeDocker{
		status:      docker.StatusOK,
		reclaimable: docker.Reclaimable{Images: 100, BuildCache: 200},
		dangling:    []string{"a", "b", "c"},
		freed: map[docker.PruneKind]int64{
			docker.PruneDangling:  10,
			docker.PruneAllUnused: 40,
			docker.PruneBuilder:   25,
		},
	}
}

func dockerCfg(d config.Docker) *config.Config {
	return &config.Config{
		Storage:   config.Storage{Volume: "/", ThresholdType: "percent", ThresholdValue: 15, SafetyMarginPercent: 5},
		Retention: config.Retention{MinAgeDays: 0},
		Docker:    d,
		EvictionTargets: config.EvictionTargets{
			SafeCaches: []config.Target{{Path: "~/App", Type: "cache"}},
		},
	}
}

func dockerHome(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return home, writeFileIn(t, dir, "f", 100, time.Now())
}

func TestDockerDryRunReportsReclaimableAndDoesNotPrune(t *testing.T) {
	// arrange
	home, file := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	opts := EngineOptions{Apply: false, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if len(fd.pruned) != 0 {
		t.Errorf("dry-run must not prune, got %v", fd.pruned)
	}
	if res.Docker.ReclaimableBytes != 200 {
		t.Errorf("ReclaimableBytes = %d, want 200 (dangling-only does not count df images)", res.Docker.ReclaimableBytes)
	}
	if res.Docker.ImagesBytes != 0 {
		t.Errorf("ImagesBytes = %d, want 0", res.Docker.ImagesBytes)
	}
	if res.Docker.BuildCacheBytes != 200 {
		t.Errorf("BuildCacheBytes = %d, want 200", res.Docker.BuildCacheBytes)
	}
	if !slices.Equal(res.Docker.DanglingImageIDs, []string{"a", "b", "c"}) {
		t.Errorf("DanglingImageIDs = %v, want [a b c]", res.Docker.DanglingImageIDs)
	}
	if res.Docker.DanglingImageCount != 3 {
		t.Errorf("DanglingImageCount = %d, want 3", res.Docker.DanglingImageCount)
	}
	if res.Docker.FreedBytes != 0 {
		t.Errorf("FreedBytes = %d, want 0", res.Docker.FreedBytes)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("dry-run must not delete files: %v", err)
	}
}

func TestDockerApplyPrunesDanglingAndBuilder(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	want := []docker.PruneKind{docker.PruneDangling, docker.PruneBuilder}
	if !slices.Equal(fd.pruned, want) {
		t.Errorf("pruned = %v, want %v", fd.pruned, want)
	}
	if res.Docker.FreedBytes != 35 {
		t.Errorf("Docker.FreedBytes = %d, want 35", res.Docker.FreedBytes)
	}
	if res.FreedBytes != 100 {
		t.Errorf("file FreedBytes = %d, want 100 (docker bytes reported separately)", res.FreedBytes)
	}
	if !res.Docker.Ran {
		t.Error("Docker.Ran = false, want true")
	}
	if len(res.Docker.Prunes) != 2 {
		t.Errorf("Docker.Prunes = %v, want 2 entries", res.Docker.Prunes)
	}
}

func TestDockerApplyUnusedImagesReplacesDanglingPrune(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true, UnusedImages: true})
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	_, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	want := []docker.PruneKind{docker.PruneAllUnused, docker.PruneBuilder}
	if !slices.Equal(fd.pruned, want) {
		t.Errorf("pruned = %v, want %v", fd.pruned, want)
	}
}

func TestDockerApplyBuilderCacheOffSkipsBuilderPrune(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true})
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	_, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(fd.pruned, []docker.PruneKind{docker.PruneDangling}) {
		t.Errorf("pruned = %v, want [PruneDangling]", fd.pruned)
	}
}

func TestDockerApplyDanglingOffSkipsImagePrune(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, BuilderCache: true})
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	_, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(fd.pruned, []docker.PruneKind{docker.PruneBuilder}) {
		t.Errorf("pruned = %v, want [PruneBuilder]", fd.pruned)
	}
}

func TestDockerDisabledMakesNoCalls(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: false, DanglingImages: true, BuilderCache: true})
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if fd.probed != 0 || len(fd.pruned) != 0 {
		t.Errorf("docker must not be touched when disabled: probed=%d pruned=%v", fd.probed, fd.pruned)
	}
	if res.Docker.Ran {
		t.Error("Docker.Ran = true, want false")
	}
}

func TestDockerBinaryMissingSkipsWithReason(t *testing.T) {
	// arrange
	home, file := dockerHome(t)
	fd := newFakeDocker()
	fd.status = docker.StatusBinaryMissing
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatalf("missing docker must not be an error: %v", err)
	}
	if res.Docker.SkippedReason == "" {
		t.Error("SkippedReason empty, want a reason")
	}
	if len(fd.pruned) != 0 {
		t.Errorf("pruned = %v, want none", fd.pruned)
	}
	if res.FreedBytes != 100 {
		t.Errorf("file pass must proceed normally; FreedBytes = %d", res.FreedBytes)
	}
	if _, err := os.Stat(file); err == nil {
		t.Error("file should still be deleted")
	}
}

func TestDockerDaemonDownSkipsWithReason(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	fd.status = docker.StatusDaemonDown
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatalf("daemon down must not be an error: %v", err)
	}
	if res.Docker.SkippedReason == "" {
		t.Error("SkippedReason empty, want a reason")
	}
	if len(fd.pruned) != 0 {
		t.Errorf("pruned = %v, want none", fd.pruned)
	}
	if res.FreedBytes != 100 {
		t.Errorf("file pass must proceed normally; FreedBytes = %d", res.FreedBytes)
	}
}

func TestDockerSkippedWhenFileEvictionRecoveredDisk(t *testing.T) {
	// arrange
	home, file := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	usage := func(string) (int64, int64, error) {
		if _, err := os.Stat(file); err == nil {
			return 90, 1000, nil
		}
		return 900, 1000, nil
	}
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: usage, Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if res.FreedBytes != 100 {
		t.Fatalf("precondition: file eviction FreedBytes = %d, want 100", res.FreedBytes)
	}
	if len(fd.pruned) != 0 {
		t.Errorf("docker must not prune once disk recovered; pruned = %v", fd.pruned)
	}
	if res.Docker.Ran {
		t.Error("Docker.Ran = true, want false")
	}
}

func TestDockerRunsAfterFileEviction(t *testing.T) {
	// arrange
	home, file := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true})
	var fileGoneAtPrune bool
	wrapped := &orderDocker{fakeDocker: fd, onPrune: func() {
		_, err := os.Stat(file)
		fileGoneAtPrune = err != nil
	}}
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: wrapped}

	// act
	_, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if len(fd.pruned) != 1 {
		t.Fatalf("pruned = %v, want 1 prune", fd.pruned)
	}
	if !fileGoneAtPrune {
		t.Error("docker prune must run after file eviction")
	}
}

type orderDocker struct {
	*fakeDocker
	onPrune func()
}

func (o *orderDocker) Prune(ctx context.Context, k docker.PruneKind) (docker.PruneResult, error) {
	o.onPrune()
	return o.fakeDocker.Prune(ctx, k)
}

func TestDockerNotRunAboveThreshold(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	opts := EngineOptions{Apply: true, HomeDir: home, DiskUsage: fakeDiskUsage(500, 1000), Docker: fd}

	// act
	_, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if len(fd.pruned) != 0 {
		t.Errorf("docker must not prune when above threshold; pruned = %v", fd.pruned)
	}
}

func TestDockerDryRunUnusedImagesCountsImagesBytes(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true, UnusedImages: true})
	opts := EngineOptions{Apply: false, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if res.Docker.ImagesBytes != 100 || res.Docker.BuildCacheBytes != 200 {
		t.Errorf("ImagesBytes/BuildCacheBytes = %d/%d, want 100/200", res.Docker.ImagesBytes, res.Docker.BuildCacheBytes)
	}
	if res.Docker.ReclaimableBytes != 300 {
		t.Errorf("ReclaimableBytes = %d, want 300", res.Docker.ReclaimableBytes)
	}
}

func TestDockerDryRunBuilderCacheOffExcludesBuildCache(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	cfg := dockerCfg(config.Docker{Enabled: true, UnusedImages: true})
	opts := EngineOptions{Apply: false, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if res.Docker.BuildCacheBytes != 0 || res.Docker.ReclaimableBytes != 100 {
		t.Errorf("BuildCacheBytes/Reclaimable = %d/%d, want 0/100", res.Docker.BuildCacheBytes, res.Docker.ReclaimableBytes)
	}
}

func TestDockerApplyPruneFailureGoesToErrorsNotSkipped(t *testing.T) {
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
	if res.Docker.SkippedReason != "" {
		t.Errorf("SkippedReason = %q, want empty", res.Docker.SkippedReason)
	}
	if len(res.Docker.Errors) != 1 || !strings.Contains(res.Docker.Errors[0], "boom") {
		t.Errorf("Errors = %v, want one entry containing boom", res.Docker.Errors)
	}
	if !res.Docker.Ran {
		t.Error("Docker.Ran = false, want true")
	}
	if res.Docker.FreedBytes != 25 {
		t.Errorf("FreedBytes = %d, want 25 (builder prune still ran)", res.Docker.FreedBytes)
	}
}

func TestDockerDryRunDfFailureGoesToErrorsNotSkipped(t *testing.T) {
	// arrange
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	fd.dfErr = errors.New("df broke")
	cfg := dockerCfg(config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	opts := EngineOptions{Apply: false, HomeDir: home, DiskUsage: fakeDiskUsage(90, 1000), Docker: fd}

	// act
	res, err := Run(cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if res.Docker.SkippedReason != "" {
		t.Errorf("SkippedReason = %q, want empty", res.Docker.SkippedReason)
	}
	if len(res.Docker.Errors) != 1 || !strings.Contains(res.Docker.Errors[0], "df broke") {
		t.Errorf("Errors = %v, want one entry containing df broke", res.Docker.Errors)
	}
	if res.Docker.ReclaimableBytes != 0 {
		t.Errorf("ReclaimableBytes = %d, want 0", res.Docker.ReclaimableBytes)
	}
}
