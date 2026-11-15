package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"br4zz4/thepurge/config"
	"br4zz4/thepurge/internal/docker"
	"br4zz4/thepurge/internal/eviction"
)

type stubDocker struct {
	status      docker.Status
	reclaimable docker.Reclaimable
	dangling    []string
	freed       int64
	pruned      []docker.PruneKind
}

func (s *stubDocker) Probe(context.Context) (docker.Status, error) { return s.status, nil }
func (s *stubDocker) Reclaimable(context.Context) (docker.Reclaimable, error) {
	return s.reclaimable, nil
}
func (s *stubDocker) DanglingImageIDs(context.Context) ([]string, error) { return s.dangling, nil }
func (s *stubDocker) Prune(_ context.Context, k docker.PruneKind) (docker.PruneResult, error) {
	s.pruned = append(s.pruned, k)
	return docker.PruneResult{FreedBytes: s.freed}, nil
}

type evictionEnv struct {
	home   string
	target string
	log    string
}

func newEvictionEnv(t *testing.T) evictionEnv {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "App")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "cache.bin")
	if err := os.WriteFile(target, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(target, old, old); err != nil {
		t.Fatal(err)
	}
	return evictionEnv{home: home, target: target, log: filepath.Join(t.TempDir(), "hooks.log")}
}

func (e evictionEnv) cfg(h config.Hooks, d config.Docker) *config.Config {
	return &config.Config{
		Storage:   config.Storage{Volume: "/", ThresholdType: "percent", ThresholdValue: 15, SafetyMarginPercent: 5},
		Retention: config.Retention{MinAgeDays: 0},
		Docker:    d,
		Hooks:     h,
		EvictionTargets: config.EvictionTargets{
			SafeCaches: []config.Target{{Path: "~/App", Type: "cache"}},
		},
	}
}

func (e evictionEnv) opts(apply bool, free int64, dk docker.Ops) eviction.EngineOptions {
	return eviction.EngineOptions{
		Apply:   apply,
		HomeDir: e.home,
		DiskUsage: func(string) (int64, int64, error) {
			return free, 1000, nil
		},
		Docker: dk,
	}
}

func (e evictionEnv) record(tag string) config.Hook {
	return config.Hook{Command: `if [ -e "` + e.target + `" ]; then s=present; else s=gone; fi; echo "` + tag + ` $THEPURGE_MODE $THEPURGE_EVENT $s" >> "` + e.log + `"`}
}

func (e evictionEnv) lines(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(e.log)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func (e evictionEnv) fileExists() bool {
	_, err := os.Stat(e.target)
	return err == nil
}

func TestRunEvictionHooksPreAndPostAroundEviction(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	cfg := e.cfg(config.Hooks{
		PreEviction:  []config.Hook{e.record("pre")},
		PostEviction: []config.Hook{e.record("post")},
		OnFailure:    []config.Hook{e.record("fail")},
	}, config.Docker{})
	var out bytes.Buffer

	// act
	code, err := runEviction(cfg, e.opts(true, 90, nil), &out)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("runEviction = %d, %v; out:\n%s", code, err, out.String())
	}
	want := []string{"pre apply pre_eviction present", "post apply post_eviction gone"}
	if got := e.lines(t); !slices.Equal(got, want) {
		t.Errorf("hook log = %v, want %v", got, want)
	}
}

func TestRunEvictionHooksRunAboveThreshold(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	cfg := e.cfg(config.Hooks{
		PreEviction:  []config.Hook{e.record("pre")},
		PostEviction: []config.Hook{e.record("post")},
		OnFailure:    []config.Hook{e.record("fail")},
	}, config.Docker{})
	var out bytes.Buffer

	// act
	code, err := runEviction(cfg, e.opts(true, 500, nil), &out)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("runEviction = %d, %v; out:\n%s", code, err, out.String())
	}
	want := []string{"pre apply pre_eviction present", "post apply post_eviction present"}
	if got := e.lines(t); !slices.Equal(got, want) {
		t.Errorf("hook log = %v, want %v", got, want)
	}
	if !e.fileExists() {
		t.Error("file must not be evicted above threshold")
	}
}

