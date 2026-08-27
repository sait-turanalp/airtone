#!/bin/bash
# AirTone engine — SETUP (one-time)
# 1) Compiles the CoreAudio system tap used to capture system audio.
# 2) Downloads the Snapweb browser player into the data dir.
# No virtual audio driver, no admin password, no reboot: the tap is a public
# CoreAudio API (macOS 14.2+) and never touches the user's output device.
set -euo pipefail
# shellcheck source=scripts/common.sh disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

SNAPWEB_VERSION="${AIRTONE_SNAPWEB_VERSION:-v0.9.3}"
SNAPWEB_URL="https://github.com/snapcast/snapweb/releases/download/${SNAPWEB_VERSION}/snapweb.zip"

mkdir -p "$AIRTONE_HOME"

echo "==> Building the system tap..."
build_tap
echo "    Ready at $TAP_BIN"

echo "==> Fetching Snapweb ($SNAPWEB_VERSION)..."
if [ -f "$SNAPWEB_DIR/index.html" ]; then
  echo "    Already present at $SNAPWEB_DIR"
else
  mkdir -p "$SNAPWEB_DIR"
  tmp="$(mktemp -t airtone-snapweb).zip"
  curl -fsSL "$SNAPWEB_URL" -o "$tmp"
  unzip -oq "$tmp" -d "$SNAPWEB_DIR"
  rm -f "$tmp"
  [ -f "$SNAPWEB_DIR/index.html" ] || { echo "error: Snapweb download looks empty" >&2; exit 1; }
  echo "    Installed to $SNAPWEB_DIR"
fi

echo "==> Setup complete. Run start next."
