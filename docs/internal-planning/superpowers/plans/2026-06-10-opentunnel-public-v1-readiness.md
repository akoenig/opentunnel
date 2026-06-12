# OpenTunnel Public V1 Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make OpenTunnel public-v1 ready by polishing the generated agent prompt, adding self-hosting and security documentation, mapping acceptance criteria to `docs/internal-planning/plan.md`, and verifying the public `/cli` create/exec flow.

**Architecture:** Keep product behavior minimal and centered on the existing foreground `create`, one-off `exec`, opaque relay, and `/cli` bootstrapper. Add no new protocol surface; M5 changes should be prompt text, documentation, and verification only unless tests expose a small public-UX gap.

**Tech Stack:** Go standard library, existing `cmd/opentunnel`, existing `internal/relay`, existing `internal/artifact`, Markdown documentation, shell-based manual verification.

---

## File Structure

- Modify `cmd/opentunnel/main.go`: replace the terse `writeCreateReady` output with the public v1 prompt described in `docs/internal-planning/plan.md`.
- Modify `cmd/opentunnel/main_test.go`: add prompt tests for the public template, suggested first command, bearer-secret warning, notes, and absence of user-facing `--relay`.
- Create `docs/public-v1/self-hosting.md`: relay build/run guidance and artifact flag explanation.
- Create `docs/public-v1/security.md`: trust boundaries, relay blindness, invite handling, same-origin checksum limitation.
- Create `docs/public-v1/non-goals.md`: explicit v1 exclusions.
- Create `docs/public-v1/acceptance.md`: `docs/internal-planning/plan.md` acceptance mapping.
- Commit after each task.

## Task 1: Polish Generated Agent Prompt

**Files:**
- Modify: `cmd/opentunnel/main.go`
- Modify: `cmd/opentunnel/main_test.go`

- [ ] **Step 1: Add failing prompt coverage**

Replace `TestWriteCreateReadyPrintsBootstrapPromptWithoutStandaloneSecrets` in `cmd/opentunnel/main_test.go` with this stricter test:

```go
func TestWriteCreateReadyPrintsPublicAgentPrompt(t *testing.T) {
	var stdout bytes.Buffer
	invite := "ot1_example_secret"

	writeCreateReady(&stdout, invite, "http://localhost:8080")

	output := stdout.String()
	wants := []string{
		"I opened an OpenTunnel session for you.",
		"Run commands on my host with:",
		"curl -fsSL http://localhost:8080/cli | sh -s -- exec \\",
		"  --invite '" + invite + "' \\",
		"  -- '<COMMAND>'",
		"Start with:",
		"  -- 'hostname && uname -a && pwd'",
		"Commands execute without per-command approval while this foreground session is running.",
		"Treat the invite as bearer-secret material.",
		"The host owner can revoke access with Ctrl+C.",
		"Use non-interactive commands.",
		"No PTY or interactive stdin is available in the first major version.",
		"Avoid sudo unless it is passwordless and non-interactive.",
		"Only one client can connect to this tunnel at a time.",
		"Only one command runs at a time.",
		"The temporary OpenTunnel CLI is cached in the system temp directory during the session.",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("create output missing %q in:\n%s", want, output)
		}
	}
	if strings.Count(output, invite) != 2 {
		t.Fatalf("create output prints invite %d times, want twice in command examples:\n%s", strings.Count(output, invite), output)
	}
	if strings.Contains(output, " --relay ") || strings.Contains(output, "\nrelay:") || strings.Contains(output, "\nsecret:") {
		t.Fatalf("create output contains user-facing relay flag or standalone secret label in:\n%s", output)
	}
}
```

- [ ] **Step 2: Run the prompt test and verify RED**

Run:

```bash
go test ./cmd/opentunnel -run TestWriteCreateReadyPrintsPublicAgentPrompt -count=1
```

Expected: FAIL because the current prompt only prints `agent-ready` and one `run:` line.

- [ ] **Step 3: Implement the public prompt**

Replace `writeCreateReady` in `cmd/opentunnel/main.go` with:

```go
func writeCreateReady(stdout io.Writer, invite string, relayURL string) {
	origin := strings.TrimRight(relayURL, "/")
	fmt.Fprintf(stdout, `I opened an OpenTunnel session for you.

