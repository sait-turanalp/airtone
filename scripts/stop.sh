#!/bin/bash
# AirTone engine — STOP
# Stops snapserver, the local snapclient and the system tap. Destroying the tap
# is what un-mutes the Mac, so it must happen on every exit path. No audio
# device is ever switched, so there is nothing to restore.
set -uo pipefail
# shellcheck source=scripts/common.sh disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

echo "==> Stopping services..."
kill_pipeline
rm -f "$FIFO"

echo "==> Stopped. Audio back to normal."
