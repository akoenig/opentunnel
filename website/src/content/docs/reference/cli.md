---
title: CLI Reference
description: The opentunnel subcommands (create, exec, and relay) and their flags.
---

The `opentunnel` binary has three subcommands. In normal use you never install it: the `/cli` bootstrapper downloads a temporary, checksum-verified copy and passes your arguments through, so `curl -fsSL https://opentunnel.sh/create | sh` and `opentunnel create` are equivalent.

## `create`

Starts a foreground host session on the current machine and prints the agent prompt.

```bash
curl -fsSL https://opentunnel.sh/create | sh
```

| Flag | Default | Purpose |
|---|---|---|
| `--relay` | bootstrap origin | Relay origin (`http(s)://host[:port]`, no path). When bootstrapped via `/cli`, this is supplied automatically through `OPENTUNNEL_RELAY_ORIGIN`. |

The session stays open until Ctrl+C, idle timeout, relay failure, or process exit. Exiting revokes all access.

## `exec`

Connects to an active session and runs one command. This is the command your agent uses.

```bash
curl -fsSL https://opentunnel.sh/cli | OPENTUNNEL_INVITE='<invite>' sh -s -- exec \
  -- 'hostname && uname -a'
```

| Flag | Default | Purpose |
|---|---|---|
| `OPENTUNNEL_INVITE` | preferred | Environment variable carrying the invite printed by `create`. Keeps the bearer secret out of process command lines. |
| `--invite-stdin` | off | Read the invite from stdin for stronger local secrecy. |
| `--invite` | supported | Pass the invite as a flag. Compatible but places the secret in process argv; prefer the alternatives above. |

The invite contains everything needed to connect, including the relay origin. Everything after `--` is the command to execute on the host. Remote stdout and stderr stream to the local stdout and stderr, and `exec` exits with the remote command's exit code, which is what lets agents treat it like a local tool call.

Commands must be non-interactive: no PTY, no stdin. One client and one command run at a time.

## `relay`

Runs a relay server. Only operators need this; see [self-hosting](/guides/self-hosting/).

```bash
opentunnel relay --public-url https://relay.example.com
```

| Flag | Default | Purpose |
|---|---|---|
| `--public-url` | *(required)* | Public origin embedded into the `/cli` bootstrapper. Must be a bare `http(s)` origin without path, query, or userinfo. |
| `--listen` | `:8080` | HTTP listen address. |
| `--artifact-dir` | `/opentunnel-artifacts` | Directory containing CLI artifacts named like `opentunnel-1.0.0-linux-amd64`. |
| `--version` | build version | Version string used to resolve artifact filenames. |

## Exit codes

`exec` propagates the remote command's exit code. All subcommands exit `2` on argument errors and `1` on runtime failures.
