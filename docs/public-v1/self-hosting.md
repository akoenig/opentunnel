# OpenTunnel Public V1 Self-Hosting

OpenTunnel v1 runs as a self-hosted relay plus temporary clients downloaded from that relay's `/cli` endpoint. The relay stores no sessions, commands, outputs, invites, or audit logs. Active tunnel state exists only in memory.

## Build

From the repository root:

```bash
docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .
```

The Docker image includes supported Linux and macOS temporary CLI artifacts for `amd64` and `arm64` under `/opentunnel-artifacts`.

## Run A Local Relay

For local testing:

```bash
docker run --rm -p 8080:8080 opentunnel-relay:dev relay --public-url http://127.0.0.1:8080
```

For a public relay, set `--public-url` to the HTTPS origin users will fetch:

```bash
docker run -p 8080:8080 opentunnel-relay:dev relay --public-url https://relay.example.com
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

- `--artifact-dir` points to all supported files named like `opentunnel-1.0.0-linux-amd64`; the container default is `/opentunnel-artifacts`.
- `--version` defaults to the build version from `VERSION`; release versions have no leading `v`.
- `--public-url` is required and is embedded into the bootstrapper.

The relay serves only configured artifacts and their checksums. It does not expose arbitrary files.

## Operations

For Docker, systemd, CI, and manual release guidance, see `docs/public-v1/operations.md`.