Run commands on my host with:

curl -fsSL %[1]s/cli | sh -s -- exec \
  --invite '%[2]s' \
  -- '<COMMAND>'

Start with:

curl -fsSL %[1]s/cli | sh -s -- exec \
  --invite '%[2]s' \
  -- 'hostname && uname -a && pwd'

Commands execute without per-command approval while this foreground session is running.
Treat the invite as bearer-secret material. Do not copy it into shared logs, tickets, summaries, or long-lived notes. The host owner can revoke access with Ctrl+C.

Notes:
- Use non-interactive commands.
- No PTY or interactive stdin is available in the first major version.
- Avoid sudo unless it is passwordless and non-interactive.
- Avoid long-running commands unless necessary.
- Only one client can connect to this tunnel at a time.
- Only one command runs at a time.
- The temporary OpenTunnel CLI is cached in the system temp directory during the session.
`, origin, invite)
}
```

- [ ] **Step 4: Run focused and full CLI tests**

Run:

```bash
gofmt -w cmd/opentunnel/*.go
go test ./cmd/opentunnel -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit prompt polish**

Run:

```bash
git add cmd/opentunnel/main.go cmd/opentunnel/main_test.go
git commit -m "feat: polish public agent prompt"
```

## Task 2: Add Public V1 Self-Hosting Documentation

**Files:**
- Create: `docs/public-v1/self-hosting.md`

- [ ] **Step 1: Create self-hosting doc**

Create `docs/public-v1/self-hosting.md` with:

```markdown
# OpenTunnel Public V1 Self-Hosting

OpenTunnel v1 runs as a self-hosted relay plus temporary clients downloaded from that relay's `/cli` endpoint. The relay stores no sessions, commands, outputs, invites, or audit logs. Active tunnel state exists only in memory.

## Build

From the repository root:

```bash
go build -o opentunnel ./cmd/opentunnel
```

## Run A Local Relay

For local testing:

```bash
./opentunnel relay \
  --listen 127.0.0.1:8080 \
  --public-url http://127.0.0.1:8080 \
  --artifact-path ./opentunnel \
  --version dev
```

For a public relay, set `--public-url` to the HTTPS origin users will fetch:

```bash
./opentunnel relay \
  --listen :8080 \
  --public-url https://relay.example.com \
  --artifact-path /opt/opentunnel/opentunnel \
  --version v1
```

Terminate TLS in front of the relay with your normal reverse proxy or load balancer. The relay process expects the public origin to be HTTP or HTTPS and does not require a database or Redis.

## Public Host Command

Users start a foreground host session with:

```bash
curl -fsSL https://relay.example.com/cli | sh -s -- create
```

The session stays open until Ctrl+C, idle timeout, relay failure, or process exit.

## Public Client Command

The host prints an agent prompt containing commands like:

```bash
curl -fsSL https://relay.example.com/cli | sh -s -- exec \
  --invite '<invite>' \
  -- '<COMMAND>'
```

The invite contains the information the client needs to connect. The user-facing command does not include a relay flag.

## Artifact Flags

- `--artifact-path` points to the binary served by `/cli/bin/opentunnel-<version>-<platform>`.
- `--version` becomes part of the artifact URL and cache key.
- `--public-url` is the relay origin embedded into the bootstrapper.

The relay serves only the configured artifact path and its checksum. It does not expose arbitrary files.
```

- [ ] **Step 2: Verify Markdown file exists**

Run:

```bash
test -s docs/public-v1/self-hosting.md
```

Expected: exit code 0.

- [ ] **Step 3: Commit self-hosting doc**

Run:

```bash
git add docs/public-v1/self-hosting.md
git commit -m "docs: add public self-hosting guide"
```

## Task 3: Add Security And Trust Boundary Documentation

**Files:**
- Create: `docs/public-v1/security.md`

- [ ] **Step 1: Create security doc**

Create `docs/public-v1/security.md` with:

```markdown
# OpenTunnel Public V1 Security Model

OpenTunnel v1 is an ephemeral, relay-routed, end-to-end encrypted remote command tunnel. The relay coordinates active connections but does not decrypt command traffic.

