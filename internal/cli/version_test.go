package cli

import (
	"bytes"
	"testing"

	"br4zz4/thepurge/internal/version"
)

func stubCurrentVersion(t *testing.T, info version.Info) {
	t.Helper()
	previous := currentVersion
	currentVersion = func() version.Info { return info }
	t.Cleanup(func() { currentVersion = previous })
}

func TestCmdVersionPrintsInfoLines(t *testing.T) {
	// arrange
	stubCurrentVersion(t, version.Info{
		Version:    "v0.2.0",
		Commit:     "abcdef1234",
		CommitTime: "2026-11-14T00:00:00Z",
		Go:         "go1.27.0",
	})
	var out, errOut bytes.Buffer

	// act
	code, err := cmdVersion(nil, &out, &errOut)

	// assert
	if err != nil {
		t.Fatalf("cmdVersion() error = %v", err)
	}
	if code != 0 {
		t.Errorf("cmdVersion() code = %d, want 0", code)
	}
	want := "thepurge v0.2.0 (abcdef1)\ndate:   2026-11-14T00:00:00Z\ngo:     go1.27.0\n"
	if out.String() != want {
		t.Errorf("cmdVersion() output = %q, want %q", out.String(), want)
	}
	if errOut.Len() != 0 {
		t.Errorf("cmdVersion() wrote to errOut: %q", errOut.String())
	}
}

func TestRunVersionAliases(t *testing.T) {
	argvList := [][]string{
		{"version"},
		{"--version"},
		{"-v"},
	}
	want := "thepurge v0.2.0 (abcdef1)\ndate:   2026-11-14T00:00:00Z\ngo:     go1.27.0\n"

	for _, argv := range argvList {
		t.Run(argv[0], func(t *testing.T) {
			// arrange
			stubCurrentVersion(t, version.Info{
				Version:    "v0.2.0",
				Commit:     "abcdef1234",
				CommitTime: "2026-11-14T00:00:00Z",
				Go:         "go1.27.0",
			})
			var out, errOut bytes.Buffer

			// act
			code := Run(argv, &out, &errOut)

			// assert
			if code != 0 {
				t.Errorf("Run(%v) code = %d, want 0", argv, code)
			}
			if out.String() != want {
				t.Errorf("Run(%v) output = %q, want %q", argv, out.String(), want)
			}
			if errOut.Len() != 0 {
				t.Errorf("Run(%v) wrote to errOut: %q", argv, errOut.String())
			}
		})
	}
}

func TestRunVersionWithoutCommit(t *testing.T) {
	// arrange
	stubCurrentVersion(t, version.Info{Version: "dev", Go: "go1.27.0"})
	var out, errOut bytes.Buffer

	// act
	code := Run([]string{"--version"}, &out, &errOut)

	// assert
	if code != 0 {
		t.Errorf("Run(--version) code = %d, want 0", code)
	}
	want := "thepurge dev\ngo:     go1.27.0\n"
	if out.String() != want {
		t.Errorf("Run(--version) output = %q, want %q", out.String(), want)
	}
}
