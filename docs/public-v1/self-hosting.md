# OpenTunnel Public V1 Self-Hosting

OpenTunnel v1 runs as a self-hosted relay plus temporary clients downloaded from that relay's `/cli` endpoint. The relay stores no sessions, commands, outputs, invites, or audit logs. Active tunnel state exists only in memory.

## Build

From the repository root:

```bash
go build -o opentunnel ./cmd/opentunnel
```

## Run A Local Relay

For local testing:

```bash
./opentunnel relay \
  --listen 127.0.0.1:8080 \
  --public-url http://127.0.0.1:8080 \
  --artifact-path ./opentunnel \
  --version dev
```

For a public relay, set `--public-url` to the HTTPS origin users will fetch:

```bash
./opentunnel relay \
  --listen :8080 \
  --public-url https://relay.example.com \
  --artifact-path /opt/opentunnel/opentunnel \
  --version v1
```

Terminate TLS in front of the relay with your normal reverse proxy or load balancer. The relay process expects the public origin to be HTTP or HTTPS and does not require a database or Redis.

## Public Host Command

Users start a foreground host session with:

```bash
curl -fsSL https://relay.example.com/cli | sh -s -- create
```

The session stays open until Ctrl+C, idle timeout, relay failure, or process exit.

## Public Client Command

The host prints an agent prompt containing commands like:

```bash
curl -fsSL https://relay.example.com/cli | sh -s -- exec \
  --invite '<invite>' \
  -- '<COMMAND>'
```

The invite contains the information the client needs to connect. The user-facing command does not include a relay flag.

## Artifact Flags

- `--artifact-path` points to the binary served by `/cli/bin/opentunnel-<version>-<platform>`.
- `--version` becomes part of the artifact URL and cache key.
- `--public-url` is the relay origin embedded into the bootstrapper.

The relay serves only the configured artifact path and its checksum. It does not expose arbitrary files.
