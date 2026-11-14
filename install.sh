#!/bin/sh
set -eu

REPO="br4zz4/thepurge"
BIN="thepurge"
INSTALL_DIR="${THEPURGE_INSTALL_DIR:-$HOME/.local/bin}"

die() {
	printf 'thepurge installer: %s\n' "$*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

need curl
need tar
need shasum

[ "$(uname -s)" = "Darwin" ] || die "thepurge only runs on macOS"

case "$(uname -m)" in
arm64 | aarch64) ARCH="arm64" ;;
x86_64 | amd64) ARCH="amd64" ;;
*) die "unsupported architecture: $(uname -m)" ;;
esac

VERSION="${THEPURGE_VERSION:-}"
if [ -z "$VERSION" ]; then
	VERSION="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | sed 's|.*/tag/||')"
fi
case "$VERSION" in
v*) ;;
*) die "could not resolve the latest release (got \"$VERSION\"); set THEPURGE_VERSION=v0.1.0" ;;
esac

TARBALL="${BIN}_darwin_${ARCH}.tar.gz"
BASE="https://github.com/$REPO/releases/download/$VERSION"

TMPDIR_PURGE="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_PURGE"' EXIT

printf 'downloading %s %s (%s)\n' "$BIN" "$VERSION" "darwin/$ARCH"
curl -fsSL "$BASE/$TARBALL" -o "$TMPDIR_PURGE/$TARBALL" || die "download failed: $BASE/$TARBALL"
curl -fsSL "$BASE/checksums.txt" -o "$TMPDIR_PURGE/checksums.txt" || die "download failed: $BASE/checksums.txt"

EXPECTED="$(grep " $TARBALL\$" "$TMPDIR_PURGE/checksums.txt" | awk '{print $1}')"
[ -n "$EXPECTED" ] || die "no checksum for $TARBALL"
ACTUAL="$(shasum -a 256 "$TMPDIR_PURGE/$TARBALL" | awk '{print $1}')"
[ "$EXPECTED" = "$ACTUAL" ] || die "checksum mismatch for $TARBALL"

tar xzf "$TMPDIR_PURGE/$TARBALL" -C "$TMPDIR_PURGE"
[ -f "$TMPDIR_PURGE/$BIN" ] || die "archive did not contain $BIN"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMPDIR_PURGE/$BIN" "$INSTALL_DIR/$BIN"

printf 'installed %s to %s\n' "$VERSION" "$INSTALL_DIR/$BIN"

case ":$PATH:" in
*":$INSTALL_DIR:"*) ;;
*) printf '\nnote: %s is not in your PATH. add it with:\n  export PATH="%s:$PATH"\n' "$INSTALL_DIR" "$INSTALL_DIR" ;;
esac

printf '\nnext:\n  %s doctor            # validate config, disk access and targets\n  %s purge             # dry-run preview\n  %s service install   # run every 30 min via launchd\n' "$BIN" "$BIN" "$BIN"
