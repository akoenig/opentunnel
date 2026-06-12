# OpenTunnel GHCR Release Image Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish a self-contained OpenTunnel relay container image to GHCR when a GitHub Release is published.

**Architecture:** Add one release workflow triggered by `release.published`. The workflow validates that the release tag exactly matches `VERSION`, rejects `dev`, builds the existing Dockerfile, and pushes both the immutable version tag and `latest` to GHCR. Public docs describe GHCR as the release path while preserving local Docker build instructions for development.

**Tech Stack:** GitHub Actions, Docker Buildx, GHCR, existing Go/Docker release image.

---

## File Structure

- Create: `.github/workflows/release.yml` - release-only GHCR publish workflow.
- Modify: `docs/public-v1/operations.md` - release process and GHCR operator commands.
- Modify: `deploy/docker/README.md` - release image usage plus local build path.
- Modify: `README.md` - quick reference to released GHCR image.

---

### Task 1: Add Release Workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create the release workflow**

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  release:
    types:
      - published

permissions:
  contents: read
  packages: write

jobs:
  publish-container:
    runs-on: ubuntu-24.04
    steps:
      - name: Checkout
        uses: actions/checkout@v6.0.3

      - name: Read version
        id: version
        shell: bash
        run: |
          set -euo pipefail
          version="$(tr -d '[:space:]' < VERSION)"
          tag="${{ github.event.release.tag_name }}"
          if [ "$version" = "dev" ]; then
            printf 'VERSION must not be dev for a release\n' >&2
            exit 1
          fi
          if [ "$tag" != "$version" ]; then
            printf 'release tag %s does not match VERSION %s\n' "$tag" "$version" >&2
            exit 1
          fi
          case "$version" in
            v*)
              printf 'release versions must not use a leading v: %s\n' "$version" >&2
              exit 1
              ;;
          esac
          printf 'version=%s\n' "$version" >> "$GITHUB_OUTPUT"

      - name: Set image name
        id: image
        shell: bash
        run: |
          set -euo pipefail
          image="ghcr.io/${GITHUB_REPOSITORY,,}"
          printf 'name=%s\n' "$image" >> "$GITHUB_OUTPUT"

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3.11.1

      - name: Log in to GHCR
        uses: docker/login-action@v3.5.0
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push image
        uses: docker/build-push-action@v6.18.0
        with:
          context: .
          file: deploy/docker/Dockerfile
          push: true
          tags: |
            ${{ steps.image.outputs.name }}:${{ steps.version.outputs.version }}
            ${{ steps.image.outputs.name }}:latest
```

- [ ] **Step 2: Validate workflow content statically**

Run: `grep -n "release:" .github/workflows/release.yml && grep -n "packages: write" .github/workflows/release.yml && grep -n "docker/build-push-action" .github/workflows/release.yml`

Expected: output includes the release trigger, `packages: write`, and `docker/build-push-action@v6.18.0`.

- [ ] **Step 3: Validate version-check shell logic locally**

Run:

```bash
tmp=$(mktemp -d /tmp/opencode/ghcr-version-check.XXXXXX) && printf '1.0.0\n' > "$tmp/VERSION" && tag=1.0.0 version="$(tr -d '[:space:]' < "$tmp/VERSION")" && test "$version" != dev && test "$tag" = "$version" && case "$version" in v*) exit 1 ;; esac && rm -rf "$tmp"
```

Expected: command exits 0.

- [ ] **Step 4: Validate mismatch rejection locally**

Run:

```bash
tmp=$(mktemp -d /tmp/opencode/ghcr-version-check.XXXXXX) && printf '1.0.0\n' > "$tmp/VERSION" && tag=1.0.1 version="$(tr -d '[:space:]' < "$tmp/VERSION")" && if [ "$tag" != "$version" ]; then rm -rf "$tmp"; exit 0; fi; rm -rf "$tmp"; exit 1
```

Expected: command exits 0 because the mismatch is detected.

- [ ] **Step 5: Validate dev rejection locally**

Run:

```bash
tmp=$(mktemp -d /tmp/opencode/ghcr-version-check.XXXXXX) && printf 'dev\n' > "$tmp/VERSION" && version="$(tr -d '[:space:]' < "$tmp/VERSION")" && if [ "$version" = dev ]; then rm -rf "$tmp"; exit 0; fi; rm -rf "$tmp"; exit 1
```

Expected: command exits 0 because `dev` is detected as invalid for release publishing.

- [ ] **Step 6: Build Docker image locally**

Run: `docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .`

Expected: Docker image builds successfully.

- [ ] **Step 7: Commit workflow**

```bash
git add .github/workflows/release.yml
git commit -m "ci: publish release image to ghcr"
```

---

### Task 2: Document GHCR Release Usage

**Files:**
- Modify: `docs/public-v1/operations.md`
- Modify: `deploy/docker/README.md`
- Modify: `README.md`

- [ ] **Step 1: Update operations release process**

In `docs/public-v1/operations.md`, update the manual release process section so it contains this flow:

```markdown
## Manual Release Process