## Trust Boundaries

The host and client establish an end-to-end encrypted secure channel using invite material generated by the host. The relay sees routing metadata and encrypted frames. It should not see commands, stdout, stderr, remote exit codes, host metadata, or the invite contents.

## Relay State

The relay keeps only in-memory active connection state while sessions are connected. It does not persist sessions, invites, payloads, command logs, audit logs, client metadata, or outputs.

## Invite Handling

The invite is bearer-secret material. Anyone with a valid invite can attempt to connect while the host session is active. Do not copy invites into shared logs, tickets, summaries, or long-lived notes. The host owner revokes access by pressing Ctrl+C or letting the foreground process exit.

## `/cli` And Checksums

The `/cli` bootstrapper downloads the matching OpenTunnel binary and verifies the same-origin checksum before execution. This detects corruption or mismatched artifacts from the trusted relay origin.

Same-origin checksums are not a strong supply-chain security boundary. If the relay origin or the transport serving `/cli` is compromised, an attacker can change both the bootstrapper and checksum. Use HTTPS and operate the relay origin as trusted infrastructure.

## Execution Semantics

Commands execute without per-command approval while the foreground host session is running. OpenTunnel v1 intentionally keeps the model simple: one active client and one active command at a time.

## Host-Side Logs

Host logs are local status messages. They should help the host owner understand connection, command, timeout, truncation, and close events. They are not sent to the relay as plaintext.
```

- [ ] **Step 2: Scan for forbidden supply-chain claim wording**

Run:

```bash
grep -n "supply-chain secure\|guarantee\|tamper-proof" docs/public-v1/security.md
```

Expected: no output and non-zero exit code is acceptable.

- [ ] **Step 3: Commit security doc**

Run:

```bash
git add docs/public-v1/security.md
git commit -m "docs: add public security model"
```

## Task 4: Add V1 Non-Goals Documentation

**Files:**
- Create: `docs/public-v1/non-goals.md`

- [ ] **Step 1: Create non-goals doc**

Create `docs/public-v1/non-goals.md` with:

```markdown
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
```

- [ ] **Step 2: Verify non-goals doc exists**

Run:

```bash
test -s docs/public-v1/non-goals.md
```

Expected: exit code 0.

- [ ] **Step 3: Commit non-goals doc**

Run:

```bash
git add docs/public-v1/non-goals.md
git commit -m "docs: document public v1 non-goals"
```

## Task 5: Add Acceptance Mapping Documentation

**Files:**
- Create: `docs/public-v1/acceptance.md`

- [ ] **Step 1: Create acceptance mapping**

Create `docs/public-v1/acceptance.md` with:

```markdown
# OpenTunnel Public V1 Acceptance Mapping

This document maps `docs/internal-planning/plan.md` public v1 requirements to the current implementation state.

## Public UX

| Requirement | Status |
| --- | --- |
| Host starts with `curl -fsSL <relay>/cli \| sh -s -- create` | Implemented and manually verified. |
| Client executes with `curl -fsSL <relay>/cli \| sh -s -- exec --invite '<invite>' -- '<command>'` | Implemented and manually verified. |
| No user-facing relay flag in public UX | Implemented. Relay origin comes from `/cli` bootstrap context or invite. |
| Temporary CLI cached during session | Implemented. Cache lives under a private temp cache path and cache hits are checksum-verified. |

## Relay Privacy

| Requirement | Status |
| --- | --- |
| Relay persists no sessions or command data | Implemented. Relay state is in memory only. |
| Relay routes opaque encrypted packets | Implemented. Relay forwards binary frames and does not decrypt command traffic. |
| Relay cannot read command, output, exit code, host metadata, or `clientSecret` | Implemented by the secure channel and tunnel protocol design. |

## Session Model

| Requirement | Status |
| --- | --- |
| One active client per tunnel | Implemented. |
| One active command at a time | Implemented. |
| Foreground `create` process owns lifetime | Implemented. |
| Ctrl+C closes the tunnel | Implemented. |
| Idle timeout closes forgotten sessions | Implemented. |
| Command timeout and process cleanup | Implemented. |
| Output limit and truncation | Implemented. |

## Documentation And Non-Goals

