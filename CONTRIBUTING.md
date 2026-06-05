# Contributing to AirTone

Thanks for your interest! AirTone is a young project and contributions are welcome.

## Building from source

Requirements: Go 1.22+, and the runtime dependencies for actually running the bridge.

```bash
# runtime deps (macOS)
brew install snapcast sox
brew install --cask blackhole-2ch   # needs admin + reboot

# build
git clone https://github.com/sait-turanalp/airtone.git
cd airtone
go build ./cmd/airtone
./airtone version
```

## Project layout

```
cmd/airtone/      CLI/TUI entrypoint
internal/
  tui/            Bubble Tea screens (wizard, dashboard, settings, doctor)
  pipeline/       starts/stops snapserver + sox, manages processes
  rpc/            Snapcast JSON-RPC client (live status & control)
  installer/      BlackHole / Snapweb / Multi-Output setup
  config/         config + profiles
assets/           snapserver.conf template, Multi-Output helper
docs/             architecture & troubleshooting
```

## Architecture rule

The shell/Swift/sox/snapserver pipeline is the **proven engine**. Go code orchestrates it and reads status over Snapcast's JSON-RPC — it does not reimplement the audio path. Keep that boundary.

## Testing

- **Audio path:** can only be verified on a real Mac (CoreAudio/BlackHole/sox are host-only — containers can't test them). Run the bridge and confirm a phone gets gapless, synced audio.
- **Everything else:** `go vet ./...`, `go test ./...`, and `shellcheck` on shell scripts must pass. Pure plumbing can be smoke-tested with a synthetic PCM source feeding snapserver.

## Commit messages

Keep them concise and descriptive. Do **not** add `Co-Authored-By` trailers.

## License

By contributing, you agree your contributions are licensed under the [MIT License](LICENSE).
