# Contributing to AirTone

Thanks for your interest! AirTone is a young project and contributions are welcome.

## Building from source

Requirements: Go 1.22+, and the runtime dependencies for actually running the bridge.

```bash
# runtime deps (macOS)
brew install snapcast
xcode-select --install   # the system tap is compiled at setup

# build
git clone https://github.com/sait-turanalp/airtone.git
cd airtone
go build ./cmd/airtone
./airtone version
```

## Project layout

```
cmd/airtone/      CLI/TUI entrypoint
embedfs.go        //go:embed of scripts/ + assets/ — the self-contained engine
internal/
  engine/         extracts the embedded engine and shells out to it
  tui/            Bubble Tea screens (dashboard, settings, doctor)
  rpc/            Snapcast JSON-RPC client (live status)
  doctor/         readiness checks
  instant/        Instant-mode WebRTC server (pion)
  remote/         phone control layer: now-playing, artwork, volume, transport
scripts/          the engine: common.sh · setup.sh · start.sh · stop.sh
assets/           snapserver.conf template, systemtap.swift (the capture path)
docs/             architecture & troubleshooting
```

## Architecture rule

The shell/Swift/snapserver pipeline is the **proven engine**. Go code orchestrates it and reads status over Snapcast's JSON-RPC — it does not reimplement the audio path. Keep that boundary.

## Testing

- **Audio path:** can only be verified on a real Mac (CoreAudio process taps are host-only — containers can't test them). Run the bridge and confirm the Mac and a phone get gapless audio, together.
- **Everything else:** `go vet ./...`, `go test ./...`, and `shellcheck` on shell scripts must pass. Pure plumbing can be smoke-tested with a synthetic PCM source feeding snapserver.

## Commit messages

Keep them concise and descriptive. Do **not** add `Co-Authored-By` trailers.

## License

By contributing, you agree your contributions are licensed under the [MIT License](LICENSE).