1. Choose a version string, such as `1.0.0`.
2. Update `VERSION` to `1.0.0` and commit that change.
3. Run `go test ./... -count=1`, `go vet ./...`, `go mod tidy -diff`, and `go test -race ./... -count=1`.
4. Publish a GitHub Release tagged `1.0.0` from that commit.
5. Wait for the release workflow to publish `ghcr.io/akoenig/opentunnel:1.0.0` and `ghcr.io/akoenig/opentunnel:latest`.
6. Start the relay with `docker run -p 8080:8080 ghcr.io/akoenig/opentunnel:1.0.0 relay --public-url https://relay.example.com`.
7. Verify `/cli`.
8. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-linux-amd64` and `/cli/bin/opentunnel-1.0.0-linux-amd64.sha256`.
9. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-linux-arm64` and `/cli/bin/opentunnel-1.0.0-linux-arm64.sha256`.
10. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-darwin-amd64` and `/cli/bin/opentunnel-1.0.0-darwin-amd64.sha256`.
11. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-darwin-arm64` and `/cli/bin/opentunnel-1.0.0-darwin-arm64.sha256`.
12. Verify the public flow: `curl -fsSL https://relay.example.com/cli | sh -s -- create`, then run the generated `exec` command.

Artifact filenames are derived from `VERSION`. Development builds with `VERSION=dev` produce `/cli/bin/opentunnel-dev-*` paths instead of `opentunnel-1.0.0-*` paths. Prefer immutable GHCR version tags for production; `latest` is mutable.
```

- [ ] **Step 2: Update Docker README release path**

In `deploy/docker/README.md`, add this section after the build section:

```markdown
## Released Image

GitHub Releases publish a self-contained image to GHCR:

```bash
docker run -p 8080:8080 ghcr.io/akoenig/opentunnel:1.0.0 \
  relay --public-url https://relay.example.com
```

The release workflow also publishes `ghcr.io/akoenig/opentunnel:latest`. Prefer immutable version tags for production because `latest` moves when a new release is published.
```

Keep the existing local `docker build` instructions for development/self-built deployments.

- [ ] **Step 3: Update root README quick reference**

In `README.md`, add a short released-image example near the Docker quickstart:

```markdown
For released images:

```bash
docker run -p 8080:8080 ghcr.io/akoenig/opentunnel:1.0.0 \
  relay --public-url https://relay.example.com
```

Use immutable version tags for production. The `latest` tag is also published and moves with each release.
```

- [ ] **Step 4: Verify docs mention GHCR tags**

Run: `grep -R -n "ghcr.io/akoenig/opentunnel:1.0.0\|ghcr.io/akoenig/opentunnel:latest\|latest is mutable\|latest.*moves" README.md docs/public-v1 deploy/docker`

Expected: output includes GHCR version tag references, `latest`, and a warning that `latest` is mutable or moves.

- [ ] **Step 5: Verify no v-prefixed release examples in active docs**

Run: `grep -R -n -E "v1\.0\.0|version v1|VERSION=v1" README.md docs/public-v1 deploy/docker .github || true`

Expected: no output.

- [ ] **Step 6: Commit docs**

```bash
git add README.md docs/public-v1/operations.md deploy/docker/README.md
git commit -m "docs: describe ghcr release images"
```

---

### Task 3: Final Verification

**Files:**
- `.github/workflows/release.yml`
- `docs/public-v1/operations.md`
- `deploy/docker/README.md`
- `README.md`

- [ ] **Step 1: Run Go tests**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 2: Run Go vet**

Run: `go vet ./...`

Expected: PASS with no output.

- [ ] **Step 3: Run module tidy check**

Run: `go mod tidy -diff`

Expected: PASS with no diff output.

- [ ] **Step 4: Build Docker image**

Run: `docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .`

Expected: Docker image builds successfully.

- [ ] **Step 5: Verify workflow and docs statically**

Run:

```bash
grep -n "release:" .github/workflows/release.yml && grep -n "types:" .github/workflows/release.yml && grep -n "published" .github/workflows/release.yml && grep -n "packages: write" .github/workflows/release.yml && grep -n "ghcr.io/akoenig/opentunnel:1.0.0" README.md docs/public-v1/operations.md deploy/docker/README.md
```

Expected: command exits 0 and prints matching workflow/docs lines.

- [ ] **Step 6: Check worktree status**

Run: `git status --short`

Expected: only intended files are modified or staged. Existing unrelated `.agents/` and `skills-lock.json` may still be untracked and should not be included unless explicitly requested.
