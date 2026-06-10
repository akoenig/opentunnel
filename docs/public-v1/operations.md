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
  --listen :8080 \
  --public-url http://localhost:8080 \
  --artifact-path /opentunnel \
  --version dev
```

## systemd Deployment

Use `deploy/systemd/opentunnel-relay.service` and `deploy/systemd/opentunnel-relay.env.example` as copyable examples. Edit the environment file so `OPENTUNNEL_PUBLIC_URL` matches the public HTTPS origin and `OPENTUNNEL_ARTIFACT_PATH` points to the compatible binary served by `/cli`.

TLS is normally terminated by a reverse proxy or load balancer in front of the relay.

## Manual Release Process

1. Choose a version string, such as `v1.0.0`.
2. Run the verification commands above.
3. Build the binary with `go build -o opentunnel ./cmd/opentunnel`.
4. Deploy the binary to the relay host.
5. Start the relay with `--artifact-path` pointing to that binary and `--version` set to the chosen version.
6. Verify `/cli`, `/cli/bin/opentunnel-v1.0.0-linux-amd64`, and the `.sha256` endpoint for the version and platform you deployed.
7. Verify the public flow: `curl -fsSL https://relay.example.com/cli | sh -s -- create`, then run the generated `exec` command.

## Checksum Boundary

The relay serves same-origin checksums for corruption and mismatch detection. These checksums are not a strong supply-chain security boundary. If the trusted relay origin is compromised, an attacker can change the bootstrapper, binary, and checksum together.
