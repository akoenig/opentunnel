# OpenTunnel Release Operations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add repeatable Docker, systemd, CI, and release-operations support for operating the OpenTunnel relay without changing the public v1 runtime model.

**Architecture:** Add deployment artifacts around the existing `opentunnel relay` command. Keep runtime behavior unchanged; Docker, systemd, CI, and docs should all reference current CLI flags: `--listen`, `--public-url`, `--artifact-path`, and `--version`.

**Tech Stack:** Go 1.23, Docker multi-stage builds, systemd unit files, GitHub Actions, Markdown documentation, shell smoke tests.

---

## File Structure

- Create `deploy/docker/Dockerfile`: multi-stage relay image that builds `opentunnel` and runs the relay.
- Create `deploy/docker/README.md`: Docker build/run/smoke-test instructions.
- Create `deploy/systemd/opentunnel-relay.service`: copyable systemd unit example.
- Create `deploy/systemd/opentunnel-relay.env.example`: environment template for relay flags.
- Create `deploy/systemd/README.md`: systemd deployment guidance.
- Create `.github/workflows/ci.yml`: Go verification workflow.
- Create `docs/public-v1/operations.md`: operator-facing release/deployment guide tying Docker, systemd, CI, and manual release together.
- Modify `docs/public-v1/self-hosting.md`: add a short pointer to `docs/public-v1/operations.md`.
- Commit after each task.

## Task 1: Docker Relay Deployment Artifact

**Files:**
- Create: `deploy/docker/Dockerfile`
- Create: `deploy/docker/README.md`

- [ ] **Step 1: Create Dockerfile**

Create `deploy/docker/Dockerfile` with:

```dockerfile
# syntax=docker/dockerfile:1

FROM golang:1.23 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/opentunnel ./cmd/opentunnel

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/opentunnel /opentunnel

USER nonroot:nonroot
EXPOSE 8080

ENV OPENTUNNEL_LISTEN=:8080
ENV OPENTUNNEL_PUBLIC_URL=http://localhost:8080
ENV OPENTUNNEL_ARTIFACT_PATH=/opentunnel
ENV OPENTUNNEL_VERSION=dev

ENTRYPOINT ["/opentunnel"]
CMD ["relay", "--listen", ":8080", "--public-url", "http://localhost:8080", "--artifact-path", "/opentunnel", "--version", "dev"]
```

- [ ] **Step 2: Create Docker README**

Create `deploy/docker/README.md` with:

```markdown
# OpenTunnel Docker Relay

This Docker image runs the OpenTunnel relay and serves the same `opentunnel` binary as the temporary `/cli` artifact. It is an operator deployment path for the relay. It is not a package-manager or install-to-system distribution path for end users.

## Build

From the repository root:

```bash
docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .
```

## Run

For local testing:

```bash
docker run --rm -p 8080:8080 opentunnel-relay:dev \
  relay \
  --listen :8080 \
  --public-url http://localhost:8080 \
  --artifact-path /opentunnel \
  --version dev
```

For public deployment, set `--public-url` to the HTTPS origin users will fetch. Terminate TLS with a reverse proxy or load balancer in front of the container.

## Smoke Test

With the container running:

```bash
curl -fsSL http://localhost:8080/cli >/tmp/opentunnel-cli.sh
```

The relay stores no sessions or command data persistently. Active connection state is memory-only inside the relay process.
```

- [ ] **Step 3: Verify Dockerfile syntax text and docs**

Run:

```bash
test -s deploy/docker/Dockerfile
test -s deploy/docker/README.md
grep -En -- "relay|/cli|--artifact-path|--version" deploy/docker/README.md
```

Expected: all commands pass and grep prints matching lines.

- [ ] **Step 4: Build Docker image if Docker is available**

Run:

```bash
if command -v docker >/dev/null 2>&1; then docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .; else printf 'docker not available; skipping image build\n'; fi
```

Expected: Docker build passes when Docker is installed. If Docker is unavailable, the command prints `docker not available; skipping image build` and exits 0.

- [ ] **Step 5: Commit Docker artifacts**

Run:

```bash
git add deploy/docker/Dockerfile deploy/docker/README.md
git commit -m "deploy: add docker relay image"
```

## Task 2: Systemd Relay Deployment Artifact

**Files:**
- Create: `deploy/systemd/opentunnel-relay.service`
- Create: `deploy/systemd/opentunnel-relay.env.example`
- Create: `deploy/systemd/README.md`

- [ ] **Step 1: Create systemd unit**

Create `deploy/systemd/opentunnel-relay.service` with:

```ini
[Unit]
Description=OpenTunnel relay
Documentation=https://example.invalid/opentunnel/docs/public-v1/self-hosting
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/opentunnel/relay.env
ExecStart=/usr/local/bin/opentunnel relay --listen ${OPENTUNNEL_LISTEN} --public-url ${OPENTUNNEL_PUBLIC_URL} --artifact-path ${OPENTUNNEL_ARTIFACT_PATH} --version ${OPENTUNNEL_VERSION}
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 2: Create environment template**

Create `deploy/systemd/opentunnel-relay.env.example` with:

```bash
OPENTUNNEL_LISTEN=:8080
OPENTUNNEL_PUBLIC_URL=https://relay.example.com
OPENTUNNEL_ARTIFACT_PATH=/opt/opentunnel/opentunnel
OPENTUNNEL_VERSION=v1
```

- [ ] **Step 3: Create systemd README**

Create `deploy/systemd/README.md` with:

```markdown
# OpenTunnel systemd Relay