func TestRunEvictionPreHookFailureAborts(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	cfg := e.cfg(config.Hooks{
		PreEviction:  []config.Hook{{Command: "exit 1"}},
		PostEviction: []config.Hook{e.record("post")},
		OnFailure:    []config.Hook{e.record("fail")},
	}, config.Docker{})
	var out bytes.Buffer

	// act
	code, _ := runEviction(cfg, e.opts(true, 90, nil), &out)

	// assert
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !e.fileExists() {
		t.Error("eviction must be aborted when pre hook fails")
	}
	got := e.lines(t)
	if len(got) != 1 || !strings.HasPrefix(got[0], "fail apply on_failure") {
		t.Errorf("hook log = %v, want only on_failure", got)
	}
}

func TestRunEvictionPreHookTimeoutAborts(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	cfg := e.cfg(config.Hooks{
		PreEviction: []config.Hook{{Command: "sleep 10", TimeoutSeconds: 1}},
		OnFailure:   []config.Hook{e.record("fail")},
	}, config.Docker{})
	var out bytes.Buffer

	// act
	code, _ := runEviction(cfg, e.opts(true, 90, nil), &out)

	// assert
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !e.fileExists() {
		t.Error("eviction must be aborted when pre hook times out")
	}
	if got := e.lines(t); len(got) != 1 {
		t.Errorf("on_failure should fire once, log = %v", got)
	}
}

func TestRunEvictionPostHookFailureKeepsExitZero(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	cfg := e.cfg(config.Hooks{
		PostEviction: []config.Hook{{Command: "exit 1"}},
	}, config.Docker{})
	var out bytes.Buffer

	// act
	code, err := runEviction(cfg, e.opts(true, 90, nil), &out)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("runEviction = %d, %v; want 0, nil", code, err)
	}
	if e.fileExists() {
		t.Error("eviction must be kept when post hook fails")
	}
}

func TestRunEvictionDryRunListsHooksWithoutRunning(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	cfg := e.cfg(config.Hooks{
		PreEviction:  []config.Hook{e.record("pre")},
		PostEviction: []config.Hook{{Path: "/opt/hooks/post.sh"}},
		OnFailure:    []config.Hook{e.record("fail")},
	}, config.Docker{})
	var out bytes.Buffer

	// act
	code, err := runEviction(cfg, e.opts(false, 90, nil), &out)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("runEviction = %d, %v", code, err)
	}
	if got := e.lines(t); len(got) != 0 {
		t.Errorf("dry-run must not execute hooks, log = %v", got)
	}
	if !strings.Contains(out.String(), "would run") {
		t.Errorf("dry-run output missing 'would run'; got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "/opt/hooks/post.sh") {
		t.Errorf("dry-run output should list hook path; got:\n%s", out.String())
	}
	if !e.fileExists() {
		t.Error("dry-run must not delete")
	}
}

func TestRunEvictionOnFailureWhenNothingFreed(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	if err := os.Remove(e.target); err != nil {
		t.Fatal(err)
	}
	cfg := e.cfg(config.Hooks{
		OnFailure: []config.Hook{e.record("fail")},
	}, config.Docker{})
	var out bytes.Buffer

	// act
	code, err := runEviction(cfg, e.opts(true, 90, nil), &out)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("runEviction = %d, %v", code, err)
	}
	got := e.lines(t)
	if len(got) != 1 || !strings.HasPrefix(got[0], "fail apply on_failure") {
		t.Errorf("on_failure should fire once; log = %v", got)
	}
}

func TestRunEvictionOnFailureNotFiredOnSuccess(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	cfg := e.cfg(config.Hooks{
		OnFailure: []config.Hook{e.record("fail")},
	}, config.Docker{})
	var out bytes.Buffer

	// act
	code, err := runEviction(cfg, e.opts(true, 90, nil), &out)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("runEviction = %d, %v", code, err)
	}
	if got := e.lines(t); len(got) != 0 {
		t.Errorf("on_failure must not fire on a successful pass; log = %v", got)
	}
}

