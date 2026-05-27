# OpenTunnel V1 Milestone Design

## Purpose

This companion spec turns `plan.md` into a workable delivery sequence. It does not replace the product, security, UX, or non-goal decisions in `plan.md`. It defines what to build first, what each phase must prove, and which later concerns must stay out of earlier milestones.

The goal is to avoid boiling the ocean while still protecting the core OpenTunnel product invariant: ephemeral, relay-routed, end-to-end encrypted remote command execution for AI agents.

## Relationship To `plan.md`

`plan.md` remains the canonical product north star. This document is an execution lens over that plan.

If this document and `plan.md` disagree about product behavior, privacy guarantees, public UX, or non-goals, `plan.md` wins unless the discrepancy is explicitly reviewed and accepted.

Temporary developer affordances described here, such as `opentunnel create --relay http://localhost:8080`, are milestone tooling only. They are not part of the intended public v1 UX, which remains:

```bash
curl -fsSL https://relay.example/cli | sh -s -- create
curl -fsSL https://relay.example/cli | sh -s -- exec --invite '<inviteCode>' -- '<command>'
```

## Guiding Principle

Use vertical milestones that each prove a product invariant. Do not build large components in isolation and do not build a plaintext tunnel as the main implementation path.

The sequencing priority is:

1. Prove the security-bearing protocol.
2. Prove the local end-to-end command loop with real encryption.
3. Harden lifecycle and safety semantics.
4. Add the temporary `/cli` distribution UX.
5. Package and document public v1 honestly.

## Chosen Approach: Vertical Milestones

Vertical milestones are preferred over component-first or demo-first milestones.

Component-first milestones, such as building relay, CLI, command runner, crypto, and distribution separately, are easier to assign by directory but risk retrofitting security too late.

Demo-first milestones, such as building a plaintext tunnel before the secure channel, create quick visible progress but risk anchoring the architecture around the wrong trust model.

Vertical milestones keep engineering progress tied to the product's trust boundaries.

## Milestone 1: Secure Channel Spike

### Purpose

Prove the cryptographic foundation before broader product buildout.

### In Scope

- Isolated Go tests or a small spike package.
- Validation of `github.com/flynn/noise` against v1 requirements.
- Comparison of `Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s` and `Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s`.
- 32-byte raw `clientSecret` mixed as the PSK.
- Host session public key verification against `hostPubKey` from the invite.
- Canonical prologue binding using canonical CBOR or explicit length-prefixed fields.
- Multiple encrypted transport frames after handshake.
- Failure-case tests for wrong `clientSecret`, wrong `hostPubKey`, wrong prologue, malformed frames, and replay or duplicate behavior.
- A narrow `securechannel` interface or written interface design.

### Out Of Scope

- CLI UX.
- Relay server.
- Command execution.
- `/cli` installer or artifact serving.
- Binary caching.
- Product logging.

### Gate

This milestone passes only when:

- One Noise pattern is selected and documented.
- The Go library supports the selected pattern cleanly.
- `clientSecret` is mixed as a 32-byte PSK.
- The client verifies the host session public key from the invite.
- Canonical prologue binding is implemented.
- Multiple encrypted frames can be exchanged after handshake.
- Wrong secret, wrong host key, wrong prologue, malformed frames, and replay or duplicate cases fail predictably.
- A narrow `securechannel` interface exists or is specified.

If this milestone fails, stop and revisit the protocol or library choice before proceeding. Do not continue into relay or command execution work on an unproven secure-channel foundation.

## Milestone 2: Local Binary Vertical Slice

### Purpose

Prove the real OpenTunnel command loop using local binaries, before adding public `/cli` distribution.

### Temporary Developer UX

```bash
opentunnel relay --listen :8080 --public-url http://localhost:8080
opentunnel create --relay http://localhost:8080
opentunnel exec --invite 'ot1_...' -- 'hostname && uname -a && pwd'
```

The `--relay` flag is developer milestone tooling. The public v1 host/client UX should still infer relay origin from `/cli` later.

### In Scope

- `opentunnel relay` with an in-memory route table.
- `opentunnel create --relay ...` for local development.
- `opentunnel exec --invite ... -- '<command>'`.
- Real invite generation and local invite decoding.
- Client sends only routing material to the relay.
- Relay routes opaque encrypted packets only.
- E2E encrypted command request using the secure channel from Milestone 1.
- Streaming encrypted stdout and stderr back to the client.
- Remote exit-code propagation.
- One active command at a time as a simple invariant.
- Host disconnect or relay restart ends the session.

### Out Of Scope

- `/cli` installer.
- Binary caching.
- Artifact checksums.
- Public installer UX.
- Polished deployment.
- Advanced abuse controls.
- Prometheus metrics or monitoring endpoints.
- PTY, stdin, file transfer, background command management, or interactive sessions.

### Gate

This milestone passes only when:

- The host can create an invite.
- The client can decode the invite locally.
- The relay receives only routing material and unavoidable transport metadata.
- The command request is encrypted end-to-end.
- Stdout and stderr stream back encrypted.
- The client exits with the remote command's exit code.
- Relay restart or host disconnect kills the session.
- The relay cannot read the command, output, exit code, host metadata, or `clientSecret`.

