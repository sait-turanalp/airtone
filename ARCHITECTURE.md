---
updated: 2026-08-27
---
# Architecture — AirTone

## overview
AirTone is a macOS-only Go CLI/TUI that turns a Mac into a synced, multi-room audio bridge:
it routes system audio to phones and speakers that play it in the **browser, no app to
install**. Two modes share one self-contained binary — **Party** (Snapcast multi-room,
sample-accurate sync, the Mac included) and **Instant** (WebRTC/Opus, low-latency, with
now-playing and transport controls). The Go layer only **orchestrates** a proven
shell/Swift/snapserver engine; it never reimplements the audio path.

## bird's-eye view
```
                       Music app (system audio)
                                │
                 CoreAudio process tap  (assets/systemtap.swift)
                 no virtual driver · the output device is never switched
                 Party: mutes at source, excludes snapclient
                                │  raw s16le on stdout → named pipe
                ┌───────────────┴────────────────┐
        Party  ▼                          Instant ▼
          snapserver                ffmpeg → Opus → pion/WebRTC
    (:1780 HTTP, :1705 RPC)                       │
         ├──────────────┐                         │
         ▼              ▼                         ▼
   snapclient      Snapweb in            phone browser (low-latency player +
   on the Mac      phone browser         now-playing / artwork / controls)
    (speakers)                           — the Mac keeps playing live here

   Both Party legs are snapcast clients on one server clock: that, not the
   buffer size, is what makes the Mac and the phone play as one pair.

   Go orchestrator: extracts the embedded engine, shells out to it,
   reads status over Snapcast JSON-RPC (:1705), renders the TUI.
```

## codemap
- `cmd/airtone/main.go` — entry point; dispatches subcommands (setup · party/start · instant ·
  stop · status · doctor · version) and owns Instant's run loop + teardown. Names: `main`,
  `runStatus`, `runDoctor`, `runInstant`.
- `embedfs.go` (package `airtone`) — `//go:embed scripts assets`; the engine `embed.FS` shipped
  inside the binary. Names: `Engine`.
- `internal/engine` — extracts the embedded engine to `~/.airtone/engine` and shells out to it.
  Names: `materialize`, `run`, `Setup`/`Start`/`Stop`, `TapBin`, `Home`, `LANIP`.
- `internal/rpc` — Snapcast JSON-RPC client (read-only status). Names: `GetStatus`
  (`Server.GetStatus`), `Status`.
- `internal/doctor` — readiness checks (binaries, macOS version for the tap API, the compiled
  tap, Snapweb). Names: `Run`, `Check`, `OK`, `atLeast`.
- `internal/tui` — Bubble Tea UI (dashboard · doctor · settings); idempotent exit teardown.
  Names: `Run`, `model`, `fireTeardown`, `updateHome`, `updateSettings`.
- `internal/instant` — Instant-mode WebRTC server (pion). Names: `server.go`.
- `internal/remote` — phone control layer over HTTP/SSE: now-playing, artwork, volume,
  transport. Names: `Register`, `Events` (SSE), `ResolveArtwork`/`PrefetchArtwork`, `Page`.
- `scripts/` — the engine: `common.sh` (all `AIRTONE_*` config) · `setup.sh` · `start.sh` ·
  `stop.sh`.
- `assets/` — `snapserver.conf.tmpl` (runtime config template) · `systemtap.swift` (the whole
  capture path: process tap → s16le on stdout, with `--mute`, `--exclude-pid` and `--probe`).
- ⚠️ `internal/{config,installer,pipeline}` exist but are **empty** (stale scaffold).

## runtime components & rationale
Non-derivable "why this one" — the recorded reasoning behind the engine's shape:

| Component | Role | Why this one |
|-----------|------|--------------|
| **CoreAudio process tap** (`systemtap.swift`) | Captures system audio | Public API since macOS 14.2: no driver install, no admin password, no reboot, and it never touches the user's output device. Replaced BlackHole + a Multi-Output device + sox |
| **Tap mute at source** (`CATapMutedWhenTapped`) | Silences the Mac's own playback in Party mode | The Mac's audio comes back through snapclient instead, putting it on the same clock as the phone. Dies with the tap process, so a crash restores sound |
| **Tap process exclusion** | Keeps snapclient out of the tap | Without it snapclient would capture its own playback — a feedback loop. Measured: excluded RMS 0.00000 vs included 0.34874 |
| **Silence pump at startup** | Breaks a start-order deadlock | snapclient only registers with CoreAudio once it is playing, and it only plays once the stream carries data. The tap streams real-time silence until the process it must exclude appears |
| **snapclient on the Mac** | Plays the stream back on the Mac's speakers | Makes the Mac a peer of the phone rather than a "live" source running ahead of it. It plays to the system default device — macOS snapclient has no device-selection flag |
| **snapserver** | Encodes (opus), buffers, syncs all clients, serves Snapweb | Purpose-built for sample-accurate multi-client sync |
| **Snapweb** | The player that runs in the phone browser | No app to install — open a URL / scan a QR |
| **ffmpeg → Opus → pion/WebRTC** | Instant mode's low-latency path | Trades multi-room sync for minimal phone latency |

