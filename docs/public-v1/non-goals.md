# OpenTunnel Public V1 Non-Goals

OpenTunnel v1 deliberately keeps the access model temporary and narrow.

Not included in v1:

- Accounts, teams, login, tokens, dashboards, or billing.
- Package-manager distribution.
- Install-to-system flows or daemon mode.
- Persistent relay state.
- Persistent command logs, audit logs, payload logs, or client metadata.
- MCP integration.
- Raw SSH compatibility.
- PTY support.
- Interactive stdin.
- File upload or download.
- Approval workflows.
- Multiple simultaneous clients for one tunnel.
- Concurrent command execution in one tunnel.
- Background command management.
- Public relay dashboard or session list.

These exclusions protect the core product principle: one foreground host process, one client, one temporary CLI, and zero persistent relay state.
