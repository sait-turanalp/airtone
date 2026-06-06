# AirTone

**Stream your Mac's system audio to your phone — in sync, gapless, no app required.**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![macOS](https://img.shields.io/badge/macOS-11%2B-black?logo=apple)](#requirements)

> ⚠️ Pre-release / work in progress. The audio engine works today; the CLI and TUI are being built in the open.

AirTone turns your Mac into a tiny local audio broadcaster. Whatever is playing on your Mac — Spotify, YouTube, a browser tab — keeps playing on the Mac speakers **and** is streamed to your phone at the same time. Your phone just opens a web page (scan a QR code). **No app to install on the phone.**

## Why

There is no ready-made package that does this cleanly on macOS. The pieces exist (a virtual audio device, a sync engine, a capture tool) but wiring them into something gapless and stutter-free takes real work — including avoiding the subtle sample-drop and clock-resync problems that make naive setups stutter. AirTone packages the proven, working combination behind one command and a friendly TUI.

## How it works

```
┌──────────────────────────────────────────────────────────┐
│  MAC                                                       │
│  🎵 Music (Spotify / YouTube / anything)                   │
│        │                                                   │
│        ▼                                                   │
│  "Sync Out" (Multi-Output device)                          │
│        │                  │                                │
│        ▼                  ▼                                │
│  🔊 Mac speakers     🕳  BlackHole (virtual, master clock) │
│  (instant)                │                                │
│                           ▼                                │
│                      sox  (gapless CoreAudio capture)      │
│                           │                                │
│                           ▼                                │
│                      snapserver  (opus, buffered, synced)  │
│                           │  serves Snapweb on :1780       │
└───────────────────────────┼───────────────────────────────┘
                            │
                       📶 Local Wi-Fi
                            │
                            ▼
              ┌──────────────────────────┐
              │  📱 Phone — just a browser │
              │  scan QR → Snapweb plays   │
              └──────────────────────────┘
```

- **BlackHole** — a virtual audio cable so the system audio can be captured.
- **Multi-Output** — splits sound to both the speakers and BlackHole, so the Mac is never silenced.
- **sox** — reads BlackHole gaplessly (the part where naive `ffmpeg` setups drop samples and stutter).
- **snapserver** — packetizes, encodes, and keeps every client locked to a shared clock; serves the Snapweb player.
- **Phone** — opens the served web page; no native app.

See [docs/architecture.md](docs/architecture.md) for the full design and the reasoning behind each choice.

## Remote control

The Instant page and a standalone **`/remote`** page include a control bar to
drive the Mac's music from your phone — independent of streaming:

- **Now playing** (title / artist), updated live
- **⏮ ⏯ ⏭** transport and a **volume** slider

It works with any media app (Spotify, Apple Music, a browser tab) via Apple's
MediaRemote — no app, no permission, and the helper is bundled so it works
offline. Volume targets the Mac's built-in speakers.

> **next / previous** only act in apps with real track navigation (Spotify,
> Apple Music). A single video (one YouTube clip) has no "next track", so those
> buttons no-op there — play/pause and volume always work.

## Requirements

- macOS 11+ (Apple Silicon recommended)
- [BlackHole 2ch](https://github.com/ExistentialAudio/BlackHole) audio driver
- `snapcast` and `sox` (installed via Homebrew); `ffmpeg` for Instant mode
- A phone on the **same Wi-Fi** as the Mac

## Install

```bash
# 1) AirTone (pulls snapcast, sox, switchaudio-osx automatically)
brew install sait-turanalp/airtone/airtone

# 2) the BlackHole driver (one-time; needs admin + a reboot)
brew install --cask blackhole-2ch

# 3) one-time setup, then launch
airtone setup
airtone
```

Prefer to build from source? See [CONTRIBUTING](CONTRIBUTING.md).

## Usage

```
airtone            # launch the interactive TUI (switch modes with 'm')
airtone party      # multi-device synced playback (~1s delay)   [alias: start]
airtone instant    # low-latency mode over WebRTC (~tens of ms)
airtone status     # live status
airtone doctor     # diagnose your setup
airtone stop       # stop party mode and restore output
```

Start a mode, play audio on the Mac, and **scan the QR code with your phone** — the browser opens the stream. No app either way.

## Two modes

| | **Party** | **Instant** |
|---|---|---|
| Transport | snapcast (buffered) | WebRTC (adaptive) |
| Latency | ~1s (tunable) | ~tens of ms |
| Multi-device sync | ✅ sample-accurate | ❌ each device independent |
| Best for | speakers around a room, in sync | one phone, lowest delay |

Both serve a browser page (QR) — no app to install. Instant mode additionally needs `ffmpeg` and, on iOS Safari, bottoms out around ~130ms (a Safari limitation — see [docs/troubleshooting.md](docs/troubleshooting.md)).

## Honest limitations

- **The phone can't be a native AirPlay/audio receiver** — that's an Apple restriction. AirTone uses a local web stream instead.
- **BlackHole is required** and its install needs admin rights + a reboot (the setup wizard guides you).
- **There is latency.** Sync means buffering: all listeners play a little behind "live," together. Configurable (lower = snappier, higher = smoother).
- **The stream is unencrypted on your local network.** Fine for home/personal use; don't expose it to untrusted networks.

## Roadmap

- Menu-bar GUI app
- Multi-room presets and per-client control
- Linux/Windows server side

## Contributing

Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md). The diagnostic story behind the design lives in [docs/troubleshooting.md](docs/troubleshooting.md).

## License

[MIT](LICENSE) © Sait Turanalp