## ports
| Port | Purpose |
|------|---------|
| 1780 | HTTP — serves Snapweb and the streaming WebSocket (the phone URL) |
| 1705 | TCP — JSON-RPC control/status (used by `status` and the TUI dashboard) |
| 1704 | TCP — native Snapcast client stream port (not needed for the browser flow) |

## sync model
Snapcast locks every client to a shared server clock; the `buffer` value is how far behind
"live" all listeners play — **together**. Bigger buffer = more delay but more jitter resilience,
so perfect sync and ultra-low latency are a trade-off, not both at once (AirTone exposes
Low / Balanced / Smooth profiles).

**Party** puts the Mac *inside* that clock: the tap mutes the Mac's own output and a local
snapclient plays the stream back, so the Mac and every phone are peers and the buffer delays
them equally. A phone next to the Mac no longer comb-filters. The browser (Snapweb on iOS
Safari) is the loosest client — expect tens of milliseconds, tunable per client with
snapcast's latency offset, not the sub-millisecond a native client reaches.

**Instant** does not mute: there is no local snapclient to play anything back, so the Mac
stays live and the phone trails by the WebRTC jitter buffer alone.

## boundaries
- **Go ↔ engine** — Go shells out to `/bin/bash scripts/*.sh`; the contract is the script set +
  `AIRTONE_*` env vars (`scripts/common.sh`), not function calls. Audio behavior changes go in
  scripts/assets, never Go.
- **Go ↔ snapserver** — read-only over JSON-RPC on :1705 (`internal/rpc`); Go reads status, it
  does not drive the stream.
- **Mac ↔ phone** — the phone is a pure browser client: Snapweb on :1780 (Party) or a WebRTC
  page + `/control/*` HTTP/SSE endpoints (Instant, `internal/remote`). No native app.
- **binary ↔ host** — the engine is materialized to `~/.airtone/engine` each run; the Swift tap
  is compiled once into `~/.airtone/engine/bin`. Runtime tools (snapserver/snapclient/ffmpeg/
  Snapweb) are external programs, not linked in.

## invariants
- **Go never reimplements the audio path** — it only extracts and orchestrates the proven
  shell/Swift/snapserver engine.
- **The tap is destroyed on every exit** — normal quit, Ctrl+C, SIGHUP / closed terminal, or a
  dead parent (EPIPE on stdout). A leaked tap leaves the Mac muted at the source; teardown is
  idempotent and runs on every path.
- **The output device is never switched** — so there is no device state to save, restore, or get
  wrong. This replaced the previous "always restore the output on exit" invariant.
- **The tap always excludes the local snapclient** — the engine refuses to start rather than run
  an un-excluded tap, because the failure mode is a feedback loop, not silence.
- **macOS-only audio path, macOS 14.2+** — CoreAudio process taps + snapcast; never assume
  Linux/Windows for it. Verifiable only on a real Mac.
- **No telemetry** — fully local; nothing phones home.

## cross-cutting
- **Configuration** — everything tunable is an `AIRTONE_*` env var defined in `scripts/common.sh`
  (home, device names, buffer, codec, chunk, ports, sample rate, FIFO, Snapweb version), not Go
  constants.
- **Exit / signal handling** — three layers, because the cost of failure is a muted Mac:
  `systemtap.swift` traps SIGINT/SIGTERM/SIGHUP and treats a closed stdout as a stop signal;
  `cmd/airtone/main.go` (Instant) and `internal/tui` install their own SIGHUP handlers and call
  an idempotent teardown on every quit path.
- **State to the phone** — now-playing is **pushed** over SSE (`internal/remote`), not polled;
  snapserver status is **pulled** over JSON-RPC.
- **Self-contained distribution** — the binary embeds its own engine and re-extracts its own
  scripts/assets on every run, so an upgraded binary always ships a matching engine.

## starting points
- Add/adjust a CLI command → `cmd/airtone/main.go`.
- Change audio behavior (capture, routing, sync) → `scripts/*.sh`, `assets/systemtap.swift` and
  `assets/snapserver.conf.tmpl` (never Go).
- Touch the dashboard/settings/doctor UI → `internal/tui`.
- Phone player / now-playing / controls → `internal/remote` (+ `internal/instant` for WebRTC).
- Read snapserver status → `internal/rpc`.
- Add a runtime dependency → register a check in `internal/doctor`.

## why
Component rationale is the "runtime components & rationale" table above; locked product/design
decisions live in **CLAUDE.md → WHY**. New cross-cutting decisions, as they arise, → a
`DECISIONS.md` (not yet created). No rationale is duplicated here.
