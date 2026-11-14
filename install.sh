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

case "$(uname -s)" in
Darwin) OS="darwin" ;;
Linux) OS="linux" ;;
*) die "unsupported OS: $(uname -s) (thepurge supports macOS and Linux)" ;;
esac

case "$(uname -m)" in
arm64 | aarch64) ARCH="arm64" ;;
x86_64 | amd64) ARCH="amd64" ;;
*) die "unsupported architecture: $(uname -m)" ;;
esac

if command -v shasum >/dev/null 2>&1; then
	SHASUM="shasum -a 256"
elif command -v sha256sum >/dev/null 2>&1; then
	SHASUM="sha256sum"
else
	die "missing required command: shasum or sha256sum"
fi

need curl
need tar

VERSION="${THEPURGE_VERSION:-}"
if [ -z "$VERSION" ]; then
	VERSION="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | sed 's|.*/tag/||')"
fi
case "$VERSION" in
v*) ;;
*) die "could not resolve the latest release (got \"$VERSION\"); set THEPURGE_VERSION=v0.1.0" ;;
esac

TARBALL="${BIN}_${OS}_${ARCH}.tar.gz"
BASE="https://github.com/$REPO/releases/download/$VERSION"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

printf 'downloading %s %s (%s/%s)\n' "$BIN" "$VERSION" "$OS" "$ARCH"
curl -fsSL "$BASE/$TARBALL" -o "$WORKDIR/$TARBALL" || die "download failed: $BASE/$TARBALL"
curl -fsSL "$BASE/checksums.txt" -o "$WORKDIR/checksums.txt" || die "download failed: $BASE/checksums.txt"

EXPECTED="$(grep " $TARBALL\$" "$WORKDIR/checksums.txt" | awk '{print $1}')"
[ -n "$EXPECTED" ] || die "no checksum for $TARBALL"
ACTUAL="$($SHASUM "$WORKDIR/$TARBALL" | awk '{print $1}')"
[ "$EXPECTED" = "$ACTUAL" ] || die "checksum mismatch for $TARBALL"

tar xzf "$WORKDIR/$TARBALL" -C "$WORKDIR"
[ -f "$WORKDIR/$BIN" ] || die "archive did not contain $BIN"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$WORKDIR/$BIN" "$INSTALL_DIR/$BIN"

printf 'installed %s to %s\n' "$VERSION" "$INSTALL_DIR/$BIN"

case ":$PATH:" in
*":$INSTALL_DIR:"*) ;;
*) printf '\nnote: %s is not in your PATH. add it with:\n  export PATH="%s:$PATH"\n' "$INSTALL_DIR" "$INSTALL_DIR" ;;
esac

if [ "$OS" = "darwin" ]; then
	printf '\nnext:\n  %s doctor            # validate config, disk access and targets\n  %s purge             # dry-run preview\n  %s service install   # run every 30 min via launchd\n' "$BIN" "$BIN" "$BIN"
else
	printf '\nnext:\n  %s doctor            # validate config and targets\n  %s purge             # dry-run preview\n  %s service install   # run every 30 min via systemd --user\n' "$BIN" "$BIN" "$BIN"
fi
