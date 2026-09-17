<div align="center">

# OpenTunnel

**Your agent's tool calls, on any machine.**

OpenTunnel gives AI agents an ephemeral, end-to-end encrypted command tunnel to remote machines.
No accounts, no standing access. Ctrl+C and it's gone.

[![CI](https://github.com/akoenig/opentunnel/actions/workflows/ci.yml/badge.svg)](https://github.com/akoenig/opentunnel/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/akoenig/opentunnel)](https://github.com/akoenig/opentunnel/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[opentunnel.sh](https://opentunnel.sh) · [Getting Started](https://opentunnel.sh/getting-started/) · [How It Works](https://opentunnel.sh/concepts/how-it-works/) · [Security Model](https://opentunnel.sh/concepts/security-model/)

</div>

---

## Great agents, stuck on one machine.<br>Let's fix that.

Agents are brilliant on the machine they run on. The moment the task lives on another machine, they hit a wall of key distribution, firewall rules, and standing credentials. Permanent infrastructure for a temporary need.

OpenTunnel removes the wall without creating permanent access. You start one foreground process on the remote machine and paste the printed prompt into your agent. From then on, the agent runs commands there like any other tool call: stdout, stderr, and the real exit code come back as if the machine were local. When the task is done, you press Ctrl+C. The session ends and the keys that made it possible are gone.

> **Beta.** v2 is published at `https://beta.opentunnel.sh`. The apex domain keeps serving 1.x until v2 is generally available. The 1.x relay implementation lives on the [`1.x`](https://github.com/akoenig/opentunnel/tree/1.x) branch.

## Three steps, zero infrastructure

**1 · On the remote machine**

```console
$ curl -fsSL https://beta.opentunnel.sh | sh

[opentunnel] downloading tailcat (2.0.0, linux/amd64)
[opentunnel] waiting for the agent to claim (timeout 5m). Ctrl-C to abort.
```

A temporary binary is downloaded, checksum-verified against the checksum pinned in the script itself, and one foreground session starts. Nothing is installed.

**2 · In your agent**

Paste the printed prompt. It tells the agent to run the one-line installer, and from that moment the agent runs commands on the remote machine as regular tool calls:

```bash
curl -fsSL https://beta.opentunnel.sh/agent | sh -s -- tc...
/tmp/opentunnel-agent.XXXXXX/remote 'uname -a && pwd'
```

**3 · Press Ctrl+C when you're done**

The session ends, both temp directories are removed, and the address stops working forever. Nothing persists on either machine.

## The address stops working the moment your agent connects

The tunnel address is a bearer secret, and it will end up in your agent's context and its provider's logs. So OpenTunnel makes it worthless as early as possible.

- **Claim, then pin.** The first connection to a fresh session delivers exactly one thing: the agent's public key. The host then restarts the tunnel restricted to that key. From then on the address alone gets nobody in, and the matching private key never leaves the agent's temp directory.
- **End-to-end encrypted.** Traffic is WireGuard between the two machines, with direct paths when they can be found and public relays otherwise. A relay forwards ciphertext, and there is no control plane and no account anywhere.
- **Bounded by default.** An unclaimed session expires after 5 minutes, an idle session after 30 minutes, and Ctrl+C ends it at any moment. An optional hard limit caps the total.
- **Ephemeral keys.** The server key is generated per session and removed with the temp directory, so a leaked address is useless afterwards.
- **A local audit trail.** Every command line is recorded in the session's audit log on the remote machine. It is removed with the session unless you ask to keep it.

What OpenTunnel does *not* protect against is documented in [Security notes](#security-notes) and, in full, in the [security model](https://opentunnel.sh/concepts/security-model/).

## The `remote` helper

The agent installer prints one line: `remote helper: /tmp/opentunnel-agent.XXXXXX/remote`. That helper is the whole client surface.

```bash
remote 'cd repo && go test ./...'      # exit code comes back unchanged
remote --put ./patch.diff /tmp/patch.diff
remote --get /var/log/build.log ./build.log
remote --close                         # end this client and remove its keys
```

Files also move with the standard tools, through the `ssh` wrapper written next to the helper:

```bash
rsync -av -e /tmp/opentunnel-agent.XXXXXX/ssh ./dist opentunnel:/srv/app
scp -O -S /tmp/opentunnel-agent.XXXXXX/ssh ./file opentunnel:/srv/app/
```

Commands may run concurrently: they share one connection, so a burst of tool calls does not open a burst of tunnels.

## Tunables

All of them are environment variables, all of them are seconds unless noted.

| Variable | Default | Effect |
|---|---|---|
| `OPENTUNNEL_CLAIM_TIMEOUT` | `300` | How long the host waits for the agent to claim the tunnel. |
| `OPENTUNNEL_IDLE` | `1800` | End the session after this long with no command running and none started. |
| `OPENTUNNEL_TTL` | `0` (off) | Hard limit on the total session length. |
| `OPENTUNNEL_CWD` | `$PWD` | Working directory for remote commands. |
| `OPENTUNNEL_KEEP_AUDIT` | unset | `1` copies the audit log next to where you started the host. |
| `OPENTUNNEL_CONNECT_TIMEOUT` | `90` | Agent side: how long to wait for the tunnel to come up. |
| `OPENTUNNEL_CONTROL_PERSIST` | `600` | Agent side: how long the shared connection stays open when idle. |
| `OPENTUNNEL_BASE_URL` | `https://beta.opentunnel.sh` | Where both scripts download from. |

## Security notes

Mitigated: address leakage after the claim (the peer is pinned), stale addresses (the server key is ephemeral), unattended sessions (claim window, idle timeout, optional hard limit, Ctrl+C), binary tampering (pinned upstream commit, pinned sha256 in the script, HTTPS), interactive shells (the wrapper refuses a session without a command).

Residual, by design:

- **The unclaimed window is a race.** Whoever connects first is pinned. The window is at most 5 minutes and the address exists only in your clipboard and your agent's context. If a foreign key claims it, the host logs the key prefix and ends; you rerun for a fresh address.
- **The agent gets your account.** For the lifetime of the session, the agent (and its model provider) can run anything you can, including reading and writing files. There is no command allowlist. `audit.log` records every command line, never payloads.
- **Without a hard limit, an active agent keeps the session alive.** The heartbeat and the audit log make that visible, and Ctrl+C always ends it.
- **Public relays can rate-limit or disappear.** Beta uses the public Tailscale DERP relays.
- **Upstream tailcat is experimental.** Its threat model assumes one trusted operator on both ends, which is exactly this use case.

## Development

OpenTunnel v2 is bash plus a pinned build of [tailcat](https://github.com/tailscale/tailcat).

```bash
shellcheck -s bash scripts/*.sh build/*.sh test/e2e.sh
bats test/unit
OPENTUNNEL_SKIP_CHECKSUMS=1 build/embed.sh   # writes dist/host.sh and dist/agent.sh

build/build-tailcat.sh                       # needs Go, writes dist/bin/<version>/
build/embed.sh
test/e2e.sh                                  # real session on this machine, needs network
```

| Path | What it is |
|---|---|
| `scripts/host.sh` | Served at `/`. Opens the session, prints the prompt, supervises the lifetime. |
| `scripts/agent.sh` | Served at `/agent`. Claims the tunnel and writes the `remote` helper. |
| `scripts/ot-exec.sh` | Runs on the remote machine for every command: audit, activity, no interactive shells. |
| `build/` | Builds tailcat from `build/TAILCAT_COMMIT` and embeds version, checksums, and the wrapper. |
| `site/` | The Cloudflare Worker serving `dist/` at beta.opentunnel.sh. |
| `website/` | The [opentunnel.sh](https://opentunnel.sh) site. |

During the beta every push to `main` that touches the scripts, the build, or the site deploys to `beta.opentunnel.sh` as version `dev`, so the current state of `main` is always test drivable with the normal one-liner. Those binaries live at `/bin/dev/` and are served uncached; released versions are immutable.

Cutting a release is `scripts/release.sh [patch|minor|major]`: it bumps `VERSION`, runs the verification set, tags, publishes, and reopens development. The tag push builds the binaries and deploys the beta Worker.

OpenTunnel builds on [tailcat](https://github.com/tailscale/tailcat) by Tailscale (BSD-3-Clause). The binaries we serve are built from a pinned upstream commit and ship with `LICENSE.tailcat`.

---

<div align="center">

Built by <a href="https://andrekoenig.com">André König</a> · Released under the <a href="LICENSE">MIT License</a>

</div>
