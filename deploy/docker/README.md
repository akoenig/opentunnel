# OpenTunnel Docker Relay

This Docker image runs the OpenTunnel relay and includes temporary CLI binaries for relay bootstrap downloads. It is an operator deployment path for the relay. It is not a package-manager or install-to-system distribution path for end users.

## Build And Run

From the repository root:

```bash
docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .
```

For local testing:

```bash
docker run --rm -p 8080:8080 opentunnel-relay:dev relay --public-url http://localhost:8080
```

## Released Image

GitHub Releases publish a self-contained image to GHCR:

```bash
docker run -p 8080:8080 ghcr.io/akoenig/opentunnel:1.0.0 \
  relay --public-url https://relay.example.com
```

The release workflow also publishes `ghcr.io/akoenig/opentunnel:latest`. Prefer immutable version tags for production because `latest` moves when a new release is published.

For public deployment, set `--public-url` to the HTTPS origin users will fetch:

```bash
docker run --rm -p 8080:8080 opentunnel-relay:dev relay --public-url https://relay.example.com
```

Terminate TLS with a reverse proxy or load balancer in front of the container.

Override relay defaults by passing command arguments, as shown above. Set `OPENTUNNEL_ACTIVITY_LOG_INTERVAL` to a Go duration, such as `30s` or `10m`, to change the activity logging interval.

The image includes `/opentunnel-artifacts` with Linux and macOS temporary CLI binaries for `amd64` and `arm64`. The relay serves those binaries from `/cli/bin/...` for bootstrap clients.

## Smoke Test

With the container running:

```bash
curl -fsSL http://localhost:8080/cli >/tmp/opentunnel-cli.sh
```

The relay stores no sessions or command data persistently. Active connection state is memory-only inside the relay process.
