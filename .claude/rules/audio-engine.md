---
paths:
  - "internal/engine/**"
  - "internal/doctor/**"
  - "scripts/**"
  - "assets/**"
  - "embedfs.go"
---
# Audio engine — the proven pipeline (boundary rule)

## Footguns (read first)
- **NEVER reimplement the audio path in Go.** The shell/Swift/snapserver pipeline is the proven
  engine; Go only extracts it and shells out. New audio behavior → change the *scripts/assets*,
  not Go. (`internal/engine/engine.go` `run()` → `exec.Command("/bin/bash", …)`.)
- **Destroy the tap on every exit path — a leaked tap leaves the Mac muted.** Party mode's tap
  runs with `--mute`, which silences the Mac at the source; the mute lives exactly as long as the
  tap process. `assets/systemtap.swift` handles SIGINT/SIGTERM/SIGHUP *and* treats a closed stdout
  (EPIPE, i.e. a dead parent) as a stop signal; `scripts/stop.sh` → `kill_pipeline` kills it;
  `internal/tui` `fireTeardown()` and `cmd/airtone/main.go` call that on every quit path. Any new
  teardown path MUST reach the tap. Test SIGHUP and a killed parent, not just a clean quit.
- **Never run the tap without its exclusion.** Party mode's snapclient plays the stream back on
  the Mac; if the tap captured it too, the audio would loop back on itself. The tap *refuses to
  start* rather than fall back to an un-excluded tap — keep that refusal, it is deliberate.
- **The start order is a deadlock, and the fix is the silence pump.** snapclient only registers
  with CoreAudio once it is *playing*, and it only plays once the stream carries data — which
  needs the tap that needs snapclient's process ID. `systemtap.swift` breaks this by streaming
  real-time silence until the excluded process appears, then swapping in the real capture. Don't
  "simplify" that away; measured, snapclient never registers on an idle stream.
- **macOS snapclient has no output-device flag** (`--help` on 0.35: no `-s`/`--soundcard`; the man
  page is a stale Linux copy). It always plays to the system default. Harmless here — we never
  switch the default — but it kills any design needing snapclient on a specific device.
- **The engine is re-materialized every run** (`engine.go` `materialize`) — an upgraded binary
  always ships and extracts its own engine to `~/.airtone/engine`. Don't cache or skip this. The
  compiled tap lives beside it in `~/.airtone/engine/bin` and is rebuilt when the source is newer.
- **`shellcheck scripts/*.sh` must pass** — CI gates it (ubuntu job). Quote vars, keep it portable.

## Conventions
- Data dir = `~/.airtone`, override with `AIRTONE_HOME` (`engine.go` `Home()`); compiled tap at
  `engine.TapBin()`.
- The tap is the single capture path for BOTH modes. Party: `--mute --exclude-pid <snapclient>`.
  Instant: no flags — there is no local snapclient to play the audio back, so the Mac must stay
  audible on its own.
- `--probe` prints `<rate>:16:<channels>` and exits; `start.sh` uses it to tell snapserver the
  device's real format so nothing has to resample.
- **All tunables are `AIRTONE_*` env vars, defined in `scripts/common.sh`:** `AIRTONE_HOME`,
  `AIRTONE_BUFFER`, `AIRTONE_CODEC`, `AIRTONE_CHUNK_MS`, `AIRTONE_HTTP_PORT`, `AIRTONE_FIFO`,
  `AIRTONE_ASSETS_DIR`, `AIRTONE_SNAPWEB_VERSION` (setup). Add tunables here, not as Go consts.
- `doctor.Run()` is the readiness contract: binaries (snapserver/snapclient/swiftc), macOS 14.2+
  for the tap API, the compiled tap, Snapweb. Keep it in sync when you add a runtime dependency.

## Verify
- `go vet ./... && go build ./... && go test ./...` · `shellcheck scripts/*.sh`
- Tap-only checks, no phone needed:
  - `airtone-tap --probe` → `48000:16:2`.
  - Round trip: play a tone, then capture with the *tone's* process excluded. If audio still
    records, it can only be snapclient playing the stream back — the whole chain is closed.
    (Measured 2026-08-27: 440 Hz in, 440.0 Hz out, RMS 0.247 → 0.2477.)
  - Exclusion negative cell: same capture with nothing excluded must be loud, with the source
    excluded must be silence. (Measured: 0.34874 vs 0.00000, identical frame counts.)
- Real Mac only: `airtone setup` → `airtone party` → Mac and phone play together → `airtone stop`
  → the Mac's own audio is audible again. **Test the closed-terminal (SIGHUP) path too.**

## Pointers
- Embed: `embedfs.go` (`//go:embed scripts assets`). Orchestration: `internal/engine/engine.go`.
- Scripts: `scripts/common.sh` (config + `build_tap`/`kill_pipeline`), `setup.sh`, `start.sh`,
  `stop.sh`.
- Assets: `assets/snapserver.conf.tmpl`, `assets/systemtap.swift` (the entire capture path).
