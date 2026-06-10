# OpenTunnel Root README Design

## Purpose

Add a concise root `README.md` so the GitHub repository has an immediate project overview and clear links into the existing public v1 documentation.

## Scope

In scope:

- Create a root `README.md`.
- Summarize OpenTunnel in one or two short paragraphs.
- Show the public host/client `/cli` command shape.
- Show the local build and relay command for quick self-hosted testing.
- List standard verification commands.
- Link to public v1 docs for self-hosting, operations, security, non-goals, and acceptance mapping.
- State invite bearer-secret handling and checksum boundary honestly.

Out of scope:

- Long-form tutorial content already covered by `docs/public-v1/`.
- Marketing copy, badges, screenshots, or diagrams.
- New runtime behavior or deployment artifacts.

## Content Structure

The README should use these sections:

- `# OpenTunnel`
- `## What It Is`
- `## Status`
- `## Quick Start`
- `## Public Command Shape`
- `## Verification`
- `## Documentation`
- `## Security Notes`
- `## Non-Goals`

The README should remain concise and point to deeper documentation instead of duplicating it.

## Accuracy Requirements

- Do not claim package-manager distribution, accounts, dashboards, or hosted service features.
- Do not claim strong supply-chain security for `curl | sh`.
- Describe same-origin checksums as corruption or mismatch detection within the trusted relay-origin model.
- Describe invites as bearer-secret material.
- Keep commands aligned with current CLI flags: `relay --listen --public-url --artifact-path --version`, `create`, and `exec --invite --`.

## Verification

The README work is complete when:

- `README.md` exists at the repository root.
- The README links to all current public v1 docs.
- README command snippets match current CLI behavior.
- Repository tests still pass.
