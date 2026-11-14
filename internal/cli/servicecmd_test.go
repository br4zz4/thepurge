package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"br4zz4/thepurge/internal/service"
)

func TestCmdServiceNoArgs(t *testing.T) {
	var out, errOut bytes.Buffer

	// act
	code, err := cmdService(nil, &out, &errOut)

	// assert
	if err != nil {
		t.Fatalf("cmdService() error = %v", err)
	}
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "usage:") {
		t.Errorf("expected usage message; got:\n%s", errOut.String())
	}
}

func TestCmdServiceUnknownSubcommand(t *testing.T) {
	var out, errOut bytes.Buffer

	// act
	code, err := cmdService([]string{"bogus"}, &out, &errOut)

	// assert
	if err != nil {
		t.Fatalf("cmdService() error = %v", err)
	}
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "unknown service subcommand") {
		t.Errorf("expected unknown subcommand error; got:\n%s", errOut.String())
	}
}

func withServiceHooks(
	t *testing.T,
	install func(service.InstallOptions) error,
	remove func(service.InstallOptions) error,
	current func(service.InstallOptions) (service.Status, error),
) {
	t.Helper()
	prevInstall, prevRemove, prevCurrent := serviceInstall, serviceRemove, serviceCurrent
	t.Cleanup(func() {
		serviceInstall, serviceRemove, serviceCurrent = prevInstall, prevRemove, prevCurrent
	})

	if install == nil {
		install = func(service.InstallOptions) error {
			t.Error("serviceInstall called unexpectedly")
			return nil
		}
	}
	if remove == nil {
		remove = func(service.InstallOptions) error {
			t.Error("serviceRemove called unexpectedly")
			return nil
		}
	}
	if current == nil {
		current = func(service.InstallOptions) (service.Status, error) {
			t.Error("serviceCurrent called unexpectedly")
			return service.Status{}, nil
		}
	}
	serviceInstall, serviceRemove, serviceCurrent = install, remove, current
}

func TestCmdServiceStatusStates(t *testing.T) {
	tests := []struct {
		name   string
		status service.Status
		want   []string
		absent []string
	}{
		{
			name:   "not installed",
			status: service.Status{PlistPath: "/tmp/br4zz4.thepurge.plist"},
			want:   []string{"service not installed"},
			absent: []string{"plist:", "binary:", "launchd:", "WARNING"},
		},
		{
			name: "installed and idle",
			status: service.Status{
				Installed:     true,
				PlistPath:     "/tmp/br4zz4.thepurge.plist",
				PlistProgram:  "/usr/local/bin/thepurge",
				ProgramExists: true,
				Loaded:        true,
				LastExit:      7,
			},
			want: []string{
				"plist:  /tmp/br4zz4.thepurge.plist",
				"binary: /usr/local/bin/thepurge",
				"launchd: loaded, idle (last exit 7)",
			},
			absent: []string{"service not installed", "WARNING"},
		},
		{
			name: "installed and running",
			status: service.Status{
				Installed:     true,
				PlistPath:     "/tmp/br4zz4.thepurge.plist",
				PlistProgram:  "/usr/local/bin/thepurge",
				ProgramExists: true,
				Loaded:        true,
				Running:       true,
				PID:           4242,
			},
			want:   []string{"plist:  /tmp/br4zz4.thepurge.plist", "binary: /usr/local/bin/thepurge", "launchd: loaded, running (pid 4242)"},
			absent: []string{"service not installed", "idle", "WARNING"},
		},
		{
			name: "installed but not loaded",
			status: service.Status{
				Installed:     true,
				PlistPath:     "/tmp/br4zz4.thepurge.plist",
				PlistProgram:  "/usr/local/bin/thepurge",
				ProgramExists: true,
			},
			want:   []string{"launchd: not loaded; run `thepurge service install`"},
			absent: []string{"service not installed", "WARNING"},
		},
		{
			name: "binary missing on disk",
			status: service.Status{
				Installed:    true,
				PlistPath:    "/tmp/br4zz4.thepurge.plist",
				PlistProgram: "/gone/thepurge",
			},
			want: []string{
				"binary: /gone/thepurge",
				"  WARNING: binary missing on disk; reinstall the service",
			},
		},
		{
			name: "launchd program mismatch",
			status: service.Status{
				Installed:     true,
				PlistPath:     "/tmp/br4zz4.thepurge.plist",
				PlistProgram:  "/new/thepurge",
				ProgramExists: true,
				Loaded:        true,
				Running:       true,
				PID:           9,
				Program:       "/old/thepurge",
			},
			want: []string{
				"  WARNING: launchd runs /old/thepurge but the plist points to /new/thepurge; reinstall the service",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withServiceHooks(t, nil, nil, func(service.InstallOptions) (service.Status, error) {
				return tt.status, nil
			})
			var out, errOut bytes.Buffer

			// act
			code, err := cmdService([]string{"status"}, &out, &errOut)

			// assert
			if err != nil {
				t.Fatalf("cmdService() error = %v", err)
			}
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			for _, want := range tt.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("stdout missing %q; got:\n%s", want, out.String())
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(out.String(), absent) {
					t.Errorf("stdout should not contain %q; got:\n%s", absent, out.String())
				}
			}
			if errOut.Len() != 0 {
				t.Errorf("unexpected stderr: %s", errOut.String())
			}
		})
	}
}

