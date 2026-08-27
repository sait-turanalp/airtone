# Changelog

All notable changes to this project are documented here.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Changed
- **System audio is now captured with a CoreAudio process tap** instead of the BlackHole driver
  and an "AirTone Sync" Multi-Output device. No driver install, no admin password, no reboot —
  and AirTone never switches your output device, so AirPods and external DACs just keep working.
  Requires macOS 14.2+ and the Xcode command line tools.
- **Party mode now plays on the Mac through a local snapclient.** The tap mutes the Mac's own
  output and the Mac joins its own stream as a client, so the Mac and every phone share one
  clock instead of the Mac running a full buffer ahead. A phone next to the Mac no longer echoes.
- Default buffer lowered from 1000 ms to 500 ms (`AIRTONE_BUFFER`).

### Removed
- Dependencies on `sox`, `switchaudio-osx` and the BlackHole 2ch cask.
- The output-device save/restore machinery — there is no longer a device to restore.

## [0.3.0]

### Added
- **Remote control**: the Instant-mode page is now a full-screen "now playing" remote — drive the Mac's music from the phone with transport (play/pause/next/prev), a scrubber, and a volume slider.
- **A-tier player UI**: high-res album art (iTunes lookup, with a graceful placeholder when none exists), an artwork-derived ambient background, a frosted-glass card, crossfading covers, and elastic drag-to-scrub sliders with instant, live response.
- Live now-playing over Server-Sent Events (no polling); volume applied instantly via a persistent CoreAudio helper.
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
