package version

import (
	"runtime"
	"runtime/debug"
	"strings"
)

// Version is the release version. Override it at build time with
// -ldflags "-X br4zz4/thepurge/internal/version.Version=v0.1.0".
var Version = "dev"

const shortCommitLen = 7

// Info describes the running binary. Build metadata comes from the VCS stamping
// the Go toolchain embeds when building inside a git repository.
type Info struct {
	Version    string
	Commit     string
	CommitTime string
	Dirty      bool
	Go         string
}

// Current reads the build info of the running binary.
func Current() Info {
	info := Info{Version: Version, Go: runtime.Version()}

	build, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, s := range build.Settings {
		switch s.Key {
		case "vcs.revision":
			info.Commit = s.Value
		case "vcs.time":
			info.CommitTime = s.Value
		case "vcs.modified":
			info.Dirty = s.Value == "true"
		}
	}
	return info
}

// ShortCommit returns the abbreviated commit hash, suffixed with -dirty when the
// working tree has uncommitted changes.
func (i Info) ShortCommit() string {
	if i.Commit == "" {
		return ""
	}
	c := i.Commit
	if len(c) > shortCommitLen {
		c = c[:shortCommitLen]
	}
	if i.Dirty {
		c += "-dirty"
	}
	return c
}

// String renders the one-line form: the version followed by the commit.
func (i Info) String() string {
	if c := i.ShortCommit(); c != "" {
		return i.Version + " (" + c + ")"
	}
	return i.Version
}

// Lines renders the multi-line form used by `thepurge version`.
func (i Info) Lines() []string {
	lines := []string{"thepurge " + i.String()}
	if i.CommitTime != "" {
		lines = append(lines, "date:   "+i.CommitTime)
	}
	if i.Go != "" {
		lines = append(lines, "go:     "+i.Go)
	}
	return lines
}

// Report is the single-line summary shown by `thepurge doctor`.
func (i Info) Report() string {
	return strings.TrimSpace("version: " + i.String() + " " + i.Go)
}
