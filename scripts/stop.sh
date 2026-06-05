#!/bin/bash
# AirTone engine — STOP
# Stops snapserver + sox and restores the previous system audio output.
set -uo pipefail
# shellcheck source=scripts/common.sh disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

echo "==> Stopping services..."
pkill -x snapserver 2>/dev/null || true
pkill -x sox 2>/dev/null || true
rm -f "$FIFO"

# Restore output device.
PREV="$(cat "$PREV_OUTPUT_FILE" 2>/dev/null || true)"
if [ -n "$PREV" ] && [ "$PREV" != "$DEVICE_NAME" ]; then
  "$SWITCH" -s "$PREV" >/dev/null 2>&1 && echo "    Output restored: $PREV"
else
  # Fallback: switch to the built-in speakers if we can find them.
  BUILTIN="$("$SWITCH" -a 2>/dev/null | grep -iE 'speaker|hoparlör|built-?in' | head -1 || true)"
  [ -n "$BUILTIN" ] && "$SWITCH" -s "$BUILTIN" >/dev/null 2>&1 && echo "    Output -> $BUILTIN"
fi

echo "==> Stopped. Audio back to normal."
