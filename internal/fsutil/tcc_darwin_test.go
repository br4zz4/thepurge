//go:build darwin

package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTCCProtected(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "music root", path: filepath.Join(home, "Music"), want: true},
		{name: "music child", path: filepath.Join(home, "Music", "Media", "song.m4a"), want: true},
		{name: "movies", path: filepath.Join(home, "Movies"), want: true},
		{name: "pictures library", path: filepath.Join(home, "Pictures", "Qualquer.photoslibrary"), want: true},
		{name: "safari", path: filepath.Join(home, "Library", "Safari"), want: true},
		{name: "messages", path: filepath.Join(home, "Library", "Messages"), want: true},
		{name: "reminders", path: filepath.Join(home, "Library", "Reminders"), want: true},
		{name: "accounts", path: filepath.Join(home, "Library", "Accounts"), want: true},
		{name: "caches music", path: filepath.Join(home, "Library", "Caches", "com.apple.Music"), want: true},
		{
			name: "caches media services child",
			path: filepath.Join(home, "Library", "Caches", "com.apple.AppleMediaServices", "x"),
			want: true,
		},
		{
			name: "application support photos child",
			path: filepath.Join(home, "Library", "Application Support", "com.apple.Photos", "x"),
			want: true,
		},
		{
			name: "group containers music child",
			path: filepath.Join(home, "Library", "Group Containers", "group.com.apple.music", "x"),
			want: true,
		},
		{name: "desktop stays reachable", path: filepath.Join(home, "Desktop"), want: false},
		{name: "documents stays reachable", path: filepath.Join(home, "Documents"), want: false},
		{name: "normal cache app", path: filepath.Join(home, "Library", "Caches", "normal-app"), want: false},
		{name: "logs", path: filepath.Join(home, "Library", "Logs"), want: false},
		{
			name: "application support claude",
			path: filepath.Join(home, "Library", "Application Support", "Claude"),
			want: false,
		},
		{name: "sibling name is not a child", path: filepath.Join(home, "Music2"), want: false},
		{
			name: "sibling cache prefix is not a child",
			path: filepath.Join(home, "Library", "Caches", "com.apple.MusicBackup"),
			want: false,
		},
		{name: "outside home", path: filepath.Join(string(filepath.Separator), "tmp", "Music"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// act
			got := IsTCCProtected(tt.path)

			// assert
			if got != tt.want {
				t.Errorf("IsTCCProtected(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsTCCProtectedCleansPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}

	// act
	got := IsTCCProtected(filepath.Join(home, "Library", "Caches", "com.apple.Music", ".", "Media", "..", "x"))

	// assert
	if !got {
		t.Errorf("IsTCCProtected() = false, want true for an uncleaned path inside a guarded dir")
	}
}
