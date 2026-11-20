package eviction

import (
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

var unusedT0 = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

type unusedEnv struct {
	home  string
	state string
	fd    *fakeDocker
	cfg   *config.Config
}

func newUnusedEnv(t *testing.T, d config.Docker) *unusedEnv {
	t.Helper()
	home, _ := dockerHome(t)
	fd := newFakeDocker()
	fd.images = []docker.Image{
		{ID: "sha256:idle", Tags: []string{"idle:1"}, Size: 300},
		{ID: "sha256:busy", Tags: []string{"busy:1"}, Size: 500},
	}
	fd.inUse = map[string]bool{"sha256:busy": true}
	d.Enabled = true
	d.UnusedImages = true
	return &unusedEnv{
		home:  home,
		state: filepath.Join(t.TempDir(), "state", "docker-images.json"),
		fd:    fd,
		cfg:   dockerCfg(d),
	}
}

func (e *unusedEnv) run(t *testing.T, apply bool, free int64, now time.Time) RunResult {
	t.Helper()
	res, err := Run(e.cfg, EngineOptions{
		Apply:           apply,
		HomeDir:         e.home,
		DiskUsage:       fakeDiskUsage(free, 1000),
		Docker:          e.fd,
		DockerStatePath: e.state,
		Now:             func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestUnusedImagesFirstPassRemovesNothing(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{})

	// act
	res := e.run(t, true, 90, unusedT0)

	// assert
	if len(e.fd.removed) != 0 {
		t.Errorf("removed = %v, want none on first observation", e.fd.removed)
	}
	if len(res.Docker.UnusedImages) != 0 {
		t.Errorf("UnusedImages = %v, want none", res.Docker.UnusedImages)
	}
}

func TestUnusedImagesRemovesOnlyIdleAfterMinAge(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{})
	e.run(t, true, 90, unusedT0)

	// act
	res := e.run(t, true, 90, unusedT0.Add(49*time.Hour))

	// assert
	if !slices.Equal(e.fd.removed, []string{"sha256:idle"}) {
		t.Errorf("removed = %v, want [sha256:idle]", e.fd.removed)
	}
	if !slices.Equal(res.Docker.UnusedImages, []string{"idle:1"}) {
		t.Errorf("UnusedImages = %v", res.Docker.UnusedImages)
	}
}

func TestUnusedImagesBelowMinAgeNotRemoved(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{})
	e.run(t, true, 90, unusedT0)

	// act
	e.run(t, true, 90, unusedT0.Add(47*time.Hour))

	// assert
	if len(e.fd.removed) != 0 {
		t.Errorf("removed = %v, want none before 48h", e.fd.removed)
	}
}

func TestUnusedImagesInUseRecentlyNeverRemoved(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{})
	e.fd.inUse = map[string]bool{"sha256:busy": true}
	e.run(t, true, 90, unusedT0)
	e.fd.inUse = map[string]bool{"sha256:idle": true, "sha256:busy": true}
	e.run(t, true, 90, unusedT0.Add(48*time.Hour))
	e.fd.inUse = map[string]bool{"sha256:busy": true}

	// act
	e.run(t, true, 90, unusedT0.Add(49*time.Hour))

	// assert
	if len(e.fd.removed) != 0 {
		t.Errorf("image in use 1h ago must not be removed; removed = %v", e.fd.removed)
	}
}

func TestUnusedImagesWithStoppedContainerNeverRemoved(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{})
	e.fd.inUse = map[string]bool{"sha256:idle": true, "sha256:busy": true}
	e.run(t, true, 90, unusedT0)

	// act
	e.run(t, true, 90, unusedT0.Add(500*time.Hour))

	// assert
	if len(e.fd.removed) != 0 {
		t.Errorf("removed = %v, want none", e.fd.removed)
	}
}

func TestUnusedImagesObservedAboveThresholdButNotRemoved(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{})

	// act
	res := e.run(t, true, 500, unusedT0)
	later := e.run(t, true, 90, unusedT0.Add(49*time.Hour))

	// assert
	if !res.UnderThreshold {
		t.Fatal("precondition: first pass must be above threshold")
	}
	if _, err := os.Stat(e.state); err != nil {
		t.Errorf("state must be written above threshold: %v", err)
	}
	if !slices.Equal(e.fd.removed, []string{"sha256:idle"}) {
		t.Errorf("baseline from the above-threshold pass should make the image stale; removed = %v", e.fd.removed)
	}
	if !slices.Equal(later.Docker.UnusedImages, []string{"idle:1"}) {
		t.Errorf("UnusedImages = %v", later.Docker.UnusedImages)
	}
}

func TestUnusedImagesAboveThresholdNeverRemoves(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{})
	e.run(t, true, 500, unusedT0)

	// act
	e.run(t, true, 500, unusedT0.Add(100*time.Hour))

	// assert
	if len(e.fd.removed) != 0 || len(e.fd.pruned) != 0 {
		t.Errorf("above threshold must not remove or prune; removed = %v pruned = %v", e.fd.removed, e.fd.pruned)
	}
}

func TestUnusedImagesDisabledNeverTouchesImages(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{DanglingImages: true})
	e.cfg.Docker.UnusedImages = false

	// act
	e.run(t, true, 90, unusedT0)
	e.run(t, true, 90, unusedT0.Add(100*time.Hour))

	// assert
	if e.fd.imagesCalls != 0 || len(e.fd.removed) != 0 {
		t.Errorf("imagesCalls = %d removed = %v, want none", e.fd.imagesCalls, e.fd.removed)
	}
	if _, err := os.Stat(e.state); !os.IsNotExist(err) {
		t.Errorf("state file must not be written; stat err = %v", err)
	}
}

