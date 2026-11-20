package docker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// UsageStore remembers when each image was last seen in use by a container.
// Docker does not track this itself, so thepurge records it on every pass.
type UsageStore struct {
	Path string
}

type usageState map[string]time.Time

func (u UsageStore) load() usageState {
	st := usageState{}
	if b, err := os.ReadFile(u.Path); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	return st
}

// Observe updates the last-use time of every image and returns it. An image in
// use gets now; an image seen for the first time gets now as its baseline, so
// nothing becomes stale before it has been observed for the full min age.
// Entries of images that no longer exist are dropped.
func (u UsageStore) Observe(images []Image, inUse map[string]bool, now time.Time) (map[string]time.Time, error) {
	old := u.load()
	st := usageState{}
	for _, img := range images {
		last, seen := old[img.ID]
		if inUse[img.ID] || !seen {
			last = now
		}
		st[img.ID] = last
	}
	if err := os.MkdirAll(filepath.Dir(u.Path), 0o755); err != nil {
		return st, err
	}
	b, err := json.Marshal(st)
	if err != nil {
		return st, err
	}
	return st, os.WriteFile(u.Path, b, 0o644)
}

// Stale returns the images not in use whose last use is at least minAge old.
func Stale(images []Image, inUse map[string]bool, lastUse map[string]time.Time, now time.Time, minAge time.Duration) []Image {
	var out []Image
	for _, img := range images {
		if inUse[img.ID] {
			continue
		}
		if last, ok := lastUse[img.ID]; ok && now.Sub(last) >= minAge {
			out = append(out, img)
		}
	}
	return out
}
