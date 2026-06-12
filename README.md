<div align="center">

# OpenTunnel

**Your agent's tool calls, on any machine.**

OpenTunnel gives AI agents an ephemeral, end-to-end encrypted command tunnel to remote machines.
No SSH, no accounts, no standing access. Ctrl+C and it's gone.

[![CI](https://github.com/akoenig/opentunnel/actions/workflows/ci.yml/badge.svg)](https://github.com/akoenig/opentunnel/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/akoenig/opentunnel)](https://github.com/akoenig/opentunnel/releases)

[opentunnel.sh](https://opentunnel.sh) · [Getting Started](https://opentunnel.sh/getting-started/) · [How It Works](https://opentunnel.sh/concepts/how-it-works/) · [Security Model](https://opentunnel.sh/concepts/security-model/) · [Self-Hosting](https://opentunnel.sh/guides/self-hosting/)

</div>

---

## Your agent ends at localhost

Agents are brilliant on the machine they run on. The moment the task lives on another machine (a server, a build box, your homelab) they hit a wall of SSH keys, firewall rules, and standing credentials. Permanent infrastructure for a temporary need.

OpenTunnel removes the wall without creating permanent access. You start one foreground process on the remote machine and paste the printed prompt into your agent. From then on, the agent runs commands there like any other tool call: stdout, stderr, and the real exit code come back as if the machine were local. When the task is done, you press Ctrl+C. The session ends, the invite expires, and no trace of the access remains.

## Sixty seconds to a working tunnel

**1 · On the remote machine**

```console
$ curl -fsSL https://opentunnel.sh/cli | sh -s -- create

I opened an OpenTunnel session for you.
Session active. Press Ctrl+C to revoke access.
```

A temporary CLI is downloaded, checksum-verified, and opens one foreground session. Nothing is installed.

**2 · In your agent**

Paste the printed prompt. Your agent now has a remote shell as a tool and runs commands like this on its own:

```bash
curl -fsSL https://opentunnel.sh/cli | sh -s -- exec \
  --invite '<invite>' \
  -- 'systemctl restart caddy'
```

Remote stdout and stderr stream back, and `exec` exits with the remote command's exit code. To the agent, the remote machine feels local.

**3 · Press Ctrl+C when you're done**

The session ends, the invite expires, and the relay forgets the connection ever existed. Nothing persists, not on your machine and not on the relay.

## You don't have to trust the relay

The relay routes opaque, encrypted frames between your agent and the remote machine. It cannot read your traffic, so it doesn't matter who operates it.

- **End-to-end encrypted.** Commands, output, and exit codes are encrypted between host and client. The relay forwards ciphertext and sees only routing metadata, timing, and frame sizes.
- **Nothing persisted.** Only in-memory state for active connections. No sessions, invites, payloads, logs, or client metadata are ever stored.
- **Revocation is Ctrl+C.** Access lives exactly as long as the foreground host process. Stop it, and the tunnel ceases to exist.
- **No accounts, no keys.** No signup, no tokens to rotate, no SSH keys to distribute and forget. A session invite is the only secret, and it expires with the session.

The boundaries, including what OpenTunnel does *not* protect against, are documented precisely in the [security model](https://opentunnel.sh/concepts/security-model/).

## Run your own relay

Because the relay needs no database, no accounts, and no persistent state, self-hosting is one command:

```bash
docker run -p 8080:8080 ghcr.io/akoenig/opentunnel:latest \
  relay --public-url https://relay.example.com
```

Sessions started from your origin print agent prompts that point there automatically:

```bash
curl -fsSL https://relay.example.com/cli | sh -s -- create
```

Prefer immutable version tags in production; `latest` moves with each release. The [self-hosting guide](https://opentunnel.sh/guides/self-hosting/) and [relay operations](https://opentunnel.sh/guides/relay-operations/) cover TLS termination, systemd, upgrades, and deployment verification.

## Deliberately small

OpenTunnel keeps the access model temporary and narrow on purpose. That is the security model, not a missing feature list: no accounts, no daemons, no audit logs (because there is nothing to log), no PTY, no file transfer, one agent, one command at a time. The full list lives in [scope and non-goals](https://opentunnel.sh/reference/scope-and-non-goals/).

## Development

OpenTunnel is a single Go module. The `opentunnel` binary contains all three roles: the relay, the host (`create`), and the client (`exec`).

```bash
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
```

CI builds binaries for `linux` and `darwin` on `amd64` and `arm64`, and releases publish `ghcr.io/akoenig/opentunnel`. The [opentunnel.sh](https://opentunnel.sh) website lives in [`website/`](website/), and the operational source docs in [`docs/public-v1/`](docs/public-v1/).

---

<div align="center">

Built by <a href="https://andrekoenig.com">André König</a>

</div>
