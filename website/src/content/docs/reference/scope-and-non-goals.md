---
title: Scope & Non-Goals
description: What OpenTunnel v1 deliberately does not do, and why that is the security model.
---

OpenTunnel v1 deliberately keeps the access model temporary and narrow. The exclusions below are not a roadmap of missing features; they protect the core product principle: **one foreground host process, one client, one temporary CLI, and zero persistent relay state.**

Every item on this list is something that would create standing access, standing state, or standing infrastructure: exactly what OpenTunnel exists to avoid.

## Not included in v1

**No standing identity or access:**

- Accounts, teams, login, tokens, dashboards, or billing.
- Install-to-system flows or daemon mode.
- Package-manager distribution.

**No standing state:**

- Persistent relay state.
- Persistent command logs, audit logs, payload logs, or client metadata.
- Public relay dashboard or session list.

**No expanded execution surface:**

- Raw SSH compatibility.
- PTY support.
- Interactive stdin.
- File upload or download.
- Multiple simultaneous clients for one tunnel.
- Concurrent command execution in one tunnel.
- Background command management.

**No additional integration surface:**

- MCP integration.
- Approval workflows.

## What this means in practice

If your task needs interactive shells, file sync, or always-on access, OpenTunnel is the wrong tool; SSH and its ecosystem do that well. OpenTunnel covers the case those tools handle badly: giving an agent temporary, revocable, end-to-end encrypted command execution on a machine, with nothing to set up beforehand and nothing left behind afterwards.
