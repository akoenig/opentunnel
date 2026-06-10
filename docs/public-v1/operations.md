# OpenTunnel Public V1 Operations

This guide describes repeatable ways to build, run, and verify a self-hosted OpenTunnel relay.

## Verification Commands

Run these before publishing a release or changing deployment artifacts:

```bash
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
rm -f ./opentunnel
```

## Docker Deployment

Build the relay image from the repository root:

```bash
docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .
```

Run it locally:

```bash
docker run --rm -p 8080:8080 opentunnel-relay:dev \
  relay \
  --public-url http://127.0.0.1:8080
```

Override relay defaults by passing command arguments; the Docker relay does not read `OPENTUNNEL_*` environment variables.

## systemd Deployment

Use `deploy/systemd/opentunnel-relay.service` and `deploy/systemd/opentunnel-relay.env.example` as copyable examples. Edit the environment file so `OPENTUNNEL_PUBLIC_URL` matches the public HTTPS origin and `OPENTUNNEL_ARTIFACT_DIR` points to the compatible artifacts served by `/cli`.

TLS is normally terminated by a reverse proxy or load balancer in front of the relay.

## Manual Release Process

1. Choose a version string, such as `1.0.0`.
2. Update `VERSION` to `1.0.0` before building.
3. Run the verification commands above.
4. Build the Docker image with `docker build -f deploy/docker/Dockerfile -t opentunnel-relay:1.0.0 .`.
5. Deploy the image to the relay host.
6. Start the relay with `docker run -p 8080:8080 opentunnel-relay:1.0.0 relay --public-url https://relay.example.com`.
7. Verify `/cli`.
8. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-linux-amd64` and `/cli/bin/opentunnel-1.0.0-linux-amd64.sha256`.
9. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-linux-arm64` and `/cli/bin/opentunnel-1.0.0-linux-arm64.sha256`.
10. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-darwin-amd64` and `/cli/bin/opentunnel-1.0.0-darwin-amd64.sha256`.
11. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-darwin-arm64` and `/cli/bin/opentunnel-1.0.0-darwin-arm64.sha256`.
12. Verify the public flow: `curl -fsSL https://relay.example.com/cli | sh -s -- create`, then run the generated `exec` command.

Artifact filenames are derived from `VERSION`. Development builds with `VERSION=dev` produce `/cli/bin/opentunnel-dev-*` paths instead of `opentunnel-1.0.0-*` paths.

## Checksum Boundary

The relay serves same-origin checksums for corruption and mismatch detection. These checksums are not a strong supply-chain security boundary. If the trusted relay origin is compromised, an attacker can change the bootstrapper, binary, and checksum together.