func TestRunEvictionDockerDryRunShowsReclaimable(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	sd := &stubDocker{
		status:      docker.StatusOK,
		reclaimable: docker.Reclaimable{Images: 12_300_000, BuildCache: 1_500_000_000},
		dangling:    []string{"a", "b"},
	}
	cfg := e.cfg(config.Hooks{}, config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	var out bytes.Buffer

	// act
	code, err := runEviction(cfg, e.opts(false, 90, sd), &out)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("runEviction = %d, %v", code, err)
	}
	if !strings.Contains(out.String(), "docker: reclaimable") {
		t.Errorf("output missing docker reclaimable line; got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "2 dangling image") {
		t.Errorf("output missing dangling count; got:\n%s", out.String())
	}
	if len(sd.pruned) != 0 {
		t.Errorf("dry-run must not prune, got %v", sd.pruned)
	}
}

func TestRunEvictionDockerApplyReportsFreedSeparately(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	sd := &stubDocker{status: docker.StatusOK, freed: 5_000_000}
	cfg := e.cfg(config.Hooks{}, config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	var out bytes.Buffer

	// act
	code, err := runEviction(cfg, e.opts(true, 90, sd), &out)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("runEviction = %d, %v", code, err)
	}
	if !strings.Contains(out.String(), "docker freed") {
		t.Errorf("output missing 'docker freed'; got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "freed:") {
		t.Errorf("output missing file 'freed:' line; got:\n%s", out.String())
	}
	if !slices.Equal(sd.pruned, []docker.PruneKind{docker.PruneDangling, docker.PruneBuilder}) {
		t.Errorf("pruned = %v", sd.pruned)
	}
}

func TestRunEvictionDockerMissingDoesNotFail(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	sd := &stubDocker{status: docker.StatusBinaryMissing}
	cfg := e.cfg(config.Hooks{}, config.Docker{Enabled: true, DanglingImages: true, BuilderCache: true})
	var out bytes.Buffer

	// act
	code, err := runEviction(cfg, e.opts(true, 90, sd), &out)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("runEviction = %d, %v; out:\n%s", code, err, out.String())
	}
	if e.fileExists() {
		t.Error("file pass should proceed normally")
	}
}

func writeFakeDocker(t *testing.T, dir string, down bool) {
	t.Helper()
	body := "#!/bin/sh\n"
	if down {
		body += "exit 1\n"
	} else {
		body += "echo 27.0.1\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func noExtraDockerPaths(t *testing.T) {
	t.Helper()
	old := docker.ExtraPaths
	docker.ExtraPaths = nil
	t.Cleanup(func() { docker.ExtraPaths = old })
}

func doctorDockerConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "App"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDoctorConfig(t, home, "storage:\n  volume: \"/\"\n"+
		"docker:\n  enabled: true\n  dangling_images: true\n  builder_cache: true\n"+
		"eviction_targets:\n  safe_caches:\n    - path: \"~/App\"\n      type: cache\n")
}

func TestCmdDoctorDockerOK(t *testing.T) {
	// arrange
	doctorDockerConfig(t)
	bin := t.TempDir()
	writeFakeDocker(t, bin, false)
	t.Setenv("PATH", bin)
	var out, errOut bytes.Buffer

	// act
	code, err := cmdDoctor(nil, &out, &errOut)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("cmdDoctor = %d, %v; out:\n%s", code, err, out.String())
	}
	if !strings.Contains(out.String(), "docker: ok") {
		t.Errorf("doctor missing 'docker: ok'; got:\n%s", out.String())
	}
}

func TestCmdDoctorDockerNotInstalledDoesNotFail(t *testing.T) {
	// arrange
	doctorDockerConfig(t)
	noExtraDockerPaths(t)
	t.Setenv("PATH", t.TempDir())
	var out, errOut bytes.Buffer

	// act
	code, err := cmdDoctor(nil, &out, &errOut)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("cmdDoctor = %d, %v; out:\n%s", code, err, out.String())
	}
	if !strings.Contains(out.String(), "docker: not installed") {
		t.Errorf("doctor missing 'docker: not installed'; got:\n%s", out.String())
	}
}

func TestCmdDoctorDockerDaemonDownDoesNotFail(t *testing.T) {
	// arrange
	doctorDockerConfig(t)
	bin := t.TempDir()
	writeFakeDocker(t, bin, true)
	t.Setenv("PATH", bin)
	var out, errOut bytes.Buffer

	// act
	code, err := cmdDoctor(nil, &out, &errOut)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("cmdDoctor = %d, %v; out:\n%s", code, err, out.String())
	}
	if !strings.Contains(out.String(), "docker: daemon not running") {
		t.Errorf("doctor missing 'docker: daemon not running'; got:\n%s", out.String())
	}
}

