---
title: Self-Hosting
description: "Run your own OpenTunnel relay: one stateless container, no database, no accounts."
---

An OpenTunnel relay is a single stateless process. It needs no database, no Redis, and no persistent volume; active tunnel state exists only in memory. Self-hosting means running one container behind your usual TLS-terminating reverse proxy.

## Run a relay

Use the released image and set `--public-url` to the HTTPS origin your users will fetch from:

```bash
docker run -p 8080:8080 ghcr.io/akoenig/opentunnel:latest \
  relay --public-url https://relay.example.com
```

Prefer immutable version tags for production, since `latest` is mutable and moves with each release:

```bash
docker run -p 8080:8080 ghcr.io/akoenig/opentunnel:1.0.0 \
  relay --public-url https://relay.example.com
```

Terminate TLS in front of the relay with your normal reverse proxy or load balancer. The `--public-url` value is embedded into the `/cli` bootstrapper, so it must match the public origin exactly. Public origins must use HTTPS; HTTP is accepted only for localhost and loopback development origins.

That's the whole deployment. Users on your relay then run:

```bash
curl -fsSL https://relay.example.com/cli | sh -s -- create
```

and the printed agent prompt automatically points at your origin, with no relay flag anywhere in the user-facing commands.

## Build from source

From the repository root:

```bash
docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .
```

The image includes the supported Linux and macOS temporary CLI artifacts for `amd64` and `arm64` under `/opentunnel-artifacts`. For local testing:

```bash
docker run --rm -p 8080:8080 opentunnel-relay:dev relay --public-url http://127.0.0.1:8080
```

## Relay flags

| Flag | Default | Purpose |
|---|---|---|
| `--public-url` | *(required)* | Public origin embedded into the `/cli` bootstrapper. |
| `--listen` | `:8080` | HTTP listen address. |
| `--health-listen` | off | Optional second listen address serving `GET /healthz` with the active tunnel count. Keep it private (for example bound to localhost); the count is operational telemetry, not public information. |
| `--artifact-dir` | `/opentunnel-artifacts` | Directory of CLI artifacts named like `opentunnel-1.0.0-linux-amd64`. |
| `--version` | build version | Version string used to resolve artifact filenames. |

The relay serves only configured artifacts and their checksums; it does not expose arbitrary files.

## systemd instead of Docker

`deploy/systemd/opentunnel-relay.service` and `deploy/systemd/opentunnel-relay.env.example` in the repository are copyable starting points. Set `OPENTUNNEL_PUBLIC_URL` to your public HTTPS origin and `OPENTUNNEL_ARTIFACT_DIR` to the artifacts served by `/cli`. See [relay operations](/guides/relay-operations/) for the operational details.
