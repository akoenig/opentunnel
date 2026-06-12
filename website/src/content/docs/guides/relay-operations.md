---
title: Relay Operations
description: "Operate a self-hosted OpenTunnel relay: Docker, systemd, upgrades, and verification."
---

This guide covers the day-2 concerns of running your own relay. If you haven't deployed one yet, start with [self-hosting](/guides/self-hosting/).

## Docker

Run the relay with explicit command arguments; the Docker relay does not read `OPENTUNNEL_*` environment variables:

```bash
docker run -p 8080:8080 ghcr.io/akoenig/opentunnel:1.0.0 \
  relay \
  --public-url https://relay.example.com
```

Override any default by passing additional flags (`--listen`, `--artifact-dir`, `--version`).

## Activity logging

The relay reports the number of active tunnels to stderr every five minutes:

```text
relay: active tunnels: 3
```

The count is the only thing reported. In line with the [security model](/concepts/security-model/), no sessions, invites, payloads, or client metadata are ever logged.

For supervisors and load balancers, `--health-listen` starts a separate private listener serving `GET /healthz` with the same count. It is off by default and deliberately not part of the public endpoint; bind it to an address that is not publicly reachable.

## systemd

Use the copyable examples from the repository:

- `deploy/systemd/opentunnel-relay.service`
- `deploy/systemd/opentunnel-relay.env.example`

Edit the environment file so `OPENTUNNEL_PUBLIC_URL` matches the public HTTPS origin and `OPENTUNNEL_ARTIFACT_DIR` points to the artifacts served by `/cli`. TLS is normally terminated by a reverse proxy or load balancer in front of the relay.

## Upgrades and version pinning

Prefer immutable GHCR version tags (`ghcr.io/akoenig/opentunnel:1.0.0`) for production; `latest` is mutable. Because the relay holds no persistent state, an upgrade is a container swap: in-flight sessions on the old process end when it stops, and users simply re-run `create`.

## Verifying a deployment

After deploying or upgrading, confirm the public surface works end to end:

1. Fetch the bootstrapper: `curl -fsSL https://relay.example.com/cli` should return a shell script with your origin embedded.
2. Verify artifacts and checksums resolve for each supported platform, for example:
   - `/cli/bin/opentunnel-1.0.0-linux-amd64` and `/cli/bin/opentunnel-1.0.0-linux-amd64.sha256`
   - `/cli/bin/opentunnel-1.0.0-darwin-arm64` and `/cli/bin/opentunnel-1.0.0-darwin-arm64.sha256`
3. Run the real flow: `curl -fsSL https://relay.example.com/cli | sh -s -- create`, then execute the generated `exec` command from another machine.

Artifact filenames derive from the release version. Development builds (`VERSION=dev`) serve `/cli/bin/opentunnel-dev-*` paths instead.

## Checksum boundary

The relay serves same-origin checksums for corruption and mismatch detection. They are **not** a supply-chain security boundary: if the trusted relay origin is compromised, an attacker can change the bootstrapper, binary, and checksum together. Operate the relay origin as trusted infrastructure and serve it exclusively over HTTPS. See the [security model](/concepts/security-model/).
