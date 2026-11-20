package docker

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const imagesDockerScript = `#!/bin/sh
echo "$*" >> "$FAKE_DOCKER_LOG"
case "$1 $2" in
  "image ls")
    printf '%b' "$FAKE_IMAGE_LS"
    ;;
  "container ls")
    printf '%b' "$FAKE_CONTAINER_LS"
    ;;
  "image rm")
    if [ -n "$FAKE_RM_FAIL" ]; then
      echo "conflict: image is being used" >&2
      exit 1
    fi
    ;;
esac
case "$1" in
  inspect)
    printf '%b' "$FAKE_INSPECT"
    ;;
esac
`

func newImagesFake(t *testing.T) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "docker")
	if err := os.WriteFile(bin, []byte(imagesDockerScript), 0o755); err != nil {
		t.Fatal(err)
	}
	logFile := filepath.Join(dir, "calls.log")
	t.Setenv("FAKE_DOCKER_LOG", logFile)
	return &Client{Bin: bin}, logFile
}

func TestImagesDedupesByIDAndCollectsTags(t *testing.T) {
	// arrange
	c, _ := newImagesFake(t)
	t.Setenv("FAKE_IMAGE_LS", "sha256:aaa\\tapp:1\\t100MB\\nsha256:bbb\\tdb:latest\\t1.5GB\\nsha256:aaa\\tapp:latest\\t100MB\\n")

	// act
	imgs, err := c.Images(context.Background())

	// assert
	if err != nil {
		t.Fatalf("Images() error = %v", err)
	}
	if len(imgs) != 2 {
		t.Fatalf("Images() = %+v, want 2 entries", imgs)
	}
	if imgs[0].ID != "sha256:aaa" || !slices.Equal(imgs[0].Tags, []string{"app:1", "app:latest"}) || imgs[0].Size != 100_000_000 {
		t.Errorf("imgs[0] = %+v", imgs[0])
	}
	if imgs[1].ID != "sha256:bbb" || !slices.Equal(imgs[1].Tags, []string{"db:latest"}) || imgs[1].Size != 1_500_000_000 {
		t.Errorf("imgs[1] = %+v", imgs[1])
	}
}

func TestImagesIgnoresUntaggedEntries(t *testing.T) {
	// arrange
	c, _ := newImagesFake(t)
	t.Setenv("FAKE_IMAGE_LS", "sha256:aaa\\t<none>:<none>\\t10MB\\nsha256:bbb\\tapp:1\\t5MB\\n\\n")

	// act
	imgs, err := c.Images(context.Background())

	// assert
	if err != nil {
		t.Fatalf("Images() error = %v", err)
	}
	if len(imgs) != 1 || imgs[0].ID != "sha256:bbb" {
		t.Errorf("Images() = %+v, want only sha256:bbb", imgs)
	}
}

func TestImagesEmpty(t *testing.T) {
	// arrange
	c, _ := newImagesFake(t)
	t.Setenv("FAKE_IMAGE_LS", "")

	// act
	imgs, err := c.Images(context.Background())

	// assert
	if err != nil || len(imgs) != 0 {
		t.Errorf("Images() = %+v, %v; want empty", imgs, err)
	}
}

func TestImageLabel(t *testing.T) {
	// arrange
	tagged := Image{ID: "sha256:aaa", Tags: []string{"app:1", "app:latest"}}
	untagged := Image{ID: "sha256:bbb"}

	// act
	a, b := tagged.Label(), untagged.Label()

	// assert
	if a != "app:1" || b != "sha256:bbb" {
		t.Errorf("Label() = %q, %q", a, b)
	}
}

func TestInUseImageIDsIncludesStoppedContainers(t *testing.T) {
	// arrange
	c, logFile := newImagesFake(t)
	t.Setenv("FAKE_CONTAINER_LS", "c1\\nc2\\n")
	t.Setenv("FAKE_INSPECT", "sha256:aaa\\nsha256:bbb\\nsha256:aaa\\n")

	// act
	used, err := c.InUseImageIDs(context.Background())

	// assert
	if err != nil {
		t.Fatalf("InUseImageIDs() error = %v", err)
	}
	if len(used) != 2 || !used["sha256:aaa"] || !used["sha256:bbb"] {
		t.Errorf("used = %v", used)
	}
	calls := readCalls(t, logFile)
	if len(calls) < 2 || !strings.HasPrefix(calls[0], "container ls -a") || !strings.HasPrefix(calls[1], "inspect") || !strings.Contains(calls[1], "c1 c2") {
		t.Errorf("calls = %v", calls)
	}
}

func TestInUseImageIDsNoContainersSkipsInspect(t *testing.T) {
	// arrange
	c, logFile := newImagesFake(t)
	t.Setenv("FAKE_CONTAINER_LS", "")

	// act
	used, err := c.InUseImageIDs(context.Background())

	// assert
	if err != nil || len(used) != 0 {
		t.Fatalf("used = %v, err = %v; want empty", used, err)
	}
	for _, call := range readCalls(t, logFile) {
		if strings.HasPrefix(call, "inspect") {
			t.Errorf("inspect must not run without containers; calls = %v", readCalls(t, logFile))
		}
	}
}

func TestRemoveImageRunsWithoutForce(t *testing.T) {
	// arrange
	c, logFile := newImagesFake(t)

	// act
	err := c.RemoveImage(context.Background(), "sha256:aaa")

	// assert
	if err != nil {
		t.Fatalf("RemoveImage() error = %v", err)
	}
	if calls := readCalls(t, logFile); !slices.Equal(calls, []string{"image rm sha256:aaa"}) {
		t.Errorf("calls = %v, want [image rm sha256:aaa]", calls)
	}
}

func TestRemoveImageFailureReturnsStderr(t *testing.T) {
	// arrange
	c, _ := newImagesFake(t)
	t.Setenv("FAKE_RM_FAIL", "1")

	// act
	err := c.RemoveImage(context.Background(), "sha256:aaa")

	// assert
	if err == nil || !strings.Contains(err.Error(), "image is being used") {
		t.Errorf("RemoveImage() error = %v, want stderr included", err)
	}
}
