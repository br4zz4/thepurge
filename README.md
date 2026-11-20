# thepurge

Reclaims disk space by evicting caches and logs when free space drops below a
threshold. Runs on macOS (launchd) and Linux (systemd --user), as a CLI plus a
scheduled agent. Everything is dry-run by default — nothing is deleted until you
pass `--apply`.

## Install

```sh
sh -c "$(curl -fsSL https://raw.githubusercontent.com/br4zz4/thepurge/main/install.sh)"
```

The script resolves the latest release for your OS/arch, verifies it against
`checksums.txt`, and installs the binary to `~/.local/bin`. Override with:

| Variable | Meaning |
| --- | --- |
| `THEPURGE_VERSION` | pin a release, e.g. `v0.1.0` |
| `THEPURGE_INSTALL_DIR` | install elsewhere, e.g. `/usr/local/bin` |

Prebuilt tarballs for `darwin/{arm64,amd64}` and `linux/{arm64,amd64}` are on the
[releases page](../../releases). Requires Go only if you build from source:

```sh
make build && make install
```

## Quick start

```sh
thepurge doctor            # config, host access, targets
thepurge purge             # dry-run: what would be freed
thepurge purge --apply     # actually delete
thepurge service install   # run every 30 min (dry-run)
thepurge service install --apply
```

## Commands

| Command | Purpose |
| --- | --- |
| `thepurge status` | free space, threshold and last run |
| `thepurge doctor` | validate config, host access and targets |
| `thepurge purge [--apply]` | preview (default) or perform eviction of everything eligible, files and docker, regardless of the threshold |
| `thepurge run [--apply]` | one eviction pass, only when below the threshold; used by the service |
| `thepurge logs [-t] [-n N]` | last N log lines, or follow |
| `thepurge service install\|uninstall\|status` | agent management |
| `thepurge config init\|get\|set\|validate` | inspect and edit `config.yml` |
| `thepurge version` | version, commit and Go toolchain |

`service install` writes a launchd plist on macOS and a systemd user unit plus
timer on Linux, then loads it — replacing any previously loaded agent. Both
backends boot out or disable the old definition first, so the loaded job never
keeps a stale binary path. `service status` reports what the manager actually
holds, including that stale-path case.

| | macOS | Linux |
| --- | --- | --- |
| Definition | `~/Library/LaunchAgents/br4zz4.thepurge.plist` | `~/.config/systemd/user/thepurge.{service,timer}` |
| Manager | `launchctl` | `systemctl --user` |
| Logs | `~/Library/Logs/thepurge/thepurge.log` | `~/.local/state/thepurge/thepurge.log` |

## Configuration

`~/.config/thepurge/config.yml`, or whatever `THEPURGE_CONFIG` points to. Run
`thepurge config init` to generate the default for your platform:

```yaml
storage:
  volume: /
  threshold_type: percent   # percent | bytes
  threshold_value: 15       # purge when free space drops below 15%
  safety_margin_percent: 5  # stop once back above threshold + margin
  check_interval_minutes: 30
  max_bytes_per_run: ""     # optional cap per pass; empty = unlimited
scope_v1:
  delete_types: [caches, logs]
  priority_types: [logs, caches]
protection:
  respect_running_apps: true
immutable:
  check_immutable: true
retention:
  min_age_days: 7
```

`eviction_targets` lists the exact paths to evict, split into `safe_caches`,
`logs`, `volatile_apps` and `orphan_apps`. Defaults differ per platform:

| | macOS | Linux |
| --- | --- | --- |
| caches | `~/Library/Application Support/*` app caches, containers | `~/.cache`, `~/.npm/_cacache`, Slack/Code/discord caches |
| logs | `~/Library/Logs`, Slack container logs | `~/.npm/_logs`, `~/.config/Code/logs` |

`pattern` narrows a target by path component: with `pattern: "*Cache*"` on an
app data dir, only files that live inside a directory whose name matches are
evicted — not `Cookies`, `History` or `Login Data` sitting next to it. An empty
pattern (the default) means every file under the target.

Locations guarded by their own macOS privacy permission are always skipped:
media libraries (`~/Music`, `~/Movies`, `~/Pictures`), Photos, Messages, Mail,
Contacts, Calendars, Reminders, Safari history and the Apple media caches. A disk
cleaner has no business reading them, and touching them makes macOS prompt for
access it re-prompts on every new binary. `Desktop`, `Documents` and `Downloads`
are not skipped — pointing a target there is an explicit choice.

