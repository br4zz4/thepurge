package docker

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const fakeDockerScript = `#!/bin/sh
if [ -n "$FAKE_DOCKER_LOG" ]; then
  echo "$*" >> "$FAKE_DOCKER_LOG"
fi
case "$1" in
  version)
    if [ -n "$FAKE_DOCKER_DOWN" ]; then
      echo "Cannot connect to the Docker daemon" >&2
      exit 1
    fi
    echo "27.0.1"
    ;;
  system)
    printf 'Images\t12.3MB\n'
    printf 'Containers\t0B\n'
    printf 'Local Volumes\t5GB\n'
    printf 'Build Cache\t1.5GB\n'
    ;;
  image)
    case "$2" in
      ls)
        printf 'abc123\ndef456\n'
        ;;
      prune)
        echo "Deleted Images: x"
        echo "Total reclaimed space: 12.3MB"
        ;;
    esac
    ;;
  builder)
    echo "Total reclaimed space: 1.5GB"
    ;;
esac
`

func newFake(t *testing.T) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "docker")
	if err := os.WriteFile(bin, []byte(fakeDockerScript), 0o755); err != nil {
		t.Fatal(err)
	}
	logFile := filepath.Join(dir, "calls.log")
	t.Setenv("FAKE_DOCKER_LOG", logFile)
	return &Client{Bin: bin}, logFile
}

func readCalls(t *testing.T, logFile string) []string {
	t.Helper()
	b, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestProbeOK(t *testing.T) {
	// arrange
	c, _ := newFake(t)

	// act
	st, err := c.Probe(context.Background())

	// assert
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if st != StatusOK {
		t.Errorf("Probe() = %v, want StatusOK", st)
	}
}

func TestProbeBinaryMissing(t *testing.T) {
	// arrange
	c := &Client{Bin: filepath.Join(t.TempDir(), "no-such-docker")}

	// act
	st, err := c.Probe(context.Background())

	// assert
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if st != StatusBinaryMissing {
		t.Errorf("Probe() = %v, want StatusBinaryMissing", st)
	}
}

func TestProbeDaemonDown(t *testing.T) {
	// arrange
	c, _ := newFake(t)
	t.Setenv("FAKE_DOCKER_DOWN", "1")

	// act
	st, err := c.Probe(context.Background())

	// assert
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if st != StatusDaemonDown {
		t.Errorf("Probe() = %v, want StatusDaemonDown", st)
	}
}

func TestReclaimableParsesDecimalSizes(t *testing.T) {
	// arrange
	c, logFile := newFake(t)

	// act
	r, err := c.Reclaimable(context.Background())

	// assert
	if err != nil {
		t.Fatalf("Reclaimable() error = %v", err)
	}
	if r.Images != 12_300_000 {
		t.Errorf("Images = %d, want 12300000", r.Images)
	}
	if r.BuildCache != 1_500_000_000 {
		t.Errorf("BuildCache = %d, want 1500000000", r.BuildCache)
	}
	calls := readCalls(t, logFile)
	if len(calls) != 1 || !strings.HasPrefix(calls[0], "system df") {
		t.Errorf("calls = %v, want a single `system df`", calls)
	}
}

func TestDanglingImageIDs(t *testing.T) {
	// arrange
	c, logFile := newFake(t)

	// act
	ids, err := c.DanglingImageIDs(context.Background())

	// assert
	if err != nil {
		t.Fatalf("DanglingImageIDs() error = %v", err)
	}
	if !slices.Equal(ids, []string{"abc123", "def456"}) {
		t.Errorf("ids = %v", ids)
	}
	calls := readCalls(t, logFile)
	if len(calls) != 1 || !strings.Contains(calls[0], "dangling=true") {
		t.Errorf("calls = %v, want dangling filter", calls)
	}
}

func TestPruneDangling(t *testing.T) {
	// arrange
	c, logFile := newFake(t)

	// act
	res, err := c.Prune(context.Background(), PruneDangling)

	// assert
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if res.FreedBytes != 12_300_000 {
		t.Errorf("FreedBytes = %d, want 12300000", res.FreedBytes)
	}
	calls := readCalls(t, logFile)
	if !slices.Equal(calls, []string{"image prune -f"}) {
		t.Errorf("calls = %v, want [image prune -f]", calls)
	}
}

func TestPruneBuilder(t *testing.T) {
	// arrange
	c, logFile := newFake(t)

	// act
	res, err := c.Prune(context.Background(), PruneBuilder)

	// assert
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if res.FreedBytes != 1_500_000_000 {
		t.Errorf("FreedBytes = %d, want 1500000000", res.FreedBytes)
	}
	calls := readCalls(t, logFile)
	if !slices.Equal(calls, []string{"builder prune -f"}) {
		t.Errorf("calls = %v, want [builder prune -f]", calls)
	}
}

func TestNewClientDefaultsToDockerBinary(t *testing.T) {
	// arrange
	// act
	c := NewClient()

	// assert
	if c.Bin != "docker" {
		t.Errorf("Bin = %q, want docker", c.Bin)
	}
}

func TestReclaimableParsesRealDfOutputWithPercentAndKB(t *testing.T) {
	// arrange
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"printf 'Images\\t1.2GB (45%%)\\n'\n" +
		"printf 'Containers\\t0B (0%%)\\n'\n" +
		"printf 'Local Volumes\\t5GB (100%%)\\n'\n" +
		"printf 'Build Cache\\t512kB\\n'\n"
	bin := filepath.Join(dir, "docker")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	c := &Client{Bin: bin}

	// act
	r, err := c.Reclaimable(context.Background())

	// assert
	if err != nil {
		t.Fatalf("Reclaimable() error = %v", err)
	}
	if r.Images != 1_200_000_000 {
		t.Errorf("Images = %d, want 1200000000", r.Images)
	}
	if r.BuildCache != 512_000 {
		t.Errorf("BuildCache = %d, want 512000", r.BuildCache)
	}
}

func TestProbeFindsBinaryInExtraPaths(t *testing.T) {
	// arrange
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte("#!/bin/sh\necho 27.0.1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	old := ExtraPaths
	ExtraPaths = []string{dir}
	t.Cleanup(func() { ExtraPaths = old })

	// act
	st, err := NewClient().Probe(context.Background())

	// assert
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if st != StatusOK {
		t.Errorf("Probe() = %v, want StatusOK", st)
	}
}

func TestProbeBinaryMissingWithoutExtraPaths(t *testing.T) {
	// arrange
	t.Setenv("PATH", t.TempDir())
	old := ExtraPaths
	ExtraPaths = nil
	t.Cleanup(func() { ExtraPaths = old })

	// act
	st, err := NewClient().Probe(context.Background())

	// assert
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if st != StatusBinaryMissing {
		t.Errorf("Probe() = %v, want StatusBinaryMissing", st)
	}
}
