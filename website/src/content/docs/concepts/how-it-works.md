---
title: How It Works
description: The three actors in an OpenTunnel session (host, client, and relay) and what travels between them.
---

An OpenTunnel session involves three actors and deliberately little else.

## The actors

**The host** is the foreground process on the remote machine, started with `create`. It generates the session invite, executes incoming commands, and defines the session's lifetime. Commands have no duration deadline and run until they finish, the client disconnects, or the host operator presses Ctrl+C. A separate 30-minute idle timer runs only between commands and closes forgotten sessions.

**The client** is the temporary CLI your agent invokes with `exec`. It connects through the relay using the invite, sends one encrypted command, streams back encrypted stdout and stderr, and exits with the command's real exit code.

**The relay** is a stateless rendezvous point. Host and client both dial out to it over WebSockets, which is what makes the whole thing work without inbound firewall rules on either side. The relay matches the two connections and forwards opaque encrypted frames between them.

## A session, start to finish

1. `create` downloads the temporary CLI from the relay's `/cli` endpoint, verifies its checksum against the same origin, and starts the host process.
2. The host connects out to the relay, generates invite material, and prints the agent prompt. The invite contains everything the client needs, including the relay origin, which is why the agent-facing command has no relay flag.
3. Your agent runs `exec` with the invite. The client bootstraps the same way (download, checksum, run), connects to the relay, and establishes an end-to-end encrypted channel with the host using the invite material.
4. The command travels encrypted through the relay. The host pauses the idle timer, executes the command without a duration deadline, and streams stdout, stderr, and the exit code back, also encrypted. After the command finishes, the host starts a new 30-minute idle period while it waits for another command.
5. Ctrl+C on the host tears everything down. The relay's in-memory connection state evaporates; the cached temporary CLI is left only in the system temp directory.

## What each party sees

| | Commands & output | Invite secrets | Routing metadata |
|---|---|---|---|
| Host | plaintext (it executes them) | generates them | yes |
| Client | plaintext (it sends/receives them) | holds them | yes |
| Relay | **ciphertext only** | never | yes: roles, timing, frame sizes |

This is the property the whole design leans on: the relay can route your traffic without being able to read it, so the question "who operates the relay?" stops being a trust decision. The details live in the [security model](/concepts/security-model/).

## The temporary CLI

There is nothing to install. The `/cli` endpoint serves a small bootstrapper with the relay origin baked in; it picks the right binary for your platform (Linux/macOS, amd64/arm64), downloads it from the same origin, verifies the checksum, and runs it. During an active session the binary is cached in a private temp path, and cache hits are checksum-verified again. When the session is over, no system-level installation has taken place.
