package docker

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func imgs(ids ...string) []Image {
	var out []Image
	for _, id := range ids {
		out = append(out, Image{ID: id, Tags: []string{id + ":1"}, Size: 10})
	}
	return out
}

func TestObserveFirstSeenGetsBaselineNow(t *testing.T) {
	// arrange
	store := UsageStore{Path: filepath.Join(t.TempDir(), "state.json")}

	// act
	got, err := store.Observe(imgs("a"), map[string]bool{}, t0)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !got["a"].Equal(t0) {
		t.Errorf("last use = %v, want %v", got["a"], t0)
	}
}

func TestObserveInUseUpdatesLastUse(t *testing.T) {
	// arrange
	store := UsageStore{Path: filepath.Join(t.TempDir(), "state.json")}
	if _, err := store.Observe(imgs("a"), map[string]bool{}, t0); err != nil {
		t.Fatal(err)
	}
	later := t0.Add(10 * time.Hour)

	// act
	got, err := store.Observe(imgs("a"), map[string]bool{"a": true}, later)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !got["a"].Equal(later) {
		t.Errorf("last use = %v, want %v", got["a"], later)
	}
}

func TestObserveNotInUsePreservesLastUse(t *testing.T) {
	// arrange
	store := UsageStore{Path: filepath.Join(t.TempDir(), "state.json")}
	if _, err := store.Observe(imgs("a"), map[string]bool{}, t0); err != nil {
		t.Fatal(err)
	}

	// act
	got, err := store.Observe(imgs("a"), map[string]bool{}, t0.Add(30*time.Hour))

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !got["a"].Equal(t0) {
		t.Errorf("last use = %v, want preserved %v", got["a"], t0)
	}
}

func TestObservePersistsAcrossStoreInstances(t *testing.T) {
	// arrange
	path := filepath.Join(t.TempDir(), "state.json")
	if _, err := (UsageStore{Path: path}).Observe(imgs("a"), map[string]bool{}, t0); err != nil {
		t.Fatal(err)
	}

	// act
	got, err := (UsageStore{Path: path}).Observe(imgs("a"), map[string]bool{}, t0.Add(5*time.Hour))

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !got["a"].Equal(t0) {
		t.Errorf("last use = %v, want %v", got["a"], t0)
	}
}

func TestObserveDropsMissingImages(t *testing.T) {
	// arrange
	store := UsageStore{Path: filepath.Join(t.TempDir(), "state.json")}
	if _, err := store.Observe(imgs("a", "b"), map[string]bool{}, t0); err != nil {
		t.Fatal(err)
	}

	// act
	got, err := store.Observe(imgs("a"), map[string]bool{}, t0.Add(time.Hour))

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["b"]; ok || len(got) != 1 {
		t.Errorf("got = %v, want only a", got)
	}
	again, _ := store.Observe(imgs("a", "b"), map[string]bool{}, t0.Add(48*time.Hour))
	if !again["b"].Equal(t0.Add(48 * time.Hour)) {
		t.Errorf("b must restart its baseline after being dropped; got %v", again["b"])
	}
}

func TestObserveCreatesMissingDirectory(t *testing.T) {
	// arrange
	path := filepath.Join(t.TempDir(), "deep", "nested", "state.json")

	// act
	_, err := UsageStore{Path: path}.Observe(imgs("a"), map[string]bool{}, t0)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("state file not created: %v", err)
	}
}

func TestObserveCorruptStateIsTreatedAsEmpty(t *testing.T) {
	// arrange
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// act
	got, err := UsageStore{Path: path}.Observe(imgs("a"), map[string]bool{}, t0)

	// assert
	if err != nil || !got["a"].Equal(t0) {
		t.Errorf("got = %v, err = %v", got, err)
	}
}

func TestStaleRespectsMinAge(t *testing.T) {
	// arrange
	images := imgs("old", "exact", "fresh")
	last := map[string]time.Time{
		"old":   t0.Add(-49 * time.Hour),
		"exact": t0.Add(-48 * time.Hour),
		"fresh": t0.Add(-47 * time.Hour),
	}

	// act
	got := Stale(images, map[string]bool{}, last, t0, 48*time.Hour)

	// assert
	if len(got) != 2 || got[0].ID != "old" || got[1].ID != "exact" {
		t.Errorf("Stale() = %+v, want old and exact", got)
	}
}

func TestStaleNeverReturnsInUse(t *testing.T) {
	// arrange
	last := map[string]time.Time{"a": t0.Add(-1000 * time.Hour)}

	// act
	got := Stale(imgs("a"), map[string]bool{"a": true}, last, t0, time.Hour)

	// assert
	if len(got) != 0 {
		t.Errorf("Stale() = %+v, want none", got)
	}
}

func TestStaleIgnoresImagesWithoutRecord(t *testing.T) {
	// arrange
	images := imgs("a")

	// act
	got := Stale(images, map[string]bool{}, map[string]time.Time{}, t0, time.Hour)

	// assert
	if len(got) != 0 {
		t.Errorf("Stale() = %+v, want none", got)
	}
}
