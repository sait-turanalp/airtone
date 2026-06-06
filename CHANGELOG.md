# Changelog

All notable changes to this project are documented here.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/).

## [0.3.0]

### Added
- **Remote control**: drive the Mac's music from the phone — now-playing, play/pause/next/prev, and a volume slider — on the Instant page and a standalone `/remote` page.
- Bundled, permission-free playback control via Apple's MediaRemote (embedded mediaremote-adapter, BSD-3); works offline and with any media app.
- CoreAudio volume helper that controls the built-in output directly (works while the "AirTone Sync" aggregate is the default device).

## [0.2.0]

### Added
- **Two modes**: `party` (multi-device synced playback; alias `start`) and `instant` (low-latency WebRTC, ~tens of ms, browser-based, no app).
- Instant mode: pion/WebRTC server with a live on-page latency readout (RTT + jitter buffer); requires `ffmpeg`.
- TUI mode switch (`m`) between Party and Instant.

### Changed
- Default Party buffer lowered from 4s to 1s.

## [0.1.0]

### Added
- Project scaffold: Go module, CLI entrypoint, MIT license, CI.
- Proven audio engine: BlackHole + Multi-Output (BlackHole master) + `sox` gapless capture + `snapserver` serving Snapweb.
- Phone connects via browser (Snapweb) — no app required.
- Headless CLI: `setup`, `start`, `stop`, `status`, `doctor`, `version`.
- Interactive TUI (Bubble Tea): live dashboard with QR code, doctor checklist with one-key setup, and latency profiles.
- Docs: architecture overview and a troubleshooting guide distilled from the build.
- Distribution: GoReleaser config + Homebrew tap (`brew install sait-turanalp/airtone/airtone`).
