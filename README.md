# thepurge

Reclaims disk space on macOS by evicting caches and logs when free space drops
below a threshold. Ships as a CLI plus a launchd agent that runs it on a
schedule. Everything is dry-run by default — nothing is deleted until you pass
`--apply`.

## Install

```sh
sh -c "$(curl -fsSL https://raw.githubusercontent.com/br4zz4/thepurge/main/install.sh)"
```

The script resolves the latest release, verifies it against `checksums.txt`, and
installs the binary to `~/.local/bin`. Override with:

| Variable | Meaning |
| --- | --- |
| `THEPURGE_VERSION` | pin a release, e.g. `v0.1.0` |
| `THEPURGE_INSTALL_DIR` | install elsewhere, e.g. `/usr/local/bin` |

Prebuilt tarballs for `darwin/arm64` and `darwin/amd64` are on the
[releases page](../../releases). Requires Go only if you build from source:

```sh
make build && make install
```

## Quick start

```sh
thepurge doctor            # config, disk access (Full Disk Access), targets
thepurge purge             # dry-run: what would be freed
thepurge purge --apply     # actually delete
thepurge service install   # run every 30 min via launchd (dry-run)
thepurge service install --apply
```

## Commands

| Command | Purpose |
| --- | --- |
| `thepurge status` | free space, threshold and last run |
| `thepurge doctor` | validate config, permissions and targets |
| `thepurge purge [--apply]` | preview (default) or perform eviction |
| `thepurge run [--apply]` | one eviction pass, used by the service |
| `thepurge logs [-t] [-n N]` | last N log lines, or follow |
| `thepurge service install\|uninstall\|status` | launchd agent management |
| `thepurge config init\|get\|set\|validate` | inspect and edit `config.yml` |
| `thepurge version` | version, commit and Go toolchain |

`service install` writes `~/Library/LaunchAgents/br4zz4.thepurge.plist` and
bootstraps it, replacing any previously loaded agent. `service status` reports
what launchd actually holds, including a stale binary path.

## Configuration

`~/.config/thepurge/config.yml`, or whatever `THEPURGE_CONFIG` points to. Run
`thepurge config init` to generate the default:

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
`logs`, `volatile_apps` and `orphan_apps`. Set `respect_running_apps: false`
only if you want caches of open apps deleted.

## Full Disk Access

macOS TCC blocks `~/Library/Application Support` and `~/Library/Containers`
until Full Disk Access is granted. Without it `doctor` fails and most targets
resolve as unreachable. Grant it to the terminal (interactive use) and to
`thepurge` itself (launchd runs).

## Uninstall

```sh
thepurge service uninstall
rm ~/.local/bin/thepurge
rm -r ~/.config/thepurge ~/Library/Logs/thepurge
```

## Development

```sh
make build     # version from git describe --tags --always --dirty
make install   # build + copy to ~/.local/bin
make test
make vet
make fmt
```

Releases are cut by pushing a `v*` tag; `.github/workflows/release.yml` runs the
tests and GoReleaser builds both architectures with the tag stamped into
`internal/version.Version`.

## License

MIT — see [LICENSE](LICENSE).
