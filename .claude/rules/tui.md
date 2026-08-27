---
paths:
  - "internal/tui/**"
---
# TUI — Bubble Tea dashboard

## Footguns (read first)
- **Tear the pipeline down on EVERY quit path.** `fireTeardown` runs `engine.Stop`, which kills
  snapserver, snapclient and — the load-bearing part — the system tap. Party mode's tap mutes the
  Mac at the source, so a missed teardown leaves the Mac silent. It runs on normal quit, on the
  quit key, and from a **SIGHUP goroutine** for a closed terminal. `engine.Stop` is idempotent, so
  calling it on a path that started nothing is free. Any new exit path MUST call it — this is the
  invariant several commits already fixed; don't regress it.
- No audio device is ever switched, so there is nothing to remember or restore. If you find
  yourself reaching for `SwitchAudioSource`, you are re-introducing the old design.

## Conventions
- Elm architecture: `model` + `Init`/`Update`/`View`. Entry: `Run()` (`tui.go:149`).
- **Theme-driven + responsive** — styles live in `styles.go`, sizes come from `tea.WindowSizeMsg`
  (`tui.go:226`); key bindings in `keys.go`. Don't hardcode colors or widths.
- Latency preset reads `AIRTONE_BUFFER` (`tui.go:174`, `:401`).
- One sub-updater per screen: `updateHome` (`tui.go:294`), `updateSettings` (`tui.go:390`).
  Engine actions are `tea.Cmd`s: `startEngine`/`stopEngine`/`runSetup`/`restartEngine`
  (`tui.go:201`–`216`); status via `pollStatus` (`tui.go:196`).

## Verify
- `go vet ./... && go build ./...`. Real Mac: launch `airtone`, exercise dashboard/settings/
  doctor, then quit via **q, Ctrl+C, and by closing the terminal** — after each, the Mac's own
  audio must be audible again and `pgrep -f airtone-tap` must find nothing.

## Pointers
- `internal/tui/tui.go` (model/update/view + teardown), `styles.go` (theme), `keys.go` (bindings).
- Teardown mirrors Instant mode in `cmd/airtone/main.go` (`runInstant`: SIGHUP + context cancel).
