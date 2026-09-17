---
title: Scope & Non-Goals
description: What OpenTunnel deliberately does not do, and why that is the security model.
---

OpenTunnel keeps the access model temporary and narrow. The exclusions below are not a roadmap of missing features; they protect the core product principle: **one foreground host process, one pinned client, ephemeral keys, and no persistent state anywhere.**

Every item on this list is something that would create standing access, standing state, or standing infrastructure: exactly what OpenTunnel exists to avoid.

## Not included

**No standing identity or access:**

- Accounts, teams, login, tokens, dashboards, or billing.
- Install-to-system flows or daemon mode.
- Package-manager distribution.

**No standing state:**

- Persistent session state on either machine. Both temp directories are removed when the session ends.
- Audit logs that outlive the session. The session audit log is local to the remote machine and removed with it, unless you copy it out with `OPENTUNNEL_KEEP_AUDIT=1`.

**No expanded execution surface:**

- A PTY from the `remote` helper, so no editors, pagers, or interactive prompts through it. This is a property of the helper, not a restriction the host enforces: a client that asks for a terminal over the tunnel gets one.
- Sandboxing or a command allowlist. Commands run as your user.
- Multiple simultaneous clients for one tunnel. The tunnel is pinned to exactly one.
- Background command management.
- Windows hosts.

**No additional integration surface:**

- MCP integration.
- Approval workflows.

## Included

Some things the first major version excluded are part of the current design:

- **File transfer**, with `remote --put` and `remote --get`, and with `rsync` and `scp` through the generated `ssh` wrapper.
- **Concurrent commands.** Several commands may run at once over one shared connection.
- **SSH as the transport.** The tunnel carries an SSH connection, which is what makes standard tools work unchanged. It is not standing SSH access: no port is exposed, no authorized key is added, and the forced command constrains what a session can do.
- **A session-scoped audit log** on the remote machine, recording the command line of every session. It records what was asked for, not what those commands then did.

## What this means in practice

If your task needs interactive shells, always-on access, or a sandbox, OpenTunnel is the wrong tool. It covers the case those tools handle badly: giving an agent temporary, revocable, end-to-end encrypted command execution on a machine, with nothing to set up beforehand and nothing left behind afterwards.
