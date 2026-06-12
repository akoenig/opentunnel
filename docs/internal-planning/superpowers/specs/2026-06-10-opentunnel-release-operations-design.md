# OpenTunnel Release Operations Design

## Purpose

This milestone turns the public v1 implementation into something operators can build, run, and verify repeatably. It adds deployment artifacts and release workflow documentation without changing OpenTunnel's core product model.

## Scope

In scope:

- Docker image support for running the relay.
- Systemd deployment guidance and an example unit for native Linux operation.
- CI workflow for Go tests, race tests, vet, module tidy diff, and build verification.
- Release workflow documentation for producing versioned relay artifacts and configuring `--artifact-path` / `--version`.
- Public operations documentation linking Docker, systemd, CI, release, and verification guidance.

Out of scope:

- Package-manager distribution.
- Host/client install-to-system flows.
- Accounts, dashboards, teams, tokens, billing, or hosted control-plane features.
- Strong supply-chain claims such as signatures, provenance, or transparency logs.
- Automatic publishing to registries unless explicitly added later.

## Architecture

The runtime code should remain mostly unchanged. Release operations should be added around the existing CLI and relay behavior:

- `deploy/docker/`: Dockerfile and container-specific documentation or helper files.
- `deploy/systemd/`: example systemd unit and environment file template.
- `.github/workflows/`: CI workflow that mirrors local verification commands.
- `docs/public-v1/operations.md`: operator-facing release and deployment guide.

The relay remains a single foreground process. Docker and systemd should run `opentunnel relay` with `--listen`, `--public-url`, `--artifact-path`, and `--version`. The artifact served by `/cli` should be the same compatible binary operators intend clients to download.

## Docker Design

The Docker image should build the Go binary in a builder stage and copy it into a small runtime image. The default container command should run the relay and make the required flags explicit through environment variables or documented command overrides.

The Docker path must avoid implying package-manager or system-wide client installation. It is an operator deployment path for the relay and served artifact only.

Verification should include building the image and starting a container or equivalent local process that serves `/cli` successfully.

## Systemd Design

The systemd unit should be an example operators can copy and adapt. It should:

- Run the relay in foreground mode.
- Use an environment file for `OPENTUNNEL_PUBLIC_URL`, `OPENTUNNEL_ARTIFACT_PATH`, and `OPENTUNNEL_VERSION`.
- Restart on failure.
- Avoid claiming sandboxing is complete or sufficient as a security boundary.

Systemd docs should explain that TLS is normally terminated by a reverse proxy in front of the relay.

## CI Design

CI should run the same verification commands used during development:

```bash
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
```

The workflow should be simple and deterministic. It should not publish artifacts or push images in this milestone.

## Release Documentation

Release docs should describe a manual release process:

1. Choose a version string.
2. Run full verification.
3. Build the `opentunnel` binary.
4. Configure relay `--artifact-path` to that binary.
5. Configure relay `--version` to the chosen version.
6. Verify `/cli`, artifact, checksum, and generated prompt flow.

The docs should explicitly say same-origin checksums detect corruption or mismatch within the trusted relay-origin model, not strong supply-chain security.

## Testing And Verification

This milestone passes when:

- Docker image builds successfully.
- Docker or an equivalent container-run smoke test can serve `/cli`.
- Systemd unit and environment template reference valid current CLI flags.
- CI workflow uses current local verification commands.
- Operations docs accurately describe Docker, systemd, release, and verification flows.
- Full local verification passes:

```bash
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
```

The generated root `opentunnel` binary from `go build ./cmd/opentunnel` should be removed after verification.

## Acceptance Criteria

The milestone is complete when:

- Docker deployment artifact exists and is verified.
- Systemd deployment artifact exists and is documented.
- CI workflow exists and mirrors local verification.
- Public operations documentation exists.
- Release documentation explains versioned artifacts and checksum limitations honestly.
- No runtime behavior changes are introduced unless needed to support current CLI flags cleanly.
