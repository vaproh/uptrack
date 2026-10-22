#!/bin/sh
# uptrack installer.
#
#   curl -fsSL https://raw.githubusercontent.com/vaproh/uptrack/main/install.sh | sh
#
# Env:
#   INSTALL_DIR      where to place the binary (default ~/.local/bin)
#   UPTRACK_VERSION  tag to install, e.g. v0.1.0 (default: latest)
set -eu

REPO="vaproh/uptrack"
DEST="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${UPTRACK_VERSION:-latest}"

if [ "$(uname -s)" != "Linux" ]; then
	echo "uptrack supports Linux only" >&2
	exit 1
fi
case "$(uname -m)" in
	x86_64) SUFFIX="x86_64" ;;
	aarch64 | arm64) SUFFIX="aarch64" ;;
	*)
		echo "unsupported architecture: $(uname -m)" >&2
		exit 1
		;;
esac

ASSET="uptrack-linux-$SUFFIX"
if [ "$VERSION" = "latest" ]; then
	BASE="https://github.com/$REPO/releases/latest/download"
else
	BASE="https://github.com/$REPO/releases/download/$VERSION"
fi

need() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "missing required tool: $1" >&2
		exit 1
	}
}
need curl
need sha256sum

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM
cd "$TMP"

echo "downloading $ASSET..."
curl -fsSL -o "$ASSET" "$BASE/$ASSET"
curl -fsSL -o sha256sums.txt "$BASE/sha256sums.txt"

echo "verifying checksum..."
grep " $ASSET\$" sha256sums.txt | sha256sum -c - >/dev/null

mkdir -p "$DEST"
install -m 0755 "$ASSET" "$DEST/uptrack"

echo "installed uptrack to $DEST/uptrack"
case ":$PATH:" in
	*":$DEST:"*) ;;
	*)
		echo "note: $DEST is not on PATH; add this to your shell config:"
		echo "  export PATH=\"\$PATH:$DEST\""
		;;
esac
echo "run 'uptrack add <owner/repo>' to track your first app."
echo "thereafter, update with 'uptrack self-update'."
