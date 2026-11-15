// Package hooks runs user-defined commands around an eviction pass.
package hooks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"br4zz4/thepurge/config"
)

const defaultTimeout = 60 * time.Second

type Runner struct {
	DefaultTimeout time.Duration
}

// Label describes a hook for logs and dry-run listings.
func Label(h config.Hook) string {
	if h.Path != "" {
		return h.Path
	}
	return h.Command
}

// Run executes hooks in order. A failing hook does not stop the others; the
// returned error aggregates every failure.
func (r Runner) Run(ctx context.Context, event, mode string, hooks []config.Hook) error {
	var errs []error
	for _, h := range hooks {
		if err := r.runOne(ctx, event, mode, h); err != nil {
			errs = append(errs, fmt.Errorf("hook %q: %w", Label(h), err))
		}
	}
	return errors.Join(errs...)
}

// ExpandHome expands a leading "~/" to the user home directory.
func ExpandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func (r Runner) runOne(ctx context.Context, event, mode string, h config.Hook) error {
	timeout := r.DefaultTimeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if h.TimeoutSeconds > 0 {
		timeout = time.Duration(h.TimeoutSeconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if h.Path != "" {
		cmd = exec.CommandContext(ctx, ExpandHome(h.Path))
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", h.Command)
	}
	cmd.Env = append(os.Environ(), "THEPURGE_MODE="+mode, "THEPURGE_EVENT="+event)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("timed out after %s", timeout)
		}
		return err
	}
	return nil
}
