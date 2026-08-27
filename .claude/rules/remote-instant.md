---
paths:
  - "internal/remote/**"
  - "internal/instant/**"
---
# Remote + Instant — WebRTC phone player & control layer

## Footguns (read first)
- **Now-playing comes from `mediaremote-adapter` (BSD-3, credited).** It's extracted to
  `~/.airtone/remote/mediaremote-adapter` (`remote.go:56`–`58`). Attribution lives in
  `THIRD_PARTY.md` — keep it when touching this code.
- **Transport (play/pause/next) is app-dependent** — some sources don't honor it. Don't assume
  every control works; the UI flips optimistically over SSE, so reflect real state when known.
- **Artwork is best-effort.** Browser sources return the app icon, not cover art
  (`artwork.go:106` `isBrowserBundle`) → fall back to an iTunes lookup (`artwork.go:119`).
- **Instant needs `ffmpeg`** (Opus encoding) on top of the Party runtime deps.

## Conventions
- Control routes are registered in one place: `remote.Register(mux)` (`http.go:17`) —
  `/control/events` (SSE), `/control/nowplaying`, `/control/artwork`, `/control/volume`,
  `/control/seek`, `/control/<transport>` (`http.go:18`–`97`). Add new control endpoints here.
- **Push, don't poll** — now-playing changes stream over SSE (`events.go:16` `Events`); heavy
  fields (artwork) are fetched separately and prefetched on track change (`events.go:63`;
  `artwork.go:45` `PrefetchArtwork`).
- Instant server: `internal/instant/server.go` (pion/WebRTC; honors `AIRTONE_HOME`).

## Verify
- `go vet ./... && go build ./...`. Real Mac: open `airtone instant`, load the page on a phone,
  confirm low-latency audio + live now-playing/artwork + working volume.

## Pointers
- `internal/remote/http.go` (routes), `events.go` (SSE), `artwork.go` (cover art), `remote.go`
  (page + adapter paths). `internal/instant/server.go` (WebRTC). Attribution: `THIRD_PARTY.md`.
