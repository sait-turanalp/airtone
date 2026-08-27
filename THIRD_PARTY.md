# Third-party components

AirTone is licensed under the GNU General Public License v3.0. It bundles and depends on the following.

## Bundled (embedded in the binary)

- **mediaremote-adapter** — BSD-3-Clause, © 2025 Jonas van den Berg and contributors.
  <https://github.com/ungive/mediaremote-adapter>
  Embedded as `internal/remote/mediaremote-adapter.tgz` and used to read and
  control the macOS "Now Playing" source. Its full license is included inside
  that archive.
- **color-thief** — MIT, © Lokesh Dhakar.
  <https://github.com/lokesh/color-thief>
  Embedded as `internal/remote/colorthief.umd.js`; extracts the album art's
  dominant colours for the player's ambient background (served locally, offline).

## Credits (technique / API, no code bundled)

- **frigopedro/Apple-Music-Background** — the dominant-colour-grid ambient
  background technique was reimplemented (in vanilla JS/CSS) from this project.
  <https://github.com/frigopedro/Apple-Music-Background>
- **iTunes Search API** — queried at runtime to fetch high-resolution album art
  by title/artist. Public, no key; falls back to the local thumbnail offline.

## Runtime dependencies (installed via Homebrew, not linked into the binary)

- **snapcast** (GPL-3.0) — synced multi-room streaming engine
- **ffmpeg** (LGPL/GPL) — Opus encoding for Instant mode
- **Snapweb** (GPL-3.0) — the browser player served (unmodified) in Party mode

These are invoked as separate programs; their source is not incorporated into
AirTone's GPL-3.0-licensed code.
