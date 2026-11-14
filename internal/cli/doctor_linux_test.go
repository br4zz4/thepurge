//go:build linux

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCheckHostAccessWithoutRuntimeDir(t *testing.T) {
	// arrange
	t.Setenv("XDG_RUNTIME_DIR", "")
	var out bytes.Buffer

	// act
	ok := checkHostAccess(&out)

	// assert
	if !ok {
		t.Error("checkHostAccess() = false, want true; host access must never fail the doctor")
	}
	if !strings.Contains(out.String(), "no XDG_RUNTIME_DIR") {
		t.Errorf("output missing runtime dir note; got:\n%s", out.String())
	}
}
