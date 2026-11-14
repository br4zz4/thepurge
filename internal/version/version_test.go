package version

import (
	"runtime"
	"slices"
	"testing"
)

func TestShortCommit(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{name: "long commit is truncated", info: Info{Commit: "12a7697abcdef0123456789"}, want: "12a7697"},
		{name: "short commit is kept whole", info: Info{Commit: "abc"}, want: "abc"},
		{name: "exactly seven chars", info: Info{Commit: "abcdefg"}, want: "abcdefg"},
		{name: "dirty appends suffix", info: Info{Commit: "12a7697abcdef", Dirty: true}, want: "12a7697-dirty"},
		{name: "empty commit", info: Info{}, want: ""},
		{name: "empty commit ignores dirty", info: Info{Dirty: true}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			info := tt.info

			// act
			got := info.ShortCommit()

			// assert
			if got != tt.want {
				t.Errorf("ShortCommit() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{name: "with commit", info: Info{Version: "dev", Commit: "12a7697abcdef"}, want: "dev (12a7697)"},
		{name: "dirty", info: Info{Version: "dev", Commit: "12a7697abcdef", Dirty: true}, want: "dev (12a7697-dirty)"},
		{name: "without commit", info: Info{Version: "dev"}, want: "dev"},
		{name: "custom version", info: Info{Version: "v0.1.0", Commit: "abcdef1234"}, want: "v0.1.0 (abcdef1)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			info := tt.info

			// act
			got := info.String()

			// assert
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLines(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want []string
	}{
		{
			name: "all fields",
			info: Info{
				Version:    "dev",
				Commit:     "12a7697abcdef",
				CommitTime: "2026-11-14T10:00:00Z",
				Go:         "go1.27.0",
			},
			want: []string{
				"thepurge dev (12a7697)",
				"date:   2026-11-14T10:00:00Z",
				"go:     go1.27.0",
			},
		},
		{
			name: "without commit time",
			info: Info{
				Version: "dev",
				Commit:  "12a7697abcdef",
				Go:      "go1.27.0",
			},
			want: []string{
				"thepurge dev (12a7697)",
				"go:     go1.27.0",
			},
		},
		{
			name: "without go",
			info: Info{
				Version:    "dev",
				Commit:     "12a7697abcdef",
				CommitTime: "2026-11-14T10:00:00Z",
			},
			want: []string{
				"thepurge dev (12a7697)",
				"date:   2026-11-14T10:00:00Z",
			},
		},
		{
			name: "version only",
			info: Info{Version: "dev"},
			want: []string{"thepurge dev"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			info := tt.info

			// act
			got := info.Lines()

			// assert
			if !slices.Equal(got, tt.want) {
				t.Errorf("Lines() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestReport(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{
			name: "with go",
			info: Info{Version: "dev", Commit: "12a7697abcdef", Go: "go1.27.0"},
			want: "version: dev (12a7697) go1.27.0",
		},
		{
			name: "dirty",
			info: Info{Version: "v0.2.0", Commit: "abcdef1234", Dirty: true, Go: "go1.27.0"},
			want: "version: v0.2.0 (abcdef1-dirty) go1.27.0",
		},
		{
			name: "without go has no trailing space",
			info: Info{Version: "dev", Commit: "12a7697abcdef"},
			want: "version: dev (12a7697)",
		},
		{
			name: "version only",
			info: Info{Version: "dev"},
			want: "version: dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			info := tt.info

			// act
			got := info.Report()

			// assert
			if got != tt.want {
				t.Errorf("Report() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCurrent(t *testing.T) {
	// arrange
	// act
	info := Current()

	// assert
	if info.Version != Version {
		t.Errorf("Current().Version = %q, want package Version %q", info.Version, Version)
	}
	if info.Go != runtime.Version() {
		t.Errorf("Current().Go = %q, want %q", info.Go, runtime.Version())
	}
	if Version != "dev" {
		t.Errorf("default Version = %q, want %q", Version, "dev")
	}
}
