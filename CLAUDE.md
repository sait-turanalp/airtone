<!-- maintainer note: hand-crafted agent-context spine. Edit policy: every line must pass
     "would the agent err without it?" — else cut. Area depth → .claude/rules/*. Last reviewed: 2026-06-15. -->

# AirTone — agent context

macOS-only Go CLI/TUI that turns a Mac into a synced, multi-room audio bridge: system audio →
phones/speakers, played in the **browser, no app**. Two modes, one self-contained binary. GPL-3.0.

**Mental model (the flow):**
```
system audio → CoreAudio process tap (no driver, no device switching)
  ├─ Party:   → snapserver → snapclient on the Mac + Snapweb in the phone browser
  │            (the Mac is a snapcast client too — that is what keeps them in sync)
  └─ Instant: → ffmpeg→Opus → pion/WebRTC → phone browser (low-latency + now-playing/controls)
```
Go **orchestrates** the engine and reads status over Snapcast JSON-RPC. The shell/Swift/
snapserver pipeline is the **proven engine** — Go never reimplements the audio path.

## North-star (raise these proactively; the user may not in the moment)
- **Efficiency + performance first** — this is realtime audio; build light/responsive from the
  start, never "make it work then optimize".
- **Maintainable structure** — keep orchestration thin, packages single-responsibility, reuse
  over copy-paste. The engine is shell; Go stays a thin, clean orchestrator.

## WHAT — stack & layout
- **Go 1.26** (`go.mod`; CI builds on 1.22 — keep code 1.22-compatible). TUI: charmbracelet
  bubbletea/bubbles/lipgloss. Instant: pion/webrtc/v4. QR: qrterminal.
- **Entry files:** `cmd/airtone/main.go` (subcommand dispatch: setup/party·start/instant/stop/
  status/doctor/version) · `embedfs.go` (`//go:embed scripts assets` → the self-contained
  engine) · `internal/engine/engine.go` (extracts & shells out to the engine).
- **Packages:** `internal/engine` (run the embedded engine, device switching) · `internal/tui`
  (Bubble Tea dashboard/doctor/settings) · `internal/remote`+`internal/instant` (WebRTC + HTTP/
  SSE phone player, now-playing, artwork) · `internal/rpc` (Snapcast JSON-RPC) · `internal/doctor`
  (readiness checks). `scripts/` = setup/start/stop/common · `assets/` = snapserver.conf.tmpl +
  systemtap.swift.

## WHY — locked decisions
- **Embedded engine** (`//go:embed`) → one self-contained binary, no repo checkout at runtime;
  re-extracted to `~/.airtone/engine` each run (an upgraded binary always ships its own engine).
- **Browser player, no app** — phones connect to Snapweb (Party) or a WebRTC page (Instant).
- **CoreAudio process tap for capture** (macOS 14.2+) — no BlackHole, no admin password, no
  reboot, and the user's output device is never switched. Replaced the Multi-Output + sox path.
- **Party mode runs a local snapclient** — the Mac plays the snapcast stream instead of its own
  audio (the tap mutes at source), so Mac and phone share one clock. The tap excludes snapclient
  or it would capture its own playback.

## Immutable invariants (NEVER break — detail in the owning rule)
1. **Engine boundary** — Go orchestrates the shell/Swift/snapserver engine via JSON-RPC; it
   NEVER reimplements the audio path. → `.claude/rules/audio-engine.md`
2. **Destroy the tap on exit** — every mode, every path (normal quit, Ctrl+C, **SIGHUP / closed
   terminal**, dead parent) tears the tap down; idempotent. A leaked tap leaves the Mac muted at
   the source. No output device is ever switched, so there is nothing to restore.
   → audio-engine + tui rules
3. **macOS-only audio path** — CoreAudio process taps + snapcast; never assume Linux/Windows for
   it (Go logic may stay portable).
4. **Offline / no telemetry** — no analytics, tracking, or phone-home. Fully local.

## HOW — verify gates (main must stay green)
- `go vet ./...` · `go build ./...` · `go test ./...` · `shellcheck scripts/*.sh`
- Tap probes that need no phone: `airtone-tap --probe` (prints `48000:16:2`) and a round-trip
  capture that excludes the source process — if it still records audio, snapclient is playing
  the stream back, i.e. the whole chain is closed.
- **Audio path can only be verified on a real Mac** (CoreAudio taps are host-only —
  CI/containers can't test it). Run the bridge, confirm a phone gets gapless, synced audio.
- CI: Go on `macos-latest`, shellcheck on ubuntu (`.github/workflows/ci.yml`).

## Architecture
Atlas = **`ARCHITECTURE.md`** (root): codemap · boundaries · invariants · components · ports.
Read it for the shape; this spine is the legend. Troubleshooting: `docs/troubleshooting.md`.

## Git
- Conventional commits **with scopes**: `feat(remote):`, `fix(tui):`, `docs:`, `ci:`, `perf:`.
- **Commit directly to main** (solo). CI runs on push. Large multi-phase work *may* use a
  short-lived `feat/…` branch (as PR #1/#2 did) — optional; keep main green.
- **NEVER add `Co-Authored-By`** trailers (also enforced by a global PreToolUse hook).

## Working with this repo
- **Context discipline (locked):** gather via `ctx_batch_execute`, follow up via `ctx_search`,
  derive via `ctx_execute` — raw bytes stay in the sandbox. Reference `file:line`, don't paste
  code. The source tree + docs are indexed — **search the index before re-reading files**.
- **Plans carry no code** — decisions, steps, gates, gotchas only.
- **Layer boundaries (single source of truth):** this CLAUDE.md = constitution/legend ·
  `ARCHITECTURE.md` = atlas (the shape) · `docs/plans/<topic>.md` = one effort's HOW
  (temporary) · `.claude/rules/*` = domain depth (lazy, path-scoped).
- **plan-doc trigger:** multi-phase effort → distill to `docs/plans/<topic>.md` at
  architecture-lock, before code. Single-phase → a one-liner suffices.
- **arch-doc reflex:** touch `ARCHITECTURE.md` only when implementation is **done** AND the
  change is **architectural** (new boundary/component/invariant). Routine features → no churn.

## Footguns (1-liners; detail in the owning rule)
- **Tap teardown is load-bearing** — a leaked tap = a permanently muted Mac. Test SIGHUP, a closed
  terminal and a dead parent (EPIPE on stdout), not just a clean quit. → audio-engine + tui rules.
- **snapclient registers with CoreAudio only once it is playing** — and it only plays once the
  stream carries data, which needs the tap. The tap breaks that deadlock by streaming silence
  until the process it must exclude appears; never "just skip the exclusion" (feedback loop).
- **Never tap a process that is on a call.** Muting an app and replaying its audio 500 ms later
  from snapclient destroys the app's echo cancellation — its reference no longer matches what the
  mic hears, so the person on the other end hears themselves. The tap excludes every process with
  a live input stream (`kAudioProcessPropertyIsRunningInput`) and rebuilds itself when that set
  changes. Match on the microphone, never on an app-name list: it covers a browser tab and a
  native app identically. Confirmed in a live Gather call, 2026-08-27.
- **macOS snapclient has no output-device flag** — it always plays to the system default. Fine
  here (we never switch it), but it rules out any design needing snapclient on a chosen device.
- **Don't run `goreleaser release` locally** — release is CI-only on a `v*` tag; a local run
  double-releases the same tag. → `.github/workflows/release.yml`.
- **Audio = host-only** — never claim the audio path is verified from CI/a container.
- **Tap needs macOS 14.2+** — `AudioHardwareCreateProcessTap` does not exist below it; `doctor`
  gates on the real `sw_vers` number.

## Maintenance & lifecycle
- **On compaction, always preserve:** modified file list, pending deploy/release state, and the
  exact verify commands last run.
- **Milestone capture:** when a feature/`feat/` branch/session wraps, proactively ask whether to
  capture hard-won learnings (footguns, invariants) into this file / a rule / memory.
- **Self-improving:** repeated mistake → add a targeted rule + emphasize it; prune what the model
  already does right; keep this spine contradiction-free.