func TestCmdServiceStatusError(t *testing.T) {
	withServiceHooks(t, nil, nil, func(service.InstallOptions) (service.Status, error) {
		return service.Status{}, errors.New("service status: permission denied")
	})
	var out, errOut bytes.Buffer

	// act
	code, err := cmdService([]string{"status"}, &out, &errOut)

	// assert
	if err == nil {
		t.Fatal("cmdService() error = nil, want error")
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(err.Error(), "service status") {
		t.Errorf("error = %v, want service status context", err)
	}
	if out.Len() != 0 {
		t.Errorf("unexpected stdout: %s", out.String())
	}
}

func TestRunServiceStatusPropagatesError(t *testing.T) {
	withServiceHooks(t, nil, nil, func(service.InstallOptions) (service.Status, error) {
		return service.Status{}, errors.New("service status: read /tmp/x: permission denied")
	})
	var out, errOut bytes.Buffer

	// act
	code := Run([]string{"service", "status"}, &out, &errOut)

	// assert
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "thepurge: service status: read /tmp/x: permission denied") {
		t.Errorf("stderr missing error; got:\n%s", errOut.String())
	}
}

func TestCmdServiceUninstall(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var got service.InstallOptions
		withServiceHooks(t, nil, func(opts service.InstallOptions) error {
			got = opts
			return nil
		}, nil)
		var out, errOut bytes.Buffer

		// act
		code, err := cmdService([]string{"uninstall"}, &out, &errOut)

		// assert
		if err != nil {
			t.Fatalf("cmdService() error = %v", err)
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		if out.String() != "service uninstalled\n" {
			t.Errorf("stdout = %q, want %q", out.String(), "service uninstalled\n")
		}
		if got != (service.InstallOptions{}) {
			t.Errorf("serviceRemove opts = %+v, want zero value", got)
		}
	})

	t.Run("error", func(t *testing.T) {
		withServiceHooks(t, nil, func(service.InstallOptions) error {
			return errors.New("service uninstall: bootout failed")
		}, nil)
		var out, errOut bytes.Buffer

		// act
		code, err := cmdService([]string{"uninstall"}, &out, &errOut)

		// assert
		if err == nil {
			t.Fatal("cmdService() error = nil, want error")
		}
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if out.Len() != 0 {
			t.Errorf("unexpected stdout: %s", out.String())
		}
	})
}

func TestCmdServiceInstall(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		want  string
		apply bool
	}{
		{
			name:  "dry run",
			args:  []string{"install"},
			want:  "every 42 min, dry-run",
			apply: false,
		},
		{
			name:  "apply",
			args:  []string{"install", "--apply"},
			want:  "every 42 min, apply",
			apply: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "config.yml")
			yml := "storage:\n  volume: \"/\"\n  check_interval_minutes: 42\n"
			if err := os.WriteFile(cfgPath, []byte(yml), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("THEPURGE_CONFIG", cfgPath)

			var got service.InstallOptions
			withServiceHooks(t, func(opts service.InstallOptions) error {
				got = opts
				return nil
			}, nil, nil)
			var out, errOut bytes.Buffer

			// act
			code, err := cmdService(tt.args, &out, &errOut)

			// assert
			if err != nil {
				t.Fatalf("cmdService() error = %v", err)
			}
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			if got.IntervalSeconds != 42*60 {
				t.Errorf("IntervalSeconds = %d, want %d", got.IntervalSeconds, 42*60)
			}
			if got.Apply != tt.apply {
				t.Errorf("Apply = %v, want %v", got.Apply, tt.apply)
			}
			if !filepath.IsAbs(got.BinPath) {
				t.Errorf("BinPath = %q, want absolute path", got.BinPath)
			}
			if _, err := os.Stat(got.BinPath); err != nil {
				t.Errorf("BinPath %q does not exist: %v", got.BinPath, err)
			}
			if !strings.Contains(out.String(), tt.want) {
				t.Errorf("stdout missing %q; got:\n%s", tt.want, out.String())
			}
			if !strings.Contains(out.String(), got.BinPath) {
				t.Errorf("stdout missing binary path %q; got:\n%s", got.BinPath, out.String())
			}
			if errOut.Len() != 0 {
				t.Errorf("unexpected stderr: %s", errOut.String())
			}
		})
	}
}

func TestCmdServiceInstallConfigError(t *testing.T) {
	t.Setenv("THEPURGE_CONFIG", filepath.Join(t.TempDir(), "missing.yml"))
	withServiceHooks(t, nil, nil, nil)
	var out, errOut bytes.Buffer

	// act
	code, err := cmdService([]string{"install"}, &out, &errOut)

	// assert
	if err == nil {
		t.Fatal("cmdService() error = nil, want error")
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(err.Error(), "config not found") {
		t.Errorf("error = %v, want config not found", err)
	}
	if out.Len() != 0 {
		t.Errorf("unexpected stdout: %s", out.String())
	}
}
