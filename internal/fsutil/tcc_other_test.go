//go:build !darwin

package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTCCProtectedOffDarwin(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(string(filepath.Separator), "tmp", "Music"),
		filepath.Join(string(filepath.Separator), "Library", "Caches", "com.apple.Music"),
	}
	if home != "" {
		paths = append(paths,
			filepath.Join(home, "Music"),
			filepath.Join(home, "Library", "Caches", "com.apple.Music"),
			filepath.Join(home, "Library", "Safari"),
		)
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			// act
			got := IsTCCProtected(p)

			// assert
			if got {
				t.Errorf("IsTCCProtected(%q) = true, want false off macOS", p)
			}
		})
	}
}
