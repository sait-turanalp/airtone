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

## Requirements

- macOS 11+ (Apple Silicon recommended)
- [BlackHole 2ch](https://github.com/ExistentialAudio/BlackHole) audio driver
- `snapcast` and `sox` (installed via Homebrew)
- A phone on the **same Wi-Fi** as the Mac

## Install

> Homebrew tap is planned — `brew install airtone` is coming.

For now, build from source (see [CONTRIBUTING](CONTRIBUTING.md)).

## Usage

```
airtone            # launch the interactive TUI
airtone start      # start the bridge (headless)
airtone status     # live status
airtone doctor     # diagnose your setup
airtone stop       # stop and restore audio output
```

Start it, play music on the Mac, and **scan the QR code with your phone** — the browser opens the synced stream.

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