### Docker cleanup

```yaml
docker:
  enabled: true          # absent or false = the phase never runs
  dangling_images: true  # docker image prune -f
  builder_cache: true    # docker builder prune -f
  unused_images: false   # opt-in: also remove tagged images idle for a while
  unused_images_min_age_hours: 48  # idle time before an image is removed (0 = 48)
```

Once free space is below the threshold, a pass evicts everything eligible
(unless `max_bytes_per_run` is set) and then always runs the docker phase
(logs → caches → docker). It does not count against
`max_bytes_per_run`; freed bytes are reported separately (`docker freed:`).
With `unused_images: true` the service records, on every pass, which images
have a container (running or stopped); an image is removed only after it has
had none for `unused_images_min_age_hours` (default 48), counted from when
thepurge first saw it. The record lives in
`~/.local/state/thepurge/docker-images.json`, so enabling the option never
removes anything on the first pass. Images that are tagged and in use are never
touched. Dry-run lists the dangling IDs and the idle images that would go, and
removes nothing. A missing `docker` binary or a stopped daemon
is skipped with a warning, never an error. Volumes, networks and stopped
containers are never touched.

### Hooks

```yaml
hooks:
  pre_eviction:
    - path: ~/.config/thepurge/hooks/pre.sh
      timeout_seconds: 30
  post_eviction:
    - command: docker compose up -d
  on_failure:
    - command: notify-send "purge failed"
```

Each hook has exactly one of `path` (executable file, absolute or `~/`) or `command` (run via
`sh -c`), plus an optional `timeout_seconds` (default 60; a timeout is a
failure). Hooks run only with `--apply` and on every pass, even when free space
is above the threshold. In dry-run they are listed as `would run`, not executed.

- `pre_eviction` failing aborts the pass (exit 1, nothing deleted) and fires `on_failure`.
- `post_eviction` always runs; a failure is logged and notified but keeps exit 0.
- `on_failure` fires when the pass errors, the pre hook fails, or it is below the threshold and freed nothing.

Scripts receive `THEPURGE_MODE` (`apply`/`dry-run`) and `THEPURGE_EVENT`
(`pre_eviction`/`post_eviction`/`on_failure`).

Phases not shipped in V1: notification on eviction failure, 60s polling and
SIGHUP hot-reload.

## Host access

**macOS:** TCC blocks `~/Library/Application Support` and `~/Library/Containers`
until Full Disk Access is granted. Without it `doctor` fails and most targets
resolve as unreachable. Grant it to the terminal (interactive use) and to
`thepurge` itself (launchd runs).

macOS also asks per-binary permissions ("access data from other apps") the first
time a given path touches app containers. The grant covers the launchd agent too
— it judges the binary path, not who launched it — and it survives replacing the
binary at that same path. What re-prompts is running the tool from a *different*
path (a repo build, an old copy in `/usr/local/bin`). Keep one install path,
`~/.local/bin/thepurge`, and run `thepurge purge` once from a terminal to settle
the grant before the agent needs it.

**Linux:** nothing needs to be granted. `doctor` reports whether the systemd user
session is reachable, since `service install` needs it; eviction itself works
without. User timers only fire while the user has a session — run
`loginctl enable-linger "$USER"` on a headless host.

Immutable files are detected via `UF_IMMUTABLE` flags and `com.apple.*` xattrs on
macOS, and via the `FS_IMMUTABLE_FL`/`FS_APPEND_FL` inode flags (`chattr +i`) on
Linux. Failure notifications use `osascript` on macOS and `notify-send` on Linux
(skipped when libnotify is absent).

## Uninstall

```sh
thepurge service uninstall
rm ~/.local/bin/thepurge
rm -r ~/.config/thepurge
rm -r ~/Library/Logs/thepurge      # macOS
rm -r ~/.local/state/thepurge      # Linux
```

## Development

```sh
make build     # version from git describe --tags --always --dirty
make install   # build + copy to ~/.local/bin
make test
make vet
make fmt
```

CI runs the suite on macOS, Ubuntu and Arch (container). Releases are cut by
pushing a `v*` tag; `.github/workflows/release.yml` runs the tests and
GoReleaser builds both platforms with the tag stamped into
`internal/version.Version`.

## License

MIT — see [LICENSE](LICENSE).
