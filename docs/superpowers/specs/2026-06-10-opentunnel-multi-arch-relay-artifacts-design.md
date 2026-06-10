# OpenTunnel Multi-Arch Relay Artifacts Design

## Purpose

Self-hosting a relay should be a two-step Docker flow: build the image, then start the relay container with the public URL. The relay image should already contain every supported temporary CLI binary needed by `/cli`, so operators do not need to build, mount, or configure host/client artifacts for each architecture.

## Goals

- Make the Docker relay image self-contained for supported v1 platforms.
- Let users run `curl -fsSL <relay>/cli | sh -s -- create` and `exec` without knowing their machine architecture.
- Keep `--public-url` required, because only the operator knows the externally reachable origin.
- Infer the relay artifact version from repository state instead of requiring `--version` for normal operation.
- Use version values without a leading `v`, such as `1.0.0`, while keeping `dev` for unreleased development.

## Non-Goals

- Windows support in this milestone.
- Package-manager installation or install-to-system flows.
- Strong supply-chain guarantees such as signatures, provenance, or transparency logs.
- Dynamic artifact discovery from arbitrary directories or remote registries.
- Git-based version inference at runtime or during Docker builds.

## Supported Platforms

The v1 multi-arch artifact set is:

- `linux-amd64`
- `linux-arm64`
- `darwin-amd64`
- `darwin-arm64`

Windows can be added later as a separate design because it needs more than a `.exe` artifact. Native Windows support will require a PowerShell bootstrapper and Windows-specific command execution semantics.

## Version Source

Add a repo-root `VERSION` file as the version source of truth.

Development state:

```text
dev
```

Release state:

```text
1.0.0
```

Release versions should not include a leading `v`. Git tags and GitHub Releases should match the file value where practical, such as tag `1.0.0` for `VERSION` value `1.0.0`.

The build should embed this value into the `opentunnel` binary. The relay command should make `--version` optional and default to the embedded version. Docker builds should read the same `VERSION` value and use it for artifact filenames.

## Relay Artifact Directory

Add artifact-directory support to the relay. The directory contains all supported platform binaries named with this shape:

```text
opentunnel-<version>-<platform>
```

For `VERSION=dev`, the self-contained container includes:

```text
/opentunnel-artifacts/opentunnel-dev-linux-amd64
/opentunnel-artifacts/opentunnel-dev-linux-arm64
/opentunnel-artifacts/opentunnel-dev-darwin-amd64
/opentunnel-artifacts/opentunnel-dev-darwin-arm64
```

The relay command should default `--artifact-dir` to `/opentunnel-artifacts`, which is the path used by the runtime container image. Non-container deployments can override the path with `--artifact-dir`.

When `/cli` artifact serving is enabled, startup validation should fail closed if any required platform binary is missing or unreadable. The relay should serve only known artifact paths and their checksums, not arbitrary files in the directory.

## Bootstrap Behavior

`/cli` should serve one POSIX shell bootstrapper. The script should:

- Detect the operating system with `uname -s`.
- Detect the architecture with `uname -m`.
- Map the detected pair to one of the supported platform keys.
- Fail clearly on unsupported platforms.
- Download `/cli/bin/opentunnel-<version>-<platform>`.
- Download the matching `.sha256` file.
- Verify the downloaded checksum.
- Cache by platform, version, and checksum.
- Export `OPENTUNNEL_RELAY_ORIGIN`.
- Execute the cached binary with the original arguments.

The script should continue using POSIX `sh` and should not claim Windows support.

## Docker UX

The intended operator flow is:

```bash
docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .
```

```bash
docker run --rm -p 8080:8080 opentunnel-relay:dev \
  relay --public-url http://localhost:8080
```

For production:

```bash
docker run -p 8080:8080 opentunnel-relay:1.0.0 \
  relay --public-url https://relay.example.com
```

The Docker build should cross-compile the relay executable and the four supported temporary CLI artifacts. The runtime image should include `/opentunnel` and `/opentunnel-artifacts`. The relay command defaults should provide the listen address, artifact directory, and embedded version, but `--public-url` remains operator-supplied.

## Release Flow

Normal development keeps `VERSION` as `dev`.

For a release:

1. Update `VERSION` to the release value, such as `1.0.0`.
2. Commit that change.
3. Tag the commit with the same value where practical.
4. Build and publish release artifacts from that commit.
5. Optionally validate in CI that release tags match `VERSION`.

This avoids relying on `.git` inside Docker builds and keeps the repository state aligned with GitHub Releases.

## Artifact CLI

Replace the current single-artifact `--artifact-path` flag with one artifact configuration argument: `--artifact-dir`.

The default is `--artifact-dir=/opentunnel-artifacts`. Non-container deployments can override that one path when needed.

There is no backward-compatibility requirement for `--artifact-path`. Documentation, Docker, systemd examples, and tests should move to `--artifact-dir` and the complete multi-arch artifact set.

## Testing And Verification

Tests should cover:

- Version loading from `VERSION` and embedded version fallback for omitted `--version`.
- Relay startup validation for a complete artifact directory.
- Relay startup failure when any required platform artifact is missing.
- Serving all four `/cli/bin/opentunnel-<version>-<platform>` paths.
- Serving each matching `.sha256` path.
- `/cli` rendering platform detection rather than a fixed platform key.
- Bootstrap mapping for Linux x86_64, Linux aarch64/arm64, Darwin x86_64, and Darwin arm64.
- Bootstrap failure for unsupported OS/architecture combinations.
- Docker image build smoke test and `/cli` availability.

Public docs should show the two-step Docker flow and explain that Windows is not supported in this milestone.

## Acceptance Criteria

The design is complete when:

- A self-built Docker relay image includes all four supported platform artifacts.
- Starting the container only requires `relay --public-url <origin>` beyond Docker port publishing.
- `/cli` selects the correct supported artifact at runtime.
- The relay fails closed if the configured artifact set is incomplete.
- Artifact filenames and release docs use versions without a leading `v`.
- `VERSION=dev` works for local development builds.
