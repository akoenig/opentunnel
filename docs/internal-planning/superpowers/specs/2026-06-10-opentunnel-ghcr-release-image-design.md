# OpenTunnel GHCR Release Image Design

## Purpose

Release operators should be able to run a pre-built, self-contained OpenTunnel relay image from GitHub Container Registry instead of building the image themselves. The published image must contain the relay binary and all supported temporary CLI artifacts.

## Goals

- Publish a GHCR container image when a GitHub Release is published.
- Publish both an immutable version tag and a mutable `latest` tag.
- Use the existing Dockerfile so the image contains `/opentunnel` and `/opentunnel-artifacts`.
- Keep `VERSION` as the release version source of truth.
- Fail closed if the GitHub Release tag does not match `VERSION`.

## Non-Goals

- Publishing images on every `main` push.
- Publishing an `edge`, `nightly`, or branch tag.
- Supporting `v`-prefixed release tags.
- Adding signing, provenance, SBOMs, or vulnerability scanning in this milestone.
- Changing the runtime relay UX; `--public-url` remains required.

## Trigger

Add a release workflow triggered only by published GitHub Releases:

```yaml
on:
  release:
    types: [published]
```

This makes GitHub Releases the source of truth and avoids publishing images from accidental tag pushes.

## Version Validation

The workflow reads `VERSION` from the checked-out release commit.

Validation rules:

- `VERSION` must not be `dev`.
- `github.event.release.tag_name` must exactly equal `VERSION`.
- `v`-prefixed tags such as `v1.0.0` are rejected because release versions use no leading `v`.

For a release `1.0.0`, the repository must contain:

```text
1.0.0
```

in `VERSION`, and the GitHub Release tag must be `1.0.0`.

## Image Publishing

The workflow logs in to GHCR using the repository `GITHUB_TOKEN` and pushes:

```text
ghcr.io/akoenig/opentunnel:1.0.0
ghcr.io/akoenig/opentunnel:latest
```

The workflow should use repository metadata for the image name where possible, for example lowercasing `github.repository` to produce `ghcr.io/akoenig/opentunnel`.

Required permissions:

```yaml
permissions:
  contents: read
  packages: write
```

The build uses `deploy/docker/Dockerfile`. No alternate release Dockerfile is needed.

## Operator UX

After release, operators can run:

```bash
docker run -p 8080:8080 ghcr.io/akoenig/opentunnel:1.0.0 \
  relay --public-url https://relay.example.com
```

Operators who accept mutable image tags can use:

```bash
docker run -p 8080:8080 ghcr.io/akoenig/opentunnel:latest \
  relay --public-url https://relay.example.com
```

Docs should state that `latest` is mutable and immutable version tags are preferred for production.

## Documentation

Update public operations docs to describe:

- Set `VERSION` to the release value, such as `1.0.0`.
- Commit the version change.
- Publish a GitHub Release with the matching tag, such as `1.0.0`.
- The release workflow publishes GHCR tags `1.0.0` and `latest`.
- Operators can run the pre-built image directly from GHCR.

Update Docker docs to mention the GHCR image as the release path while keeping local `docker build` instructions for development and self-built deployments.

## Testing And Verification

Implementation should verify:

- Workflow syntax references the `release.published` trigger.
- Workflow permissions include `contents: read` and `packages: write`.
- Version validation rejects `dev` and mismatched tags.
- Docker metadata or explicit tagging produces both the version tag and `latest`.
- Public docs show `ghcr.io/akoenig/opentunnel:1.0.0` and explain `latest`.

Local verification can statically inspect the workflow and run a Docker build. Publishing to GHCR is verified by GitHub Actions during an actual release.

## Acceptance Criteria

This feature is complete when:

- A GitHub Release tagged `1.0.0` with `VERSION=1.0.0` builds and pushes `ghcr.io/akoenig/opentunnel:1.0.0` and `ghcr.io/akoenig/opentunnel:latest`.
- A GitHub Release whose tag differs from `VERSION` fails before publishing.
- A GitHub Release with `VERSION=dev` fails before publishing.
- Public docs tell operators how to run the pre-built GHCR image.
