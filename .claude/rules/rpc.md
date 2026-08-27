---
paths:
  - "internal/rpc/**"
---
# Snapcast JSON-RPC client

## Footguns (read first)
- **Read-only status, not control of the audio path.** This client queries snapserver; it does
  not drive audio (engine boundary). Keep it a thin reader.
- Raw TCP + a hand-written JSON-RPC frame, newline-terminated (`client.go:45`). Preserve the
  framing if you add methods.

## Conventions
- `GetStatus(addr)` (`client.go:37`) calls `Server.GetStatus` and returns a flattened snapshot
  (streams + groups, `client.go:83`–`86`). Add new queries as small typed wrappers like this.
- snapserver's control port comes from the engine config (`AIRTONE_*`), not hardcoded here.

## Verify
- `go vet ./... && go build ./...`. Real Mac: with `airtone party` running, `airtone status`
  shows live stream/group state.

## Pointers
- `internal/rpc/client.go`. Status consumers: `cmd/airtone/main.go:83` `runStatus`,
  `internal/tui/tui.go:196` `pollStatus`.
