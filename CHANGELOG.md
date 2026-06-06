# Changelog

All notable changes to this project are documented here.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added
- Project scaffold: Go module, CLI entrypoint, MIT license, CI.
- Proven audio engine: BlackHole + Multi-Output (BlackHole master) + `sox` gapless capture + `snapserver` serving Snapweb.
- Phone connects via browser (Snapweb) — no app required.
- Headless CLI: `setup`, `start`, `stop`, `status`, `doctor`, `version`.
- Interactive TUI (Bubble Tea): live dashboard with QR code, doctor checklist with one-key setup, and latency profiles (Low / Balanced / Smooth).
- Docs: architecture overview and a troubleshooting guide distilled from the build.
- **Two modes**: `party` (multi-device synced playback, default ~1s buffer; alias `start`) and `instant` (low-latency WebRTC, ~tens of ms, browser-based, no app).
- Instant mode: pion/WebRTC server with a live on-page latency readout (RTT + jitter buffer); needs `ffmpeg`.
- TUI mode switch (`m`) between Party and Instant.
- Default Party buffer lowered from 4s to 1s.
