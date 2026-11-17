package docker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func failingClient(t *testing.T) *Client {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "docker")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho boom >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Client{Bin: bin}
}

func TestClientCommandFailuresSurfaceStderr(t *testing.T) {
	// arrange
	c := failingClient(t)
	ctx := context.Background()

	// act
	_, recErr := c.Reclaimable(ctx)
	_, idsErr := c.DanglingImageIDs(ctx)
	_, pruneErr := c.Prune(ctx, PruneBuilder)

	// assert
	for name, err := range map[string]error{"Reclaimable": recErr, "DanglingImageIDs": idsErr, "Prune": pruneErr} {
		if err == nil {
			t.Errorf("%s: err = nil, want failure", name)
			continue
		}
		if got := err.Error(); !contains(got, "boom") {
			t.Errorf("%s: err = %q, want stderr included", name, got)
		}
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestPruneUnknownKindFails(t *testing.T) {
	// arrange
	c, _ := newFake(t)

	// act
	_, err := c.Prune(context.Background(), PruneKind(99))

	// assert
	if err == nil {
		t.Error("err = nil, want unknown prune kind")
	}
}

func TestReclaimableInvalidSizeFails(t *testing.T) {
	// arrange
	bin := filepath.Join(t.TempDir(), "docker")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf 'Images\\tzzz\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// act
	_, err := (&Client{Bin: bin}).Reclaimable(context.Background())

	// assert
	if err == nil {
		t.Error("err = nil, want parse error")
	}
}

func TestParseReclaimedWithoutMarkerIsZero(t *testing.T) {
	// act / assert
	if got := parseReclaimed("nothing here\nTotal reclaimed space: junk\n"); got != 0 {
		t.Errorf("parseReclaimed = %d, want 0", got)
	}
}
