# OpenTunnel

OpenTunnel is an ephemeral, relay-routed, end-to-end encrypted remote command tunnel for AI agents. A host starts one foreground session, copies the generated prompt into an agent, and the agent can run one-off commands through an opaque relay without SSH, inbound firewall rules, accounts, or persistent relay state.

## What It Is

OpenTunnel v1 is built around one temporary host process, one client, one command at a time, and a temporary CLI downloaded from the relay's `/cli` endpoint. The relay routes encrypted packets and stores only active in-memory connection state.

## Status

This repository contains the public v1 self-hosted implementation and release-operations artifacts. It is designed for operators who run their own relay and serve a compatible temporary `opentunnel` binary from that relay.

## Quick Start

Build the CLI:

```bash
go build -o opentunnel ./cmd/opentunnel
```

Run a local relay:

```bash
./opentunnel relay \
  --listen 127.0.0.1:8080 \
  --public-url http://127.0.0.1:8080 \
  --artifact-path ./opentunnel \
  --version dev
```

Start a host session through the public `/cli` path:

```bash
curl -fsSL http://127.0.0.1:8080/cli | sh -s -- create
```

## Public Command Shape

The host prints an agent prompt containing commands like:

```bash
curl -fsSL https://relay.example.com/cli | sh -s -- exec \
  --invite '<invite>' \
  -- '<COMMAND>'
```

The invite is bearer-secret material. Anyone with a valid invite can attempt to connect while the foreground host session is active.

## Verification

Run the standard local verification commands:

```bash
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
rm -f ./opentunnel
```

CI also builds downloadable binaries for:

- `linux-amd64`
- `linux-arm64`
- `linux-armv7`
- `darwin-arm64` Apple Silicon

## Documentation

- [Self-hosting](docs/public-v1/self-hosting.md)
- [Operations](docs/public-v1/operations.md)
- [Security model](docs/public-v1/security.md)
- [Non-goals](docs/public-v1/non-goals.md)
- [Acceptance mapping](docs/public-v1/acceptance.md)

## Security Notes

The relay sees routing/session, timing, size, and network metadata, but command traffic is end-to-end encrypted. Same-origin checksums for `/cli` artifacts detect corruption or mismatch within the trusted relay-origin model; they are not a strong supply-chain security boundary.

## Non-Goals

OpenTunnel v1 does not include accounts, dashboards, package-manager distribution, install-to-system flows, MCP, raw SSH, PTY, interactive stdin, file transfer, approval workflows, multiple clients for one tunnel, concurrent commands, persistent relay state, or persistent audit logs.
