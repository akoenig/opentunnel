# OpenTunnel Lifecycle And Safety Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Milestone 3 from `docs/superpowers/specs/2026-05-22-opentunnel-v1-milestone-design.md`: make the local encrypted tunnel behave like temporary supervised access with one active client/command, command timeout, output limit, idle timeout, process cleanup, local host logs, and semantic errors.

**Architecture:** Preserve Milestone 2 package boundaries. `tunnel` owns protocol errors, command/session defaults, host logs, command timeout, output limit, and idle timeout. `command` owns local process execution and process-group cleanup where supported. `relay` keeps deterministic one-client admission. `cmd/opentunnel` wires local logs to the host terminal. No `/cli`, cache, artifact serving, approval workflow, PTY, stdin, file transfer, dashboard, accounts, metrics, or persistent audit log is added.

**Tech Stack:** Go, existing `securechannel`, `invite`, `relay`, `command`, `tunnel`, standard `context`, `time`, `errors`, `os/exec`, Unix `syscall` where supported, Go tests and race detector.

---

## File Structure

- Create: `internal/tunnel/errors.go`
  - Semantic error names and encrypted error message helpers.
- Modify: `internal/tunnel/protocol.go`
  - Add error messages and command defaults fields.
- Modify: `internal/tunnel/host.go`, `internal/tunnel/client.go`, `internal/tunnel/tunnel_test.go`
  - Add command timeout, output limit, idle timeout, host logs, and semantic error handling.
- Modify: `internal/command/runner.go`, `internal/command/runner_test.go`
  - Add process-group cleanup hooks and context cancellation behavior.
- Create: `internal/command/process_unix.go`
  - Unix process group configuration and termination helpers.
- Create: `internal/command/process_other.go`
  - Non-Unix no-op fallback behind build tags.
- Modify: `cmd/opentunnel/main.go`, `cmd/opentunnel/main_test.go`
  - Wire host logs and preserve remote exit codes with semantic errors.

## Task 1: Add Semantic Tunnel Errors

**Files:**
- Create: `internal/tunnel/errors.go`
- Modify: `internal/tunnel/protocol.go`
- Modify: `internal/tunnel/tunnel_test.go`

- [ ] **Step 1: Write failing tests**

