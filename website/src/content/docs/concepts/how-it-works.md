---
title: How It Works
description: The two actors in an OpenTunnel session, the claim that pins the tunnel to one agent, and what travels between them.
---

An OpenTunnel session involves two machines and deliberately little else.

## The actors

**The host** is the foreground process on the remote machine, started with `curl -fsSL https://beta.opentunnel.sh | sh`. It generates an ephemeral tunnel key, prints the prompt for your agent, runs the commands that arrive, and defines the session's lifetime.

**The agent side** is the machine your coding agent runs on. A one-line installer downloads the same binary, generates a client identity, claims the tunnel, and writes a `remote` helper. Every command the agent runs goes through that helper.

**The transport** is [tailcat](https://github.com/tailscale/tailcat): WireGuard between the two machines, with Tailscale's NAT traversal and its public relays as a fallback path. There is no control plane, no account, and no inbound port on either side. A relay only ever forwards ciphertext.

## A session, start to finish

1. The host script downloads the tailcat binary, verifies it against the sha256 pinned inside the script, and generates a session key. The key never leaves the temp directory. Its tunnel address goes into the prompt.
2. **Claim.** The host listens for exactly one connection. The agent installer connects and sends one thing: the public key of the client identity it just generated. The host validates it and stops listening.
3. **Pin.** The host restarts the tunnel on the same address, now restricted to that one key. Every other key is ignored at the WireGuard layer, so the address by itself no longer gets anyone in.
4. The agent opens one shared SSH connection over the tunnel. Each command the agent runs becomes a session on that connection, so commands can run concurrently without opening a second tunnel.
5. On the remote machine every session runs the same wrapper: it refuses sessions without a command, records the command line in the session audit log, marks the session as active, and runs the command with your login environment in the session working directory.
6. Ctrl+C on the host ends everything. Both temp directories are removed, the ephemeral key is gone, and the address is permanently useless.

## Lifetime

Nothing keeps a forgotten session open.

| Limit | Default | What it does |
|---|---|---|
| Claim window | 5 minutes | If no agent claims the tunnel, the session ends. |
| Idle timeout | 30 minutes | With no command running and none started, the session ends. |
| Hard limit | off | `OPENTUNNEL_TTL` caps the total session length, even during an active command. |
| Ctrl+C | always | Ends the session immediately. |

A malformed or foreign claim ends the session too. The host does not go back to waiting: you run the command again and get a fresh address.

## What each party sees

| | Commands and output | Tunnel address | Private keys |
|---|---|---|---|
| Host | plaintext (it runs them) | generates it | its own, in its temp directory |
| Agent machine | plaintext (it sends and receives them) | holds it | its own, in its temp directory |
| Relay | **ciphertext only** | never | never |

## The temporary binary

Nothing is installed. Both scripts download the same tailcat binary for your platform (Linux or macOS, amd64 or arm64) into a private temp directory, verify it against a checksum pinned in the script itself, and remove it when the session ends. The binaries are built from a commit of tailcat pinned in this repository, so the checksum in the script and the binary served always belong together.