func TestUnusedImagesDockerDisabledNeverTouchesImages(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{})
	e.cfg.Docker.Enabled = false

	// act
	e.run(t, true, 90, unusedT0)

	// assert
	if e.fd.imagesCalls != 0 {
		t.Errorf("imagesCalls = %d, want 0", e.fd.imagesCalls)
	}
}

func TestUnusedImagesApplyRecordsPrunesAndFreedBytes(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{DanglingImages: true, BuilderCache: true})
	e.run(t, true, 90, unusedT0)

	// act
	res := e.run(t, true, 90, unusedT0.Add(49*time.Hour))

	// assert
	if !slices.Equal(res.Docker.Prunes, []string{"image prune", "builder prune", "image rm idle:1"}) {
		t.Errorf("Prunes = %v", res.Docker.Prunes)
	}
	if res.Docker.FreedBytes != 10+25+300 {
		t.Errorf("FreedBytes = %d, want 335", res.Docker.FreedBytes)
	}
	if !slices.Equal(e.fd.pruned[len(e.fd.pruned)-2:], []docker.PruneKind{docker.PruneDangling, docker.PruneBuilder}) {
		t.Errorf("pruned = %v", e.fd.pruned)
	}
}

func TestUnusedImagesRemoveFailureGoesToErrors(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{})
	e.fd.images = append(e.fd.images, docker.Image{ID: "sha256:idle2", Tags: []string{"idle2:1"}, Size: 40})
	e.fd.rmErr = map[string]error{"sha256:idle": errors.New("conflict")}
	e.run(t, true, 90, unusedT0)

	// act
	res := e.run(t, true, 90, unusedT0.Add(49*time.Hour))

	// assert
	if len(res.Docker.Errors) != 1 || !strings.Contains(res.Docker.Errors[0], "conflict") || !strings.Contains(res.Docker.Errors[0], "idle:1") {
		t.Errorf("Errors = %v", res.Docker.Errors)
	}
	if !slices.Equal(res.Docker.Prunes, []string{"image rm idle2:1"}) {
		t.Errorf("Prunes = %v, want only the successful removal", res.Docker.Prunes)
	}
	if res.Docker.FreedBytes != 40 {
		t.Errorf("FreedBytes = %d, want 40", res.Docker.FreedBytes)
	}
	if res.Docker.SkippedReason != "" {
		t.Errorf("SkippedReason = %q", res.Docker.SkippedReason)
	}
}

func TestUnusedImagesCustomMinAgeHours(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{UnusedMinAgeHours: 6})
	e.run(t, true, 90, unusedT0)

	// act
	e.run(t, true, 90, unusedT0.Add(5*time.Hour))
	before := len(e.fd.removed)
	e.run(t, true, 90, unusedT0.Add(7*time.Hour))

	// assert
	if before != 0 {
		t.Errorf("removed after 5h = %v, want none", e.fd.removed)
	}
	if !slices.Equal(e.fd.removed, []string{"sha256:idle"}) {
		t.Errorf("removed after 7h = %v, want [sha256:idle]", e.fd.removed)
	}
}

func TestUnusedImagesDryRunListsIdleAndSumsSizes(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{DanglingImages: true, BuilderCache: true})
	e.run(t, false, 90, unusedT0)

	// act
	res := e.run(t, false, 90, unusedT0.Add(49*time.Hour))

	// assert
	if !slices.Equal(res.Docker.UnusedImages, []string{"idle:1"}) {
		t.Errorf("UnusedImages = %v", res.Docker.UnusedImages)
	}
	if res.Docker.ImagesBytes != 300 || res.Docker.BuildCacheBytes != 200 || res.Docker.ReclaimableBytes != 500 {
		t.Errorf("Images/BuildCache/Reclaimable = %d/%d/%d, want 300/200/500", res.Docker.ImagesBytes, res.Docker.BuildCacheBytes, res.Docker.ReclaimableBytes)
	}
	if len(e.fd.removed) != 0 || len(e.fd.pruned) != 0 {
		t.Errorf("dry-run must not remove or prune; removed = %v pruned = %v", e.fd.removed, e.fd.pruned)
	}
}

func TestUnusedImagesDryRunIgnoresDfImagesBytes(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{})
	e.fd.reclaimable = docker.Reclaimable{Images: 999_999, BuildCache: 0}

	// act
	res := e.run(t, false, 90, unusedT0)

	// assert
	if res.Docker.ImagesBytes != 0 || res.Docker.ReclaimableBytes != 0 {
		t.Errorf("ImagesBytes/Reclaimable = %d/%d, want 0/0", res.Docker.ImagesBytes, res.Docker.ReclaimableBytes)
	}
}

func TestUnusedImagesDefaultStatePathUnderHome(t *testing.T) {
	// arrange
	e := newUnusedEnv(t, config.Docker{})
	opts := EngineOptions{Apply: false, HomeDir: e.home, DiskUsage: fakeDiskUsage(90, 1000), Docker: e.fd}

	// act
	_, err := Run(e.cfg, opts)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(e.home, ".local", "state", "thepurge", "docker-images.json")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("default state file missing: %v", err)
	}
}