Add tests proving `ErrorType` constants exist for `HostUnavailableError`, `ClientAlreadyConnectedError`, `HandshakeFailedError`, `CommandAlreadyRunningError`, `CommandTimeoutError`, `MaxOutputExceededError`, `CommandStartFailedError`, `IdleSessionTimeoutError`, `ProtocolError`, and that an encrypted `error` protocol message round-trips through `encryptJSON`/`decryptJSON`.

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/tunnel -run 'TestSemanticErrorTypes|TestEncryptedErrorMessageRoundTrip' -count=1
```

Expected: FAIL because error types/message support are missing.

- [ ] **Step 3: Implement semantic errors**

Create `errors.go` with exported `ErrorType` string constants and add `ErrorType` plus `Message` fields to the encrypted protocol message struct. Keep all errors inside encrypted tunnel messages except transport-level relay admission failures.

- [ ] **Step 4: Verify green**

Run:

```bash
gofmt -w internal/tunnel/*.go
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tunnel/errors.go internal/tunnel/protocol.go internal/tunnel/tunnel_test.go
```

## Task 2: Add Command Timeout And Process Cleanup

**Files:**
- Modify: `internal/command/runner.go`
- Modify: `internal/command/runner_test.go`
- Create: `internal/command/process_unix.go`
- Create: `internal/command/process_other.go`
- Modify: `internal/tunnel/host.go`
- Modify: `internal/tunnel/tunnel_test.go`

- [ ] **Step 1: Write failing tests**

Add command package tests proving a context-cancelled command returns promptly with a context error and that non-zero command exit remains `ExitCode` with nil error. Add tunnel test configuring a short command timeout and running `sleep 2`, expecting an encrypted `CommandTimeoutError` or Exec error classified as timeout.

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/command -run TestRunCancelsCommandOnContextCancel -count=1
```

Expected: FAIL because timeout integration/process cleanup behavior is missing or too slow.

- [ ] **Step 3: Implement command cleanup**

Use `exec.CommandContext`; on Unix set process group with `SysProcAttr.Setpgid = true` and terminate the process group on context cancellation. Keep non-Unix fallback compiling. Do not add user-facing timeout flags.

- [ ] **Step 4: Implement tunnel command timeout**

Add defaults in tunnel: `CommandTimeout = 120 * time.Second`, plus test-only configurable `HostConfig.CommandTimeout`. Host wraps command execution in `context.WithTimeout`; on deadline exceeded send encrypted `CommandTimeoutError` and return an error to the client.

- [ ] **Step 5: Verify green**

Run:

```bash
gofmt -w internal/command/*.go internal/tunnel/*.go
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/command internal/tunnel
```

## Task 3: Add Output Limit And Truncation Error

**Files:**
- Modify: `internal/tunnel/host.go`
- Modify: `internal/tunnel/client.go`
- Modify: `internal/tunnel/tunnel_test.go`

- [ ] **Step 1: Write failing tests**

Add tunnel test using `HostConfig.MaxOutputBytes = 5` and command `printf 123456789`; expect client receives only the allowed output prefix and receives/returns `MaxOutputExceededError` or an Exec error classified as max-output exceeded.

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/tunnel -run TestExecMaxOutputExceeded -count=1
```

Expected: FAIL because output limit is missing.

- [ ] **Step 3: Implement output limit**

Add default `MaxOutputBytes = 10 * 1024 * 1024`, with test-only configurable `HostConfig.MaxOutputBytes`. Track combined stdout/stderr bytes before encrypted output send. Send allowed prefix, then encrypted `MaxOutputExceededError`, and stop further output writes. Do not add public CLI flags.

- [ ] **Step 4: Verify green**

Run:

```bash
gofmt -w internal/tunnel/*.go
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/tunnel
```

## Task 4: Add Idle Session Timeout And Host Logs

**Files:**
- Modify: `internal/tunnel/host.go`
- Modify: `internal/tunnel/tunnel_test.go`
- Modify: `cmd/opentunnel/main.go`

- [ ] **Step 1: Write failing tests**

Add tunnel test with `HostConfig.IdleTimeout = 50 * time.Millisecond`; after StartHost and no commands, `session.Done` should close with `IdleSessionTimeoutError`. Add test with a `bytes.Buffer` log writer proving lines include `opentunnel event=sessionOpen`, `event=waiting`, `event=commandStart`, `event=commandFinish`, and `event=sessionClose`, without printing the full invite after initial prompt.

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/tunnel -run 'TestHostIdleTimeout|TestHostLocalStatusLogs' -count=1
```

Expected: FAIL because idle timeout/logging support is missing.

- [ ] **Step 3: Implement idle timeout**

Add default `IdleTimeout = 30 * time.Minute`, configurable in `HostConfig` for tests. Reset idle timer after each successful command. Pause idle timer while command is running by resetting after command finish. Close host session on idle timeout without relay persistence.

- [ ] **Step 4: Implement local logs**

Add `HostConfig.LogWriter io.Writer`. Emit compact lines prefixed with `opentunnel`, stable `event=<name>`, and camelCase fields for session open, waiting, client connected, command start, command finish, command timeout, output truncated, idle timeout, and session close. Wire CLI `create` to pass stderr as log writer.

- [ ] **Step 5: Verify green**

Run:

```bash
gofmt -w internal/tunnel/*.go cmd/opentunnel/*.go
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/tunnel cmd/opentunnel/main.go
```

## Task 5: Final Milestone 3 Verification

**Files:**
- Verify all files modified by this plan.

- [ ] **Step 1: Run full verification**

Run:

```bash
gofmt -w internal/command/*.go internal/tunnel/*.go cmd/opentunnel/*.go
```

Expected: all pass.

- [ ] **Step 2: Run manual lifecycle checks**

Using a temp binary, verify relay/create/exec still works, sequential exec still works, timeout command fails semantically if exposed through test config only via tests, and interrupting create closes the session.

- [ ] **Step 3: Commit cleanup if needed**

If formatting/tidy/doc changes exist:

```bash
git add internal/command internal/tunnel cmd/opentunnel docs/superpowers/plans/2026-06-09-opentunnel-lifecycle-safety-hardening.md go.mod go.sum
```

Do not create an empty commit.

## Self-Review Checklist

- Milestone 3 gate is covered: second client rejection remains, concurrent command semantics remain one command per client route, command timeout works, output limit works, idle timeout closes forgotten sessions, Ctrl+C/context cancellation closes host, host logs are local and readable, semantic errors are stable.
- Out-of-scope items remain absent: approval mode, first-client pinning, persistent audit logs, command cancellation protocol, background process management, configurable user policy profiles, multiple clients, concurrent command execution.
- Final verification includes normal tests, race tests, vet, module tidy, build, and manual lifecycle checks.
