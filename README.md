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
| `thepurge purge [--apply]` | preview (default) or perform eviction |
| `thepurge run [--apply]` | one eviction pass, used by the service |
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
  max_bytes_per_run: 10GB
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

Phases not shipped in V1: notification on eviction failure, 60s polling and
SIGHUP hot-reload.

## Host access

**macOS:** TCC blocks `~/Library/Application Support` and `~/Library/Containers`
until Full Disk Access is granted. Without it `doctor` fails and most targets
resolve as unreachable. Grant it to the terminal (interactive use) and to
`thepurge` itself (launchd runs).

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