## Milestone 3: Lifecycle And Safety Hardening

### Purpose

Turn the local binary vertical slice into the intended temporary access model.

### In Scope

- One active client connection per tunnel.
- One active command per tunnel.
- Rejection of a second active client.
- Rejection of concurrent commands.
- Fixed command timeout.
- Fixed idle session timeout.
- Combined stdout/stderr output limit and truncation behavior.
- Process-group cleanup where supported.
- Graceful then forced command termination on timeout, disconnect, idle shutdown, or host Ctrl+C.
- Readable host-side local status logs.
- Semantic error mapping using stable `PascalCase` error types suffixed with `Error`.

### Out Of Scope

- Approval mode.
- First-client pinning.
- Persistent audit logs.
- Explicit command cancellation protocol.
- Background process management.
- Configurable policy profiles.
- Multiple clients or concurrent command execution.

### Gate

This milestone passes only when:

- A second active client is rejected.
- Concurrent command execution is rejected.
- Command timeout works.
- Output limit and truncation work.
- Idle timeout closes forgotten sessions.
- Ctrl+C closes the tunnel and attempts process cleanup.
- Host logs are readable, local-only, and do not expose the full invite after the initial generated prompt.
- Errors are semantic and stable enough for tests.

## Milestone 4: Temporary `/cli` Distribution

### Purpose

Add the product's frictionless temporary binary UX after the encrypted local command loop works.

### In Scope

- Relay-served POSIX `sh` bootstrapper at `/cli`.
- Relay-origin inference from the served bootstrapper.
- Artifact serving from the relay.
- Private temporary cache path for the binary.
- Cache safety checks for ownership, permissions, regular-file status, and checksum match.
- Same-origin checksum verification for corruption detection.
- Pinned or compatible artifact selection so host and client do not accidentally use incompatible protocol versions.
- Generated prompt using the public command shape.

### Out Of Scope

- Strong supply-chain security claims for `curl | sh`.
- Mandatory external verification tools such as `minisign`.
- Accounts, tokens, login, or dashboard flows.
- Install-to-system flow.
- Package-manager distribution.

### Gate

This milestone passes only when:

- `curl -fsSL <relay>/cli | sh -s -- create` works.
- The generated prompt uses a compatible client artifact.
- The temporary cache is private and checksum-validated.
- No user-facing relay flag is needed for the public host/client UX.
- Same-origin checksums are documented only as corruption detection, not as a supply-chain security boundary.

## Milestone 5: Public V1 Readiness

### Purpose

Package, document, and validate the public v1 release against `plan.md`.

### In Scope

- Self-hosted relay usage.
- Docker or systemd-friendly relay operation guidance.
- Generated agent prompt polish.
- Security and trust-boundary documentation.
- Non-goal documentation.
- Final acceptance criteria mapped back to `plan.md`.

### Out Of Scope

- Dashboard.
- Teams or accounts.
- MCP integration.
- Raw SSH compatibility.
- Multi-client sessions.
- PTY support.
- File upload or download.
- Approval workflows.
- Persistent relay state.

### Gate

This milestone passes only when:

- Full `plan.md` acceptance criteria are satisfied or explicitly deferred.
- Documentation states trust boundaries honestly.
- Self-hosting works.
- The generated agent prompt is clear about bearer-secret invite material and command execution without per-command approval.
- Non-goals are documented.

## Cross-Milestone Rules

- Never build a plaintext tunnel as the main implementation path.
- Do not start `/cli` distribution until encrypted local-binary execution works.
- Treat developer-only flags like `--relay` as milestone tooling, not public UX.
- Do not weaken relay-blind privacy to simplify routing.
- Do not send the full invite code, `clientSecret`, command text, output, exit code, host metadata, or client identity to the relay as plaintext application data.
- If the Noise spike fails, stop and revisit protocol or library choice before broader implementation.
- Keep `/cli` supply-chain messaging honest: same-origin checksums detect corruption, not malicious origin compromise.
- Preserve `plan.md` non-goals unless an explicit re-scope decision is made.

## Explicit Non-Goals For This Milestone Plan

This milestone plan does not add or re-scope:

- accounts or login,
- OAuth,
- dashboard,
- daemon/background mode,
- `opentunnel stop`, `opentunnel list`, or `opentunnel logs`,
- MCP integration,
- raw SSH compatibility,
- approval workflows,
- policy profiles,
- file upload/download,
- background process management,
- named agents,
- first-client pinning,
- single-use continuation tokens,
- persistent relay runtime state,
- relay-side command logs,
- multi-client support,
- concurrent command execution,
- PTY support.

## Gate Summary

| Milestone | Gate Question |
|---|---|
| Secure Channel Spike | Can Go cleanly implement the selected Noise-based secure channel and failure semantics? |
| Local Binary Vertical Slice | Can local binaries execute one remote command through a relay-blind encrypted tunnel? |
| Lifecycle And Safety Hardening | Does the tunnel behave like temporary supervised access with clear limits and cleanup? |
| Temporary `/cli` Distribution | Does the public zero-install UX work without hiding its trust boundary? |
| Public V1 Readiness | Is the release deployable, documented, and aligned with `plan.md`? |
