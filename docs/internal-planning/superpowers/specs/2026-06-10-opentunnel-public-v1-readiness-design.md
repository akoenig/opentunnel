# OpenTunnel Public V1 Readiness Design

## Purpose

Milestone 5 packages the current encrypted tunnel, temporary `/cli` distribution, and foreground session model into a public v1-ready shape. The goal is not to add new product surfaces. The goal is to make the intended v1 experience clear, documented, and verifiable against `docs/internal-planning/plan.md`.

## Scope

In scope:

- Polish the generated `create` prompt so it matches the public agent handoff described in `docs/internal-planning/plan.md`.
- Document self-hosted relay operation.
- Document `/cli` temporary distribution and cache behavior.
- Document security and trust boundaries honestly.
- Document explicit v1 non-goals and deferred capabilities.
- Add an acceptance mapping from `docs/internal-planning/plan.md` to implemented behavior or explicit deferrals.
- Verify the public host and client UX with curl-piped `/cli` commands.

Out of scope:

- Accounts, login, teams, dashboards, or tokens.
- Package-manager distribution.
- Install-to-system flows.
- Strong supply-chain security claims for `curl | sh`.
- PTY, stdin, file transfer, raw SSH, MCP, approval workflows, or multi-client sessions.
- Persistent relay state, persistent audit logs, or command-log storage.

## Product UX

The public host command remains:

```bash
curl -fsSL <relay>/cli | sh -s -- create
```

The generated prompt should be suitable for a user to paste into an AI agent. It should include:

- A short statement that an OpenTunnel session is open.
- A command template:

```bash
curl -fsSL <relay>/cli | sh -s -- exec \
  --invite '<invite>' \
  -- '<COMMAND>'
```

- A suggested first command:

```bash
curl -fsSL <relay>/cli | sh -s -- exec \
  --invite '<invite>' \
  -- 'hostname && uname -a && pwd'
```

- A warning that the invite is bearer-secret material.
- A note that commands execute without per-command approval while the foreground session runs.
- Host guidance: use non-interactive commands, avoid sudo unless passwordless, avoid long-running commands unless necessary, one client and one command at a time, temporary CLI cached during the session.

The public prompt must not require the user or agent to pass a relay flag. Relay origin should come from the `/cli` bootstrapper context or the invite.

## Documentation

Add public v1 documentation under `docs/` with these pages:

- `docs/public-v1/self-hosting.md`: how to build and run a self-hosted relay, including `--public-url`, `--artifact-path`, and `--version`.
- `docs/public-v1/security.md`: trust boundaries, relay blindness, invite bearer-secret handling, same-origin checksum limitations, and what the relay can and cannot see.
- `docs/public-v1/non-goals.md`: explicit v1 exclusions from `docs/internal-planning/plan.md`.
- `docs/public-v1/acceptance.md`: acceptance criteria mapped back to `docs/internal-planning/plan.md`, marking implemented behavior and explicit deferrals.

The docs should avoid claiming that same-origin checksums make `curl | sh` supply-chain secure. They are corruption detection within the relay-origin trust model.

## Components

### CLI Prompt Rendering

Keep prompt rendering in `cmd/opentunnel` close to the existing `create` command. Add focused tests around the rendered output rather than introducing a broad templating system.

The prompt must preserve the existing safety guarantees:

- The invite is printed only as the `ot1_...` bearer string.
- The relay origin is validated as an HTTP(S) origin before it is printed in shell commands.
- The generated shell snippets remain safe for the accepted invite alphabet and relay-origin format.

### Relay And Bootstrapper

No new relay protocol behavior is required for M5. The existing M4 `/cli`, artifact, checksum, and temporary cache behavior should be used as-is unless verification finds a public-UX gap.

### Docs

Docs should describe current behavior precisely. If an item from `docs/internal-planning/plan.md` is not implemented, the acceptance mapping should either mark it deferred or call it out as a gap for a follow-up task. Documentation must not paper over missing behavior.

## Error Handling

The public UX should retain the existing concise CLI error style. M5 should not introduce new long-running background flows or persistent state.

For documentation, error cases should focus on operator-actionable guidance:

- Relay URL must be a public HTTP(S) origin.
- The relay must serve a compatible artifact path and version.
- The invite is invalid if copied incorrectly or used after the host exits.
- Ctrl+C on the host revokes access by ending the foreground session.

## Testing And Verification

M5 passes only when these checks succeed:

- Prompt-rendering tests cover the public command template, suggested first command, bearer-secret warning, non-interactive guidance, and absence of user-facing `--relay`.
- Existing M1-M4 tests still pass.
- Manual public host flow works:

```bash
curl -fsSL <relay>/cli | sh -s -- create
```

- Manual generated client prompt works with `exec` through `/cli`.
- Verification commands pass:

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go mod tidy -diff
go build ./cmd/opentunnel
```

The generated root `opentunnel` binary from `go build ./cmd/opentunnel` should be removed after verification.

## Acceptance Criteria

Milestone 5 is complete when:

- Public self-hosting documentation exists.
- Security and trust-boundary documentation exists and avoids strong supply-chain claims.
- Non-goals are documented.
- Acceptance mapping to `docs/internal-planning/plan.md` exists.
- Generated prompt is clear enough to paste into an AI agent and matches the v1 command shape.
- Public `/cli` create and exec flows are manually verified.
- Full tests, race tests, vet, tidy diff, and build pass.