func TestCmdDoctorDockerDisabledOmitsDockerLine(t *testing.T) {
	// arrange
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "App"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDoctorConfig(t, home, "storage:\n  volume: \"/\"\n"+
		"eviction_targets:\n  safe_caches:\n    - path: \"~/App\"\n      type: cache\n")
	var out, errOut bytes.Buffer

	// act
	_, _ = cmdDoctor(nil, &out, &errOut)

	// assert
	if strings.Contains(out.String(), "docker:") {
		t.Errorf("doctor must not mention docker when disabled; got:\n%s", out.String())
	}
}

func writeCfgFile(t *testing.T, yml string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(p, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("THEPURGE_CONFIG", p)
	return p
}

const baseCfgYAML = "storage:\n  volume: \"/\"\n  threshold_type: \"percent\"\n  threshold_value: 15\n" +
	"eviction_targets:\n  safe_caches:\n    - path: \"~/App\"\n      type: cache\n"

func TestConfigValidateRejectsHookWithPathAndCommand(t *testing.T) {
	// arrange
	writeCfgFile(t, baseCfgYAML+"hooks:\n  pre_eviction:\n    - path: /bin/true\n      command: \"true\"\n")
	var out, errOut bytes.Buffer

	// act
	code := Run([]string{"config", "validate"}, &out, &errOut)

	// assert
	if code == 0 {
		t.Errorf("validate exit = 0, want non-zero; out:\n%s", out.String())
	}
	if !strings.Contains(strings.ToLower(errOut.String()), "hook") {
		t.Errorf("error should point at the hook; got:\n%s", errOut.String())
	}
}

func TestConfigValidateRejectsHookWithNeither(t *testing.T) {
	// arrange
	writeCfgFile(t, baseCfgYAML+"hooks:\n  post_eviction:\n    - timeout_seconds: 5\n")
	var out, errOut bytes.Buffer

	// act
	code := Run([]string{"config", "validate"}, &out, &errOut)

	// assert
	if code == 0 {
		t.Errorf("validate exit = 0, want non-zero; out:\n%s", out.String())
	}
	if !strings.Contains(strings.ToLower(errOut.String()), "hook") {
		t.Errorf("error should point at the hook; got:\n%s", errOut.String())
	}
}

func TestConfigValidateAcceptsValidHooks(t *testing.T) {
	// arrange
	writeCfgFile(t, baseCfgYAML+"hooks:\n  pre_eviction:\n    - command: \"echo hi\"\n      timeout_seconds: 5\n")
	var out, errOut bytes.Buffer

	// act
	code := Run([]string{"config", "validate"}, &out, &errOut)

	// assert
	if code != 0 {
		t.Errorf("validate exit = %d, want 0; err:\n%s", code, errOut.String())
	}
}

func TestConfigSetGetDockerKeys(t *testing.T) {
	keys := []string{"docker.enabled", "docker.dangling_images", "docker.builder_cache", "docker.unused_images"}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			// arrange
			writeCfgFile(t, baseCfgYAML)
			var out, errOut bytes.Buffer
			if code := Run([]string{"config", "set", key, "true"}, &out, &errOut); code != 0 {
				t.Fatalf("set exit = %d, err=%s", code, errOut.String())
			}
			out.Reset()

			// act
			code := Run([]string{"config", "get", key}, &out, &errOut)

			// assert
			if code != 0 {
				t.Fatalf("get exit = %d, err=%s", code, errOut.String())
			}
			if strings.TrimSpace(out.String()) != key+": true" {
				t.Errorf("get %s = %q, want true", key, out.String())
			}

			out.Reset()
			if code := Run([]string{"config", "set", key, "false"}, &out, &errOut); code != 0 {
				t.Fatalf("set false exit = %d", code)
			}
			out.Reset()
			Run([]string{"config", "get", key}, &out, &errOut)
			if strings.TrimSpace(out.String()) != key+": false" {
				t.Errorf("get %s after false = %q", key, out.String())
			}
		})
	}
}

