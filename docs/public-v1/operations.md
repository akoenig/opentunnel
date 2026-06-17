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

Override relay defaults by passing command arguments. Set `OPENTUNNEL_ACTIVITY_LOG_INTERVAL` to a Go duration, such as `30s` or `10m`, to change the activity logging interval.

The relay reports the number of active tunnels to stderr every five minutes by default (`relay: active tunnels: 3`). The count is the only thing reported; no sessions, invites, payloads, or client metadata are ever logged.

For supervisors and load balancers, `--health-listen` starts a separate private listener serving `GET /healthz` with the same count. It is off by default and is not part of the public endpoint; bind it to an address that is not publicly reachable.

## systemd Deployment

Use `deploy/systemd/opentunnel-relay.service` and `deploy/systemd/opentunnel-relay.env.example` as copyable examples. Edit the environment file so `OPENTUNNEL_PUBLIC_URL` matches the public HTTPS origin and `OPENTUNNEL_ARTIFACT_DIR` points to the compatible artifacts served by `/cli`. Optionally set `OPENTUNNEL_ACTIVITY_LOG_INTERVAL` to a positive Go duration.

TLS is normally terminated by a reverse proxy or load balancer in front of the relay. Public relay origins must use HTTPS; HTTP is accepted only for localhost and loopback development origins.

For `/tunnel` WebSockets, proxies and load balancers must preserve `OpenTunnel-Role` and `OpenTunnel-Session` headers. Proxies and WAFs must not inject an `Origin` header into CLI tunnel requests because the relay rejects non-empty `Origin`.

## Release Command

Use the opencode slash command for normal releases:

```text
/release
/release patch
/release minor
/release major
```

The command runs `scripts/release.sh`. It fetches `origin main --tags`, requires the current branch to be `main`, requires local `main` to match `origin/main`, requires no tracked or untracked worktree changes, requires `VERSION=dev`, and requires authenticated `gh` access. It infers or applies the requested SemVer bump, updates `VERSION`, runs the release verification commands, commits the release, creates the GitHub Release, and commits `VERSION=dev` back to `main` for subsequent development.

After pulling `.opencode/opencode.json`, restart opencode before using `/release`. Running opencode sessions do not reload slash commands.

You can also run the release script directly:

```bash
scripts/release.sh
scripts/release.sh patch
scripts/release.sh minor
scripts/release.sh major
```

Use the manual release process below as a fallback when debugging or recovering from a script failure.

## Manual Release Process

1. Choose a version string, such as `1.0.0`.
2. Confirm you are on `main`, local `main` matches `origin/main`, the worktree has no tracked or untracked changes, `VERSION` is `dev`, and `gh auth status` succeeds.
3. Update `VERSION` to `1.0.0` without committing it yet.
4. Run the full verification command set:

   ```bash
   go test ./... -count=1
   go vet ./...
   go mod tidy -diff
   go test -race ./... -count=1
   go build ./cmd/opentunnel
   rm -f ./opentunnel
   ```

5. Commit the release: `git add VERSION && git commit -m "chore: release 1.0.0"`.
6. Create and push tag `1.0.0` at that commit.
7. Publish a GitHub Release tagged `1.0.0` from that commit.
8. Reset `VERSION` to `dev`, commit it, and push `main`.
9. Wait for the release workflow to publish `ghcr.io/akoenig/opentunnel:1.0.0` and `ghcr.io/akoenig/opentunnel:latest`.
10. Start the relay with `docker run -p 8080:8080 ghcr.io/akoenig/opentunnel:1.0.0 relay --public-url https://relay.example.com`.
11. Verify `/cli`.
12. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-linux-amd64` and `/cli/bin/opentunnel-1.0.0-linux-amd64.sha256`.
13. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-linux-arm64` and `/cli/bin/opentunnel-1.0.0-linux-arm64.sha256`.
14. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-darwin-amd64` and `/cli/bin/opentunnel-1.0.0-darwin-amd64.sha256`.
15. Verify each artifact plus checksum: `/cli/bin/opentunnel-1.0.0-darwin-arm64` and `/cli/bin/opentunnel-1.0.0-darwin-arm64.sha256`.
16. Verify the public flow: `curl -fsSL https://relay.example.com/cli | sh -s -- create`, then run the generated `curl -fsSL https://relay.example.com/cli | OPENTUNNEL_INVITE='<invite>' sh -s -- exec -- '<COMMAND>'` command.

Artifact filenames are derived from `VERSION`. Development builds with `VERSION=dev` produce `/cli/bin/opentunnel-dev-*` paths instead of `opentunnel-1.0.0-*` paths. Prefer immutable GHCR version tags for production; `latest` is mutable.

## Checksum Boundary

The relay serves same-origin checksums for corruption and mismatch detection. These checksums are not a strong supply-chain security boundary. If the trusted relay origin is compromised, an attacker can change the bootstrapper, binary, and checksum together.
