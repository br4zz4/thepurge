package hooks

import (
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	// arrange
	t.Setenv("HOME", "/home/fake")

	// act / assert
	if got := ExpandHome("~/bin/x.sh"); got != filepath.Join("/home/fake", "bin/x.sh") {
		t.Errorf("tilde path = %q", got)
	}
	if got := ExpandHome("/abs/x.sh"); got != "/abs/x.sh" {
		t.Errorf("absolute path = %q", got)
	}
	if got := ExpandHome("rel/x.sh"); got != "rel/x.sh" {
		t.Errorf("relative path = %q", got)
	}
}