func TestConfigSummaryShowsDockerAndHooks(t *testing.T) {
	// arrange
	writeCfgFile(t, baseCfgYAML+"docker:\n  enabled: true\n  dangling_images: true\n"+
		"hooks:\n  pre_eviction:\n    - command: \"echo hi\"\n")
	var out, errOut bytes.Buffer

	// act
	code := Run([]string{"config"}, &out, &errOut)

	// assert
	if code != 0 {
		t.Fatalf("config exit = %d, err=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "docker") {
		t.Errorf("summary missing docker block; got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "hooks") {
		t.Errorf("summary missing hooks block; got:\n%s", out.String())
	}
}

func TestRunEvictionInvalidHookExitsOneAndRunsNothing(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	cfg := e.cfg(config.Hooks{
		PreEviction:  []config.Hook{e.record("pre")},
		PostEviction: []config.Hook{{Path: "/bin/true", Command: "true"}},
		OnFailure:    []config.Hook{e.record("fail")},
	}, config.Docker{})
	var out bytes.Buffer

	// act
	code, err := runEviction(cfg, e.opts(true, 90, nil), &out)

	// assert
	if code != 1 || err == nil {
		t.Fatalf("runEviction = %d, %v; want 1 and an error", code, err)
	}
	if got := e.lines(t); len(got) != 0 {
		t.Errorf("no hook may run on invalid config; log = %v", got)
	}
	if !e.fileExists() {
		t.Error("nothing may be evicted on invalid config")
	}
}

func TestRunEvictionPostHookFailureIsLogged(t *testing.T) {
	// arrange
	e := newEvictionEnv(t)
	logPath := filepath.Join(t.TempDir(), "purge.log")
	cfg := e.cfg(config.Hooks{
		PostEviction: []config.Hook{{Command: "exit 7"}},
	}, config.Docker{})
	cfg.Logging.LogPath = logPath
	var out bytes.Buffer

	// act
	code, err := runEviction(cfg, e.opts(true, 90, nil), &out)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("runEviction = %d, %v", code, err)
	}
	b, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var found bool
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "post_eviction failed") && strings.Contains(line, "exit status 7") {
			found = true
		}
	}
	if !found {
		t.Errorf("log missing post_eviction failure line; got:\n%s", b)
	}
}

func doctorHooksConfig(t *testing.T, home, hooksYAML string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, "App"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDoctorConfig(t, home, "storage:\n  volume: \"/\"\n"+
		"eviction_targets:\n  safe_caches:\n    - path: \"~/App\"\n      type: cache\n"+hooksYAML)
}

func TestCmdDoctorHooksMissingPathWarns(t *testing.T) {
	// arrange
	home := t.TempDir()
	doctorHooksConfig(t, home, "hooks:\n  pre_eviction:\n    - path: /nonexistent/thepurge-hook.sh\n")
	var out, errOut bytes.Buffer

	// act
	code, err := cmdDoctor(nil, &out, &errOut)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("cmdDoctor = %d, %v; out:\n%s", code, err, out.String())
	}
	if !strings.Contains(out.String(), "hooks pre_eviction: 1") {
		t.Errorf("doctor missing hook count; got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "WARNING /nonexistent/thepurge-hook.sh not found") {
		t.Errorf("doctor missing not-found warning; got:\n%s", out.String())
	}
}

func TestCmdDoctorHooksInvalidHookIsReported(t *testing.T) {
	// arrange
	home := t.TempDir()
	doctorHooksConfig(t, home, "hooks:\n  post_eviction:\n    - path: /bin/true\n      command: \"true\"\n")
	var out, errOut bytes.Buffer

	// act
	_, err := cmdDoctor(nil, &out, &errOut)

	// assert
	combined := strings.ToLower(out.String() + errOut.String())
	if err != nil {
		combined += strings.ToLower(err.Error())
	}
	if !strings.Contains(combined, "hook") {
		t.Errorf("doctor must point at the invalid hook; out:\n%s\nerr:\n%s\nerror: %v", out.String(), errOut.String(), err)
	}
}

func TestCmdDoctorHooksHomePathExistingHasNoWarning(t *testing.T) {
	// arrange
	home := t.TempDir()
	script := filepath.Join(home, "hook.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	doctorHooksConfig(t, home, "hooks:\n  pre_eviction:\n    - path: ~/hook.sh\n")
	var out, errOut bytes.Buffer

	// act
	code, err := cmdDoctor(nil, &out, &errOut)

	// assert
	if err != nil || code != 0 {
		t.Fatalf("cmdDoctor = %d, %v; out:\n%s", code, err, out.String())
	}
	if !strings.Contains(out.String(), "hooks pre_eviction: 1") {
		t.Errorf("doctor missing hook count; got:\n%s", out.String())
	}
	if strings.Contains(out.String(), "hooks pre_eviction: WARNING") {
		t.Errorf("existing ~/ hook must not warn; got:\n%s", out.String())
	}
}
