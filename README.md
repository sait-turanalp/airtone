<h1 align="center">AirTone 🔊</h1>

<p align="center"><b>Play your Mac's sound on your phone — in sync, gapless, no app to install.</b></p>

<p align="center">One command on the Mac. One tap on the phone. Your music, around the whole house — together.</p>

<p align="center">
  <a href="https://github.com/sait-turanalp/airtone/releases"><img alt="Release" src="https://img.shields.io/github/v/release/sait-turanalp/airtone?sort=semver&color=2ea44f"></a>
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-yellow.svg"></a>
  <img alt="Platform: macOS 11+" src="https://img.shields.io/badge/platform-macOS%2011%2B-black?logo=apple&logoColor=white">
  <img alt="Built with Go" src="https://img.shields.io/badge/built%20with-Go-00ADD8?logo=go&logoColor=white">
  <img alt="No app required" src="https://img.shields.io/badge/phone-no%20app%20required-7c3aed">
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#quick-start">Quick start</a> •
  <a href="#the-phone">The phone</a> •
  <a href="#how-it-works">How it works</a> •
  <a href="#comparison">Comparison</a> •
  <a href="#license">License</a>
</p>

<p align="center">
  <img src="ss.png" alt="AirTone — the phone web remote, an iOS-26 liquid-glass player" width="460">
</p>

AirTone turns your Mac into a tiny local audio broadcaster. Whatever's playing — Spotify, YouTube, any browser tab — keeps playing on the Mac **and** streams to your phone at the same time, in sync. The phone just opens a web page (scan a QR code) — **nothing to install** — and doubles as a polished remote for your Mac's playback.

> **Why this exists.** macOS has no built-in way to send arbitrary *system* audio to a phone's browser, in sync — AirPlay only targets Apple speakers and devices, and no open-source project packaged this end to end. The hard part isn't the network; it's **source-timing drift**: a naive capture drops samples and forces the sync engine to re-lock several times a second, so the audio stutters. AirTone fixes it at the root — **gapless `sox` capture with BlackHole pinned as the master clock** — and hides the whole pipeline behind one command. The full diagnostic story is in [docs/troubleshooting.md](docs/troubleshooting.md).

## Features

- 🔊 **Listen in sync** — your Mac's audio on any phone and any number of speakers, sample-accurate, all locked to one clock.
- 📱 **No app, ever** — the phone just opens a web page (scan the QR). iPhone, Android, anything with a browser.
- 🎛️ **Your phone is the remote** — a modern, **iOS-26-style "liquid glass"** web player: live album art, an artwork-derived ambient background, transport, drag-to-scrub, and volume — for *any* app, with **no permissions**.
- 🎵 **Gapless & drift-free** — the capture problem that makes naive setups stutter, solved at the source.
- ⚡ **One command** — `brew install`, a guided `setup`, and a built-in `doctor` that checks everything for you.
- 🦾 **Single Go binary** — the engine, web UI, and control helpers are embedded; the playback helper needs no permissions and works offline.

## Quick start

```bash
# 1) AirTone (also pulls snapcast, sox, switchaudio-osx)
brew install sait-turanalp/airtone/airtone

# 2) the BlackHole driver — one-time (needs admin + a reboot)
brew install --cask blackhole-2ch

# 3) set up once, then launch
airtone setup
airtone
```

Then play audio on the Mac and **scan the QR code** with your phone. That's it.

## Usage

```text
airtone            # interactive TUI — live dashboard + QR
airtone party      # stream synced audio to phones/speakers   [alias: start]
airtone instant    # open the phone remote (now-playing + controls)
airtone status     # live status
airtone doctor     # check your setup, fix issues
airtone stop       # stop streaming and restore your output
```

## The phone

Open the page (or scan the QR) — there is nothing to install on the phone.

- **Listen** — a synced browser player. Add as many devices as you like; they all play together on a shared clock.
- **Control** — a full-screen **now-playing remote** styled like iOS-26 liquid glass: high-res album art (looked up via the iTunes catalogue), a live ambient background drawn from the cover, transport, a drag-to-scrub timeline, and a volume slider. It drives **any** media app (Spotify, Apple Music, a browser tab) through Apple's MediaRemote — **no app, no permission**, with the helper bundled so it works offline.

## How it works

```mermaid
flowchart TD
  A["🎵 Any app on the Mac<br/>Spotify · YouTube · a browser tab"] --> B["AirTone Sync<br/>Multi-Output device"]
  B --> C["🔊 Mac speakers<br/>you still hear it"]
  B --> D["BlackHole<br/>virtual cable · master clock"]
  D --> E["sox<br/>gapless CoreAudio capture"]
  E --> F["snapserver<br/>Opus · buffered · sample-synced"]
  F --> G["📱 Phone browser<br/>scan a QR · no app"]
```

- **BlackHole** — a virtual audio cable, so system audio can be captured.
- **Multi-Output** — sends sound to the speakers *and* BlackHole at once, so the Mac is never silenced.
- **sox** — reads BlackHole gaplessly (where naive `ffmpeg` setups drop samples and stutter).
- **snapserver** — encodes and keeps every client locked to one shared clock.

Full design notes: [docs/architecture.md](docs/architecture.md).

## Comparison

| | **AirTone** | AirPlay (built-in) | Snapcast (raw) |
|---|:---:|:---:|:---:|
| Mac audio → **any phone**, in the browser | ✅ | ❌ Apple devices only | ⚠️ DIY, no capture |
| No app on the phone | ✅ | ✅ *(Apple only)* | ⚠️ |
| Sample-accurate multi-room sync | ✅ | ✅ *(AirPlay 2)* | ✅ |
| Phone as a remote (now-playing · transport · volume) | ✅ | ❌ | ❌ |
| Modern, iOS-26-style web player | ✅ | — | ❌ |
| Gapless, drift-free capture out of the box | ✅ | ✅ | ⚠️ you wire it |
| One command to install + guided setup | ✅ | n/a | ❌ |
| Open source | ✅ MIT | ❌ | ✅ GPL |

## Requirements

- macOS 11+ (Apple Silicon recommended)
- [BlackHole 2ch](https://github.com/ExistentialAudio/BlackHole) audio driver
- `snapcast` + `sox` (pulled by Homebrew); `ffmpeg` for `instant`
- A phone on the **same Wi-Fi** as the Mac

## Honest limitations

- **The phone can't be a native AirPlay receiver** — an Apple restriction; AirTone uses a local web stream instead.
- **BlackHole needs admin + a reboot** to install (the setup wizard guides you).
- **Sync means latency** — everyone plays a little behind "live," together. Tunable: lower is snappier, higher is smoother.
- **The stream is unencrypted on your LAN** — fine for home; don't expose it to untrusted networks.

## Roadmap

- Low-latency listening in the phone player (the WebRTC engine is already in place)
- Menu-bar GUI
- Multi-room presets & per-client control
- Linux / Windows server side

## Contributing

Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md). The reasoning behind every design choice lives in [docs/troubleshooting.md](docs/troubleshooting.md).

## License

[MIT](LICENSE) © Sait Turanalp
