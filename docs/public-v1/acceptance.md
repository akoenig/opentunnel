# OpenTunnel Public V1 Acceptance Mapping

This document maps `plan.md` public v1 requirements to the current implementation state.

## Public UX

| Requirement | Status |
| --- | --- |
| Host starts with `curl -fsSL <relay>/cli \| sh -s -- create` | Implemented and manually verified. |
| Client executes with `curl -fsSL <relay>/cli \| sh -s -- exec --invite '<invite>' -- '<command>'` | Implemented and manually verified. |
| No user-facing relay flag in public UX | Implemented. Relay origin comes from `/cli` bootstrap context or invite. |
| Temporary CLI cached during session | Implemented. Cache lives under a private temp cache path and cache hits are checksum-verified. |

## Relay Privacy

| Requirement | Status |
| --- | --- |
| Relay persists no sessions or command data | Implemented. Relay state is in memory only. |
| Relay routes opaque encrypted packets | Implemented. Relay forwards binary frames and does not decrypt command traffic. |
| Relay cannot read command, output, exit code, plaintext host-provided application metadata, or `clientSecret` | Implemented by the secure channel and tunnel protocol design. Relay-visible routing, session, timing, size, and network metadata remain visible. |

## Session Model

| Requirement | Status |
| --- | --- |
| One active client per tunnel | Implemented. |
| One active command at a time | Implemented. |
| Foreground `create` process owns lifetime | Implemented. |
| Ctrl+C closes the tunnel | Implemented. |
| Idle timeout closes forgotten sessions | Implemented. |
| Command timeout and process cleanup | Implemented. |
| Output limit and truncation | Implemented. |

## Documentation And Non-Goals

| Requirement | Status |
| --- | --- |
| Self-hosting guidance | Documented in `docs/public-v1/self-hosting.md`. |
| Trust-boundary documentation | Documented in `docs/public-v1/security.md`. |
| Non-goals documented | Documented in `docs/public-v1/non-goals.md`. |
| Same-origin checksum described only as corruption detection | Documented in `docs/public-v1/security.md`. |

## Explicit V1 Exclusions

Accounts, dashboards, package-manager distribution, install-to-system flows, MCP, raw SSH, PTY, interactive stdin, file transfer, approval workflows, multiple clients, concurrent commands, persistent relay state, and persistent audit logs are excluded from v1.
