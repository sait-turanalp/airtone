# Architecture

AirTone is a thin, well-tested orchestration layer over a proven audio pipeline.
The Go program never touches audio samples itself — it drives a shell/Swift
engine and reads live state from Snapcast's control API.

## Data flow

```
Music app ─▶ "AirTone Sync" (Multi-Output)
                 ├─▶ Built-in speakers      (instant, what you hear on the Mac)
                 └─▶ BlackHole 2ch (master) ─▶ sox ─▶ named pipe ─▶ snapserver
                                                                       │
                                              serves Snapweb on :1780  │  control on :1705
                                                                       ▼
                                                        phone browser (Snapweb)
```

## Components and why each was chosen

| Component | Role | Why this one |
|-----------|------|--------------|
| **BlackHole 2ch** | Virtual output device so system audio can be captured | The standard, free, low-overhead macOS loopback driver |
| **Multi-Output device** | Splits audio to speakers **and** BlackHole | Lets the Mac stay audible while we capture |
| **BlackHole = master clock** | The Multi-Output's reference device | Stops the captured stream from drifting → no constant resync |
| **sox** (`-t coreaudio`) | Reads BlackHole gaplessly into the pipe | Callback-based capture; does **not** drop samples like `ffmpeg -f avfoundation` did |
| **snapserver** | Encodes (opus), buffers, syncs all clients, serves Snapweb | Purpose-built for sample-accurate multi-client sync |
| **Snapweb** | The player that runs in the phone browser | No app to install — open a URL / scan a QR |

## Sync model

Snapcast locks every client to a shared server clock. The `buffer` value is how
far behind "live" all listeners play — **together**. Bigger buffer = more delay
but more resilience to network jitter. This is why perfect sync and ultra-low
latency are a trade-off, not both-at-once. AirTone exposes three profiles
(Low / Balanced / Smooth).

The Mac's own speakers play **directly** through the Multi-Output (not through
Snapcast), so the Mac is effectively "live" while the phone trails by the buffer.
For a phone used as a second, away-from-the-desk speaker this is ideal; placing
the phone right next to the Mac will comb-filter, as with any two unsynced
sources.

## Go layer

```
cmd/airtone/        entrypoint + headless subcommands
internal/engine/    extracts the embedded engine to ~/.airtone and runs it
internal/rpc/       Snapcast JSON-RPC client (Server.GetStatus, …)
internal/doctor/    readiness checks (binaries, driver, device, Snapweb)
internal/tui/       Bubble Tea UI (dashboard, doctor, settings)
assets/             snapserver.conf template + Multi-Output Swift helper
scripts/            start / stop / setup engine scripts
```

The scripts and assets are **embedded** into the binary (`//go:embed`) and
extracted to `~/.airtone/engine` at runtime, so a single binary is fully
self-contained. `AIRTONE_*` environment variables (e.g. `AIRTONE_BUFFER`,
`AIRTONE_CODEC`, `AIRTONE_HOME`) tune behavior.

## Ports

| Port | Purpose |
|------|---------|
| 1780 | HTTP — serves Snapweb and the streaming WebSocket (this is the phone URL) |
| 1705 | TCP — JSON-RPC control/status (used by `status` and the TUI dashboard) |
| 1704 | TCP — native Snapcast client stream port (not needed for the browser flow) |