| Requirement | Status |
| --- | --- |
| Self-hosting guidance | Documented in `docs/public-v1/self-hosting.md`. |
| Trust-boundary documentation | Documented in `docs/public-v1/security.md`. |
| Non-goals documented | Documented in `docs/public-v1/non-goals.md`. |
| Same-origin checksum described only as corruption detection | Documented in `docs/public-v1/security.md`. |

## Explicit V1 Exclusions

Accounts, dashboards, package-manager distribution, install-to-system flows, MCP, raw SSH, PTY, interactive stdin, file transfer, approval workflows, multiple clients, concurrent commands, persistent relay state, and persistent audit logs are excluded from v1.
```

- [ ] **Step 2: Verify acceptance doc references all public docs**

Run:

```bash
grep -n "self-hosting.md\|security.md\|non-goals.md" docs/public-v1/acceptance.md
```

Expected: output includes all three referenced docs.

- [ ] **Step 3: Commit acceptance mapping**

Run:

```bash
git add docs/public-v1/acceptance.md
git commit -m "docs: map public v1 acceptance criteria"
```

## Task 6: Final Public V1 Verification

**Files:**
- No committed source changes expected.

- [ ] **Step 1: Run full Go verification**

Run:

```bash
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
rm -f ./opentunnel
```

Expected: all commands pass and no root `opentunnel` binary remains.

- [ ] **Step 2: Run public `/cli` create and generated exec manual flow**

Run this from the repository root:

```bash
bash -lc 'set -euo pipefail; tmp=$(mktemp -d "/tmp/opencode/opentunnel-m5.XXXXXX"); bin="$tmp/opentunnel"; relay_log="$tmp/relay.log"; create_out="$tmp/create.out"; create_err="$tmp/create.err"; exec_out="$tmp/exec.out"; exec_err="$tmp/exec.err"; go build -o "$bin" ./cmd/opentunnel; "$bin" relay --listen 127.0.0.1:18082 --public-url http://127.0.0.1:18082 --artifact-path "$bin" --version m5 >"$relay_log" 2>&1 & relay_pid=$!; cleanup() { kill "$relay_pid" 2>/dev/null || true; kill "$create_pid" 2>/dev/null || true; wait "$relay_pid" 2>/dev/null || true; wait "$create_pid" 2>/dev/null || true; rm -rf "$tmp"; }; trap cleanup EXIT; for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do if curl -fsS "http://127.0.0.1:18082/cli" >/dev/null; then break; fi; sleep 0.25; if [ "$i" -eq 20 ]; then printf "relay did not become ready\n" >&2; exit 1; fi; done; TMPDIR="$tmp" curl -fsSL "http://127.0.0.1:18082/cli" | TMPDIR="$tmp" sh -s -- create >"$create_out" 2>"$create_err" & create_pid=$!; for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do if grep -q "Run commands on my host with:" "$create_out"; then break; fi; sleep 0.25; if [ "$i" -eq 20 ]; then printf "create did not print public prompt\n" >&2; exit 1; fi; done; invite=$(awk "/--invite/{gsub(/^'\''|'\''$/, \"\", \$2); print \$2; exit}" "$create_out"); if [ -z "$invite" ]; then printf "invite not found in create output\n" >&2; exit 1; fi; TMPDIR="$tmp" curl -fsSL "http://127.0.0.1:18082/cli" | TMPDIR="$tmp" sh -s -- exec --invite "$invite" -- printf m5-ok >"$exec_out" 2>"$exec_err"; result=$(tr -d "\r\n" <"$exec_out"); if [ "$result" != "m5-ok" ]; then printf "unexpected exec output: %s\n" "$result" >&2; cat "$exec_err" >&2; exit 1; fi; printf "manual public /cli flow passed: %s\n" "$result"'
```

Expected: output includes `manual public /cli flow passed: m5-ok`.

- [ ] **Step 3: Check final git status**

Run:

```bash
git status --short
```

Expected: only unrelated untracked `.agents/` and `skills-lock.json`, unless the user has changed additional files concurrently.

- [ ] **Step 4: Do not create an empty verification commit**

If verification required no file changes, leave the commit history unchanged. If verification changed files unexpectedly, stop and inspect the diff before proceeding.