This directory contains an example native Linux systemd deployment for the OpenTunnel relay.

## Files

- `opentunnel-relay.service`: example systemd unit.
- `opentunnel-relay.env.example`: environment file template for relay flags.

## Install Example

```bash
sudo install -d -m 0755 /etc/opentunnel /opt/opentunnel
sudo install -m 0755 ./opentunnel /opt/opentunnel/opentunnel
sudo install -m 0644 deploy/systemd/opentunnel-relay.env.example /etc/opentunnel/relay.env
sudo install -m 0644 deploy/systemd/opentunnel-relay.service /etc/systemd/system/opentunnel-relay.service
sudo systemctl daemon-reload
sudo systemctl enable --now opentunnel-relay.service
```

Edit `/etc/opentunnel/relay.env` before starting the service. Set `OPENTUNNEL_PUBLIC_URL` to the HTTPS origin users will fetch.

TLS is normally terminated by a reverse proxy or load balancer in front of the relay. The hardening settings in the example unit are useful defaults, not a complete security boundary.
```

- [ ] **Step 4: Verify unit references current flags**

Run:

```bash
grep -n -- "--listen.*--public-url.*--artifact-path.*--version" deploy/systemd/opentunnel-relay.service
grep -n "OPENTUNNEL_PUBLIC_URL\|OPENTUNNEL_ARTIFACT_PATH\|OPENTUNNEL_VERSION" deploy/systemd/opentunnel-relay.env.example
test -s deploy/systemd/README.md
```

Expected: all commands pass and grep prints matching lines.

- [ ] **Step 5: Commit systemd artifacts**

Run:

```bash
git add deploy/systemd/opentunnel-relay.service deploy/systemd/opentunnel-relay.env.example deploy/systemd/README.md
git commit -m "deploy: add systemd relay example"
```

## Task 3: CI Verification Workflow

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create CI workflow**

Create `.github/workflows/ci.yml` with:

```yaml
name: CI

on:
  push:
    branches:
      - main
  pull_request:

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23.x'
          cache: true

      - name: Test
        run: go test ./... -count=1

      - name: Vet
        run: go vet ./...

      - name: Module tidy check
        run: go mod tidy -diff

      - name: Race tests
        run: go test -race ./... -count=1

      - name: Build CLI
        run: go build ./cmd/opentunnel
```

- [ ] **Step 2: Verify workflow command parity**

Run:

```bash
grep -n "go test ./... -count=1\|go vet ./...\|go mod tidy -diff\|go test -race ./... -count=1\|go build ./cmd/opentunnel" .github/workflows/ci.yml
```

Expected: output includes all five local verification commands.

- [ ] **Step 3: Commit CI workflow**

Run:

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add go verification workflow"
```

## Task 4: Public Operations Documentation

**Files:**
- Create: `docs/public-v1/operations.md`
- Modify: `docs/public-v1/self-hosting.md`

- [ ] **Step 1: Create operations guide**

Create `docs/public-v1/operations.md` with:

```markdown
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
```

- [ ] **Step 2: Add self-hosting pointer**

Append this section to `docs/public-v1/self-hosting.md`:

```markdown

## Operations

For Docker, systemd, CI, and manual release guidance, see `docs/public-v1/operations.md`.
```

- [ ] **Step 3: Verify operations docs references**

Run:

```bash
grep -n "Docker Deployment\|systemd Deployment\|Manual Release Process\|Checksum Boundary" docs/public-v1/operations.md
grep -n "operations.md" docs/public-v1/self-hosting.md
```

Expected: all commands pass and grep prints matching lines.

- [ ] **Step 4: Commit operations docs**

Run:

```bash
git add docs/public-v1/operations.md docs/public-v1/self-hosting.md
git commit -m "docs: add release operations guide"
```

## Task 5: Final Release Operations Verification

**Files:**
- No committed source changes expected.

- [ ] **Step 1: Run full local verification**

Run:

```bash
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
rm -f ./opentunnel
```

Expected: all commands pass and no root `opentunnel` binary remains.

- [ ] **Step 2: Verify Docker image or record unavailable Docker**

Run:

```bash
if command -v docker >/dev/null 2>&1; then docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .; else printf 'docker not available; skipping image build\n'; fi
```

Expected: Docker build passes when Docker is installed. If Docker is unavailable, the command prints `docker not available; skipping image build` and exits 0.

- [ ] **Step 3: Verify deployment artifacts reference current CLI flags**

Run:

```bash
grep -En -- "--listen|--public-url|--artifact-path|--version" deploy/docker/README.md deploy/systemd/opentunnel-relay.service docs/public-v1/operations.md
grep -n "go test ./... -count=1\|go vet ./...\|go mod tidy -diff\|go test -race ./... -count=1\|go build ./cmd/opentunnel" .github/workflows/ci.yml docs/public-v1/operations.md
grep -n "not a strong supply-chain security boundary" docs/public-v1/operations.md
```

Expected: all commands pass and print matching lines.

- [ ] **Step 4: Check final status**

Run:

```bash
git status --short
```

Expected: only unrelated untracked `.agents/` and `skills-lock.json`, unless the user has changed additional files concurrently.

- [ ] **Step 5: Do not create an empty verification commit**

If verification required no file changes, leave the commit history unchanged. If verification changed files unexpectedly, stop and inspect the diff before proceeding.
