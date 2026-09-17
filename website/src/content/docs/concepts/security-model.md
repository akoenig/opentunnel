---
title: Security Model
description: Bearer address, peer pinning, session lifetimes, the local audit log, and what OpenTunnel does not protect against.
---

OpenTunnel is an ephemeral, end-to-end encrypted command tunnel between two machines that belong to the same person. This page describes the boundaries precisely, including what OpenTunnel does *not* protect against.

## The address is a bearer secret, briefly

The tunnel address printed by the host is all it takes to claim a fresh session. It will end up in your agent's context and in its model provider's logs, so the design makes it worthless as quickly as possible.

Until the claim, anyone who has the address can connect. The window is bounded by `OPENTUNNEL_CLAIM_TIMEOUT` (5 minutes by default), and the address exists only in your clipboard and your agent's context during it.

After the claim, the host restarts the tunnel with the claiming key as its only allowed peer. Other keys are dropped at the WireGuard layer, before any handshake completes. The matching private key is generated on the agent machine, stays in its temp directory, and is removed with it.

Exactly one claim is accepted. A malformed claim, or a claim from a key you were not expecting, ends the session: the host logs the key prefix and exits instead of waiting for a second attempt. Run the host command again for a fresh address.

## Keys and state

Both sides keep everything in one `mktemp -d` directory with mode 700: the binary, the key, the session state. The host's tunnel key is generated per session and never reused, so an address that leaks after the fact is useless. Ending the session removes both directories.

There is no relay state to reason about. Public relays forward encrypted WireGuard traffic and see routing metadata, timing, and packet sizes.

## Lifetime and revocation

Access is coterminous with the host process. Ctrl+C ends it. So does the idle timeout (30 minutes by default, counted when no command is running and none has started), the optional hard limit (`OPENTUNNEL_TTL`), and any failure of the tunnel process.

An agent that keeps issuing commands keeps an idle-limited session alive indefinitely. The host prints a heartbeat with the number of running sessions and commands, and the audit log records each command line. Set `OPENTUNNEL_TTL` when you want a cap that an active agent cannot extend.

## Execution semantics

Every SSH session on the remote machine runs one wrapper script in place of a shell (OpenSSH calls this a forced command). The wrapper:

- refuses sessions without a command, so there is no interactive shell and no PTY;
- appends one line per command to the session audit log: timestamp, session id, peer key, peer address, and the command line, never the data piped through it, followed by a second line with the exit code;
- marks the session as running so the idle timer cannot end the session under an active command;
- runs the command with `bash -lc` in the session working directory, so your agent gets the same environment you would get on login.

There is no command allowlist. Granting a tunnel means granting command execution as your user for the lifetime of the session, including reading and writing files. Scope what the agent can reach accordingly, and end the session when the task is done.

Because commands run in a login shell, anything your shell profile prints on startup is mixed into command output. That also affects `remote --get`, which streams a file through the same channel.

## Watching the session

The terminal that opened the tunnel shows every command as it starts and its exit code when it ends, prefixed with the session id so concurrent commands stay readable. The wrapper writes that line to the terminal itself, before it runs the command, and it replaces control characters first, so a command line cannot carry terminal escape sequences onto your screen. Set `OPENTUNNEL_SHOW_COMMANDS=0` if you would rather have a quiet terminal; the records still go to the audit log.

Be precise about what this is: a view for you, not a barrier for a hostile agent. Anything that runs as your user can also write to your terminal, so a determined attacker with command execution can forge lines or erase them by other means. What the design guarantees is narrower and still useful: an honest agent's activity is always visible, and the cheap tricks (escape sequences in the command line, truncating the audit log) are neutralized or reported.

## If the client key leaks

The agent's client key is the only credential after the claim. Whoever holds it, together with the address that sits next to it in the same directory, has full command execution as your user from any machine, for the rest of the session. There is no second factor and no way to tell the thief from the agent in the audit log: both records carry the same key and the same tunnel address, because both derive from that key. Attribution in the audit log is to a key, not to a machine.

It does end with the session. Once the host process exits the server key is gone and the address never works again. Keep sessions short, use `OPENTUNNEL_TTL` for unattended work, and treat the agent machine as fully trusted, because it is.

## If the supervisor dies

The host script is the process that enforces the lifetime and removes the keys. A command can kill it, and so can an out-of-memory kill. Every SSH session checks that the supervisor is still there before running its command; if it is gone, the wrapper refuses the command, ends the tunnel server, and removes the session directory, so an orphaned tunnel cannot keep serving. This closes the accidental case completely. A hostile agent can defeat it by editing the wrapper first, which is the same as saying that an attacker running as your user can do anything you can.

## The audit log

The audit log lives on the remote machine, in the session's temp directory, and is removed with it. Set `OPENTUNNEL_KEEP_AUDIT=1` to copy it next to where you started the host when the session ends. It records command lines only, never payloads.

## Binaries and checksums

Both scripts pin the sha256 of every tailcat binary they can download, in the script itself, and refuse to run a binary that does not match. The binaries are built from a pinned upstream commit and served over HTTPS from the same origin as the scripts.

This is not a defence against a compromised origin: whoever can change the binary can change the script that pins it. It detects corruption, mismatched artifacts, and a tampered download path. Verify the script before piping it into a shell if that matters to you; it is plain text and it is short.

## Known residual risks

- **The claim race.** Whoever connects first is pinned. Keep the window short and do not publish the address.
- **The agent machine is fully trusted.** Its model provider sees everything the agent sends and receives.
- **No sandbox.** Commands run as your user, with your environment.
- **Public relays.** They can rate-limit or become unavailable. A self-hosted relay is a one-flag change in a future release.
- **tailcat is experimental.** Its own threat model assumes a single trusted operator on both ends, which is exactly this use case. It has not had a third-party audit.
