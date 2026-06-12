# OpenTunnel Findings Incorporation Design

## Goal

Incorporate the actionable findings from `findings.md` while preserving the v1 product shape: one temporary host process, one client, one command at a time, in-memory relay state, relay-served temporary CLI, and end-to-end encrypted command traffic.

## Scope

Implement all actionable security, maintainability, dependency, CI, documentation, and repository-hygiene recommendations from `findings.md`, except notes explicitly marked as no action required.

The work is staged so each change group can be verified independently with the standard Go suite.

## Architecture

The relay remains an opaque WebSocket frame router. It will gain operational guardrails: HTTP server timeouts, WebSocket read limits, session caps, reservation expiry, and browser-origin rejection. Tunnel routing metadata moves from URL query parameters into custom WebSocket headers so the static `/tunnel` path no longer exposes session IDs in standard access logs.

The host loop remains responsible for one command per accepted client connection. It will treat pre-command client failures, including handshake failures and early disconnects, as non-fatal session events. Genuine local failures and idle timeout still terminate the host session.

The secure-channel package keeps one production handshake path: the split `NewClientHandshake` / `NewHostHandshake` flow. Test-only helper behavior will move into tests where needed.

## Components

- `cmd/opentunnel`: update Go-facing CLI behavior, relay server timeouts, invite parsing, prompt text, and shared origin validation usage.
- `internal/relay`: add request-header tunnel metadata, browser-origin rejection, WebSocket read limits, session cap and reservation expiry behavior.
- `internal/tunnel`: update dialers to set tunnel headers, harden host error handling, and add only targeted refactoring needed to keep the loop understandable.
- `internal/securechannel`: remove duplicate handshake functions and the unused XX pattern constant while preserving coverage via split-handshake tests.
- `internal/artifact`: use shared relay-origin validation and keep the bootstrap script behavior intact.
- Docs and CI: update examples, security notes, Go versions, vulnerability checks, and lint checks where supported.

## Invite DX

`--invite` remains supported for compatibility, but it is no longer the recommended path in prompts or docs.

The preferred copy/paste flow becomes an environment variable assignment:

```sh
OPENTUNNEL_INVITE='<invite>' opentunnel exec -- '<COMMAND>'
```

For the relay-served bootstrap prompt, the command becomes:

```sh
curl -fsSL https://relay.example.com/cli | OPENTUNNEL_INVITE='<invite>' sh -s -- exec -- '<COMMAND>'
```

This keeps the invite out of the long-lived `opentunnel exec` process argv and avoids exposure through `ps` or `/proc/<pid>/cmdline`. Shell history can still capture the typed assignment, so docs will call that out and recommend safer alternatives on shared machines. `--invite-stdin` will be added for users and automation that need to avoid both argv and shell history.

## Data Flow

Host and client dial `/tunnel` with `OpenTunnel-Role` and `OpenTunnel-Session` headers. The relay validates those headers, reserves the matching slot, upgrades the WebSocket, applies read limits, and forwards only binary frames to the peer. The relay never decrypts command traffic.

The host creates an invite containing relay URL, session ID, host public key, and client PSK. The client obtains the invite from `--invite`, `OPENTUNNEL_INVITE`, or stdin, then performs the split NKpsk0 handshake with the host through the relay.

## Error Handling

Bad client handshakes and early client disconnects are logged and followed by host re-dial, not session termination. Idle timeout still terminates with the existing idle error. Relay resource-limit failures return HTTP errors before upgrade where possible and close oversized WebSocket connections after upgrade.

Invite parsing reports all supported sources in the missing-invite error message. If conflicting invite sources are provided, explicit CLI input wins over environment fallback; stdin is used only when requested.

## Testing

Tests will be updated or added for:

- Rogue client handshake/disconnect does not terminate the host session.
- Tunnel metadata is carried in headers rather than query strings.
- Browser `Origin` WebSocket upgrades are rejected, while non-browser upgrades still work.
- Relay session caps, reservation expiry, and oversized frames.
- Split-handshake secure-channel coverage after deleting duplicate production helpers.
- `OPENTUNNEL_INVITE` fallback and `--invite-stdin` behavior.
- Updated create prompt and documentation examples.

Verification after implementation:

```sh
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
rm -f ./opentunnel
```

CI should additionally run `govulncheck ./...` and `golangci-lint run` once configured.

## Non-Goals

- No accounts, dashboards, persistent relay state, audit logs, raw SSH, PTY, interactive stdin, file transfer, multiple clients, or concurrent commands.
- No command awareness in the relay.
- No removal of `--invite` compatibility.
