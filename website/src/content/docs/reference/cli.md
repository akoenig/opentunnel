---
title: CLI Reference
description: The host script, the agent installer, the remote helper, and the OPENTUNNEL_ environment variables.
---

OpenTunnel is two scripts and one generated helper. Nothing is installed: both scripts download a temporary, checksum-verified binary into a private temp directory and remove it when the session ends.

## Host script

Run this on the machine your agent should reach. It stays in the foreground and prints the prompt to paste into your agent.

```bash
curl -fsSL https://beta.opentunnel.sh | sh
```

It also works with `| bash`, and the script is plain text: curl it without the pipe to read what runs.

| Variable | Default | Purpose |
|---|---|---|
| `OPENTUNNEL_CLAIM_TIMEOUT` | `300` | Seconds to wait for an agent to claim the tunnel. |
| `OPENTUNNEL_IDLE` | `1800` | Seconds with no command running and none started before the session ends. |
| `OPENTUNNEL_TTL` | `0` | Hard limit in seconds on the whole session. `0` disables it. |
| `OPENTUNNEL_CWD` | current directory | Working directory for remote commands. |
| `OPENTUNNEL_KEEP_AUDIT` | unset | `1` copies the session audit log next to where you started the host. |
| `OPENTUNNEL_SHOW_COMMANDS` | `1` | Print each command and its exit code in the host terminal as it runs. `0` turns it off. |
| `OPENTUNNEL_BASE_URL` | `https://beta.opentunnel.sh` | Origin the scripts download from. |

Status lines go to stderr with an `[opentunnel]` prefix. Only the prompt goes to stdout, so `curl ... | sh > prompt.txt` captures exactly the prompt.

While the session runs, the host prints what the agent does:

```text
[opentunnel] active. idle timeout 30m, cwd /srv/app
[opentunnel] 41287 $ uname -sr && pwd
[opentunnel] 41287 exit=0
[opentunnel] 41302 $ ls /nonexistent
[opentunnel] 41302 exit=2
[opentunnel] alive, sessions=0, commands=2
```

The number is the session id, so commands that overlap stay readable. Command lines longer than 200 characters are shortened for display only; the audit log keeps them in full.

The session ends on Ctrl+C, on the idle timeout, on the hard limit, if nobody claims the tunnel in time, or if the claim is malformed.

## Agent installer

Your agent runs this once, with the address from the prompt.

```bash
curl -fsSL https://beta.opentunnel.sh/agent | sh -s -- tc...
```

| Variable | Default | Purpose |
|---|---|---|
| `OPENTUNNEL_CONNECT_TIMEOUT` | `90` | Seconds to wait for the tunnel to come up after claiming it. |
| `OPENTUNNEL_CONTROL_PERSIST` | `600` | Seconds the shared connection stays open while idle. |
| `OPENTUNNEL_BASE_URL` | `https://beta.opentunnel.sh` | Origin the scripts download from. |

It prints exactly one line to stdout, which is what the agent parses:

```text
remote helper: /tmp/opentunnel-agent.XXXXXX/remote
```

It requires `ssh` on the agent machine, and it fails if the tunnel was already claimed by somebody else.

## `remote`

The generated helper is the whole client surface.

```bash
remote '<command>'                 # run a command, exit code passes through
remote --put <local> <remote>      # copy a file to the remote machine
remote --get <remote> <local>      # copy a file from the remote machine
remote --close                     # end this client and remove its keys
```

Commands run non-interactively with your login environment in the session working directory. Several `remote` calls may run at the same time: they share one connection.

Next to the helper there is an `ssh` wrapper for the standard file tools. The host name is ignored; the tunnel address is baked in.

```bash
rsync -av -e /tmp/opentunnel-agent.XXXXXX/ssh ./dist opentunnel:/srv/app
scp -O -S /tmp/opentunnel-agent.XXXXXX/ssh ./file opentunnel:/srv/app/
```

`scp` needs `-O`: OpenSSH 9 and later default to the SFTP subsystem, which the forced command does not serve.

## Exit codes

| Code | Meaning |
|---|---|
| remote command's code | The command ran. `remote` exits with exactly what it returned. |
| `2` | Usage error, or a session that carried no command at all (a bare login shell is refused). |
| `255` | The tunnel is gone: the session ended, or it was never reachable. Report it and stop, do not retry in a loop. |
| `1` | The host or the installer failed. The message says why. |
