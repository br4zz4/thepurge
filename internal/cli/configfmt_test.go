package cli

import (
	"strings"
	"testing"

	"br4zz4/thepurge/config"
)

func TestRenderConfigShowsFormattedSummary(t *testing.T) {
	cfg := config.Default()
	out := renderConfig(cfg)

	for _, want := range []string{
		"storage:",
		"threshold:",
		"15%",
		"caches:",
		"logs:",
		"volatile",
		"orphans enabled: 0",
		"dry-run",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("renderConfig missing %q; got:\n%s", want, out)
		}
	}
}

func TestRenderConfigEmptyValues(t *testing.T) {
	out := renderConfig(&config.Config{})
	// Must not panic and must still render a plausible summary.
	if !strings.Contains(out, "storage:") {
		t.Errorf("renderConfig empty config missing storage header; got:\n%s", out)
	}
}
