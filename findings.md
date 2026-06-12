# OpenTunnel — Findings, Recommendations & Implementation Notes

> **Purpose of this document.** This is a hand-off brief for an implementing agent. It records the
> results of a structure / maintainability / security review of the OpenTunnel codebase, the agreed
> fixes (the repo owner has approved incorporating **all** findings), and concrete implementation
> guidance — including refined approaches for findings S2, S5, and S6 based on owner feedback.
>
> Nothing in this document has been implemented yet. The codebase currently passes
> `go test ./...` and `go vet ./...` cleanly. Re-run the full verification suite (below) after each
> change group.

## Repository at a glance

- Go module `opentunnel`, ~5k lines, `go 1.23` declared in `go.mod`.
- Entry point: `cmd/opentunnel/main.go` (subcommands `relay`, `create`, `exec`).
- Packages under `internal/`: `relay`, `tunnel`, `securechannel`, `command`, `invite`, `artifact`, `buildinfo`.
- Crypto: Noise **NKpsk0** (`Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s`) with a canonical length-prefixed
  prologue, per-session X25519 host keypair, 256-bit client PSK carried in an opaque invite.
- Relay routes opaque WebSocket binary frames between exactly one host and one client per session;
  state is in-memory only.
- Deployment: distroless `nonroot` Docker image, hardened systemd unit. CI runs test/vet/tidy/race/build.

The design is sound and matches the documented threat model (relay sees only routing/timing/size
metadata; command traffic is end-to-end encrypted). The findings below are about hardening the
edges, not reworking the architecture.

## Verification commands (run after every change group)

```bash
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
rm -f ./opentunnel
```

Also add to local/CI flow once available:

```bash
govulncheck ./...
golangci-lint run
```

---

## Security findings

### S1 — One bad client connection terminates the whole host session — **High (availability)**

**Where:** `internal/tunnel/host.go:108-139` (`runHost`), `handleOneHostConnection` /
`handleOneCommand`.

**Problem:** In `runHost`, any error returned from `handleOneHostConnection` is treated as fatal and
written to `done`, ending the session. An attacker who knows only the **session ID** (not the PSK)
can connect to `/tunnel?role=client&session=…` and either (a) send one garbage frame so the host's
handshake read fails, or (b) just disconnect — the relay's `disconnect` closes the host's socket too,
producing a "read client handshake" error. Either way the legitimate tunnel dies and the host owner
must re-create it. Because the PSK is 256-bit random, tolerating failed handshakes does **not** open a
brute-force path.

**Fix:** Make handshake failures and pre-handshake client disconnects **non-fatal** in the host loop.
Log the event, re-dial the relay, and keep waiting (subject to the existing idle timeout and Ctrl+C).
Reserve fatal `done <- err` for genuine local errors (e.g. cannot re-dial relay for non-conflict
reasons, keypair failure). The reconnect/`StatusConflict` retry logic in `dialHostRelay`
(`host.go:155-174`) already shows the intended resilient pattern; extend that resilience to the
handshake/command phase.

**Tests:** Add a test where a rogue client connects and sends junk (or disconnects immediately) and
assert the host session stays alive and a subsequent legitimate `Exec` still succeeds.

---

### S2 — Session ID travels in the URL query → leaks into access logs — **High (design / footgun)**

**Where:** `internal/tunnel/host.go:345-353` (`tunnelEndpoint`), `internal/relay/server.go:133-138`
(reads `r.URL.Query().Get("role"/"session")`).

**Problem:** `role` and `session` are passed as URL query parameters. Standard HTTP access logs
(reverse proxies, load balancers, the relay itself) record the request line, so the session ID lands
in logs by default. Combined with S1, anyone who reads a log line can kill or occupy a live tunnel.

**Owner decision:** *Remove the footgun at the source — don't just document it.*

**Fix (approved approach):** Move `role` and `session` out of the URL query and into **request
headers** (e.g. `OpenTunnel-Role` and `OpenTunnel-Session`). Standard access logs record the request
line and a configurable header subset, but not arbitrary custom request headers by default, so the
session ID stops appearing in logs without operator action. The WebSocket path becomes a static
`/tunnel` with no query string.

**Implementation notes:**
- Client/host side: set the headers via the `http.Header` argument to
  `websocket.DefaultDialer.Dial(endpoint, header)` (currently `nil`). `tunnelEndpoint` should return a
  bare `/tunnel` URL plus a populated `http.Header`.
- Relay side: read `r.Header.Get("OpenTunnel-Role")` / `…-Session` instead of `r.URL.Query()`.
- Keep the same validation (`role ∈ {host, client}`, `session != ""`).
- **Update tests:** `internal/relay/server_test.go` builds URLs with
  `?role=…&session=…` in `tunnelURL` (line 451) and reads `r.URL.Query().Get("role")` inside the
  `CheckOrigin` hook (line 225); `internal/tunnel/tunnel_test.go:668` and `host.go` callers go through
  `tunnelEndpoint`. All of these must switch to headers.
- Document in `docs/public-v1/security.md` that the session ID is a sensitive routing token carried in
  a custom header and must not be added to logged-header allowlists.

**Residual risk to note in docs:** the session ID is still visible to the relay process itself (by
design — the relay must route on it) and to any TLS-terminating proxy that is explicitly configured to
log custom headers. The header approach removes the *default* leakage, which is the footgun.

---

### S3 — Unbounded relay memory; no server/read timeouts or read limits — **High (availability)**

**Where:** `internal/relay/server.go:17` (`sessions` map), `:224-247` (`reserve`),
`cmd/opentunnel/main.go:213` (`&http.Server{...}` with no timeouts), upgrader has no `ReadLimit`.

**Problem:** `sessions` grows one entry per distinct `role=host` connection with no global cap, no
per-IP limit, and no TTL on a reserved-but-unconnected session. The `http.Server` sets no
`ReadHeaderTimeout`/`IdleTimeout`, and upgraded sockets set no `conn.SetReadLimit(...)`. A
mass-connect or slow-loris attacker can pin memory and goroutines.

**Fix:**
- Set timeouts on the `http.Server`: `ReadHeaderTimeout` (e.g. 5–10s) and `IdleTimeout`.
- Cap total active sessions (configurable; return `503`/`429` when exceeded) and consider a per-remote
  reservation cap.
- Add a reservation TTL: a host slot reserved but never followed by a connected client should expire
  and be reaped.
- Call `conn.SetReadLimit(maxFrameBytes)` after upgrade (ties into S4).

**Tests:** session-cap rejection; reservation-expiry reaping; oversized header / slow client rejected
by timeout.

---

### S4 — Unauthenticated frame relaying with no size/rate limits — **Medium (availability)**

**Where:** `internal/relay/server.go:264-284` (`forward`).

**Problem:** The relay copies binary frames between peers with no per-frame size cap and no rate
limiting. The host bounds *command output* (`maxOutputBytes`, 10 MB) but the relay will shuttle
arbitrarily large frames in either direction, and a client can stream unbounded data toward the host
before the handshake meaningfully gates anything.

**Fix:** Enforce a maximum frame size at the relay via `conn.SetReadLimit(...)` (shared with S3) and a
sane upper bound on forwarded frames. Optionally add basic per-connection rate limiting. Choose a max
frame size comfortably above the largest legitimate output chunk (host reads in 4096-byte chunks; the
JSON+Noise framing adds overhead) but well below "unbounded".

**Tests:** frame over the limit is rejected/closed rather than forwarded.

---

### S5 — Invite (contains the PSK) passed as a CLI argument → visible in process listing / history — **Medium**

**Where:** `cmd/opentunnel/main.go:116-140` (`parseExecArgs`, `--invite`), the printed prompt in
`writeCreateReady` (`main.go:264-291`), README.

**Problem:** `exec --invite '<invite>'` puts bearer-secret material into `argv`, visible via
`/proc/<pid>/cmdline` to other local users and prone to leaking into shell history and `ps` output.

**Owner decision:** *Avoid leaking into process listing / shell history.*

**Fix (approved approach):** Accept the invite from an **environment variable** (e.g.
`OPENTUNNEL_INVITE`) in addition to `--invite`, and make the **prompt printed by `create` use the env
var form by default** so the copy-pasted command never carries the secret in `argv`.

**Implementation notes:**
- In `parseExecArgs`, if `--invite` is empty, fall back to `os.Getenv("OPENTUNNEL_INVITE")` (mirrors
  the existing `OPENTUNNEL_RELAY_ORIGIN` fallback for `create` at `main.go:104-106`). Keep `--invite`
  working for backward compatibility but prefer the env var in docs.
- Update `writeCreateReady` so the generated prompt passes the invite via the environment, e.g.:

  ```sh
  curl -fsSL https://relay.example.com/cli | OPENTUNNEL_INVITE='<invite>' sh -s -- exec -- '<COMMAND>'
  ```

  This keeps the secret out of the `exec` process's `argv`. (Note: the assignment is still on the
  command line the user types, so it can still reach shell history — call this out, and recommend a
  leading space / `HISTCONTROL=ignorespace` or reading from a file for shared machines. The key win is
  that the long-lived `opentunnel exec` **process** no longer exposes the invite in `/proc/.../cmdline`
  or `ps`.)
- Consider also supporting `--invite-stdin` (read invite from stdin) as the most hardened option for
  shared hosts; document it.
- **Update tests:** `cmd/opentunnel/main_test.go` — `TestParseArgsExec` and the
  `TestWriteCreateReadyPrintsPublicAgentPrompt` assertions (lines 153-189) pin the exact prompt text
  and that the invite appears exactly twice; these expectations change with the new prompt shape. Add
  a test for the `OPENTUNNEL_INVITE` fallback (parallel to
  `TestParseArgsCreateUsesRelayOriginFromEnvironment`).
- Update README "Public Command Shape" and `docs/public-v1/` examples to the env-var form.

---

### S6 — `CheckOrigin` always returns true — **Owner wants this tightened to non-browser clients**

**Where:** `internal/relay/server.go:113-117` (`upgrader.CheckOrigin` returns `true`).

**Problem:** The relay accepts WebSocket upgrades from any origin, including browsers on arbitrary
websites. With the protocol being binary and PSK-gated this is not a direct compromise, but it widens
the DoS surface (S2/S3) to drive-by browser traffic.

**Owner decision:** *Strongly in favor of minimizing to non-browser clients.*

**Fix (approved approach):** Reject upgrades that carry an `Origin` header. Browsers always send an
`Origin` on WebSocket handshakes; the legitimate Go client/host (`gorilla/websocket` dialer) does not
set one. So `CheckOrigin` should return `false` when `r.Header.Get("Origin") != ""` and `true`
otherwise. This restricts the relay to non-browser clients with a single, simple check.

**Implementation notes:**
- Replace the always-`true` closure with one that rejects a non-empty `Origin`.
- **Caution / test interaction:** `internal/relay/server_test.go:216-274`
  (`TestClientSlotReservedBeforeWebSocketUpgrade`) overrides `server.upgrader.CheckOrigin` for its own
  synchronization; that override is fine. But the Go test dialer
  (`websocket.DefaultDialer.Dial`) does not send `Origin`, so existing happy-path tests should still
  pass. Add an explicit test that a request **with** an `Origin` header is rejected (non-101 response)
  and one confirming a request **without** `Origin` still upgrades.
- Document the decision in `docs/public-v1/security.md` (relay is intentionally non-browser-accessible).

---

### S7 — `/cli` bootstrap trusts same-origin checksum — **Low (already documented; keep as-is)**

**Where:** `internal/artifact/bootstrap.go`.

**Status:** The bootstrap downloads the binary and its `.sha256` from the same origin, so the checksum
detects corruption, not a compromised relay. This is **correctly and explicitly** documented in
`README.md` and `docs/public-v1/security.md`. No code change required. The script is well written
(`set -eu`, `umask 077`, `chmod 700`, `mktemp` + `trap` cleanup, per-UID cache).

**Minor robustness nit (optional):** `cache_dir` references `$expected_checksum`
(`bootstrap.go:78`), which is only assigned inside the per-platform `case` block rendered at
`:57`. Correct as written, but a one-line comment noting the ordering dependency would reduce the risk
of a future edit breaking it.

---

## Maintainability & structure findings

### M1 — Dead/duplicate handshake implementation in `securechannel` — **Medium**

**Where:** `internal/securechannel/channel.go:46-75` (`EstablishChannelWithHostKey`), `:154-225`
(`establishNKpsk0`, `establishNKpsk0WithConfigs`).

**Problem:** These three functions implement a second, all-in-one handshake path used **only by
tests**. Production code uses the split `NewClientHandshake` / `NewHostHandshake` flow. Maintaining a
divergent second implementation of the most security-sensitive logic is a real risk.

**Fix:** Delete the unused production functions and rewrite the affected tests against the real split
flow (the `relay`/`tunnel` tests already exercise the split path). If a one-shot test helper is
genuinely wanted, define it in `_test.go` and have it drive `NewClientHandshake`/`NewHostHandshake` so
there is a single production code path.

**Affected tests (must be rewritten, not just deleted — keep the coverage):**
- `internal/securechannel/channel_test.go` — `TestNKpsk0HandshakeEncryptsMultipleFrames`,
  `TestHandshakeFailsWithWrongClientSecret`, `TestHandshakeFailsWithWrongHostPublicKey`,
  `TestHandshakeFailsWithWrongPrologue`, `TestDecryptRejectsReplayedCiphertext`,
  `TestDecryptRejectsMalformedCiphertext` all call the to-be-removed functions. The
  wrong-host-key and wrong-prologue/wrong-secret negative cases are valuable — re-express them against
  the split handshake (drive `NewClientHandshake` with a mismatched `expectedHostPublic`, or build
  client vs host configs with diverging prologue inputs, and assert the handshake fails / returns
  `ErrHostKeyMismatch`).
- `internal/tunnel/tunnel_test.go:721-741` — `testChannels` helper uses
  `securechannel.EstablishChannelWithHostKey`. Reimplement the helper using the split handshake (the
  `connectTestClient` helper in the same file already shows the pattern).

### M2 — Remove unused `PatternXXpsk3` constant and its availability test — **Low**

**Where:** `internal/securechannel/types.go:8-9` (`PatternXXpsk3`),
`internal/securechannel/channel_test.go:228-245` (`TestXXpsk3PatternIsAvailableForFallbackEvaluation`).

**Problem:** Leftover from the spike phase; the XX pattern is not used by the product (NKpsk0 is the
selected pattern). The constant is dead and the test asserts library capability rather than product
behavior.

**Fix:** Remove the constant and the test.

### M3 — `internal/tunnel/host.go` does too much — **Medium (refactor before next feature)**

**Where:** `internal/tunnel/host.go` (449 lines).

**Problem:** `runHost` / `handleOneHostConnection` / `handleOneCommand` interleave relay dialing +
retry, idle-timer lifecycle (`stopTimer`/`resetTimer` + a goroutine), handshake, command execution,
output truncation, and error-to-protocol-message mapping. The timer juggling and nested cancel
goroutines are easy to break in a later edit.

**Fix:** Extract a small `hostSession` type that owns the connection + idle timer, so
`handleOneCommand` becomes a pure request/response unit. This is the one file to refactor before
adding features. Pairs naturally with the S1 fix (the loop's error handling is being touched anyway).

### M4 — Shared URL/origin validation is duplicated — **Low**

**Where:** `cmd/opentunnel/main.go:155-200` (`validateRelayOrigin`, `isShellSafeURLHost`) and
`internal/artifact/bootstrap.go:197-217` (`validateRelayOrigin`).

**Problem:** Two copies of security-relevant origin validation that can drift. Also,
`validatePublicURL` (`main.go:151-153`) is a one-line passthrough to `validateRelayOrigin`.

**Fix:** Extract a single shared origin validator (e.g. a small `internal/originurl` package or a
shared helper) and have both call sites use it. Inline `validatePublicURL`.

### M5 — Non-unix process cleanup silently degrades — **Low**

**Where:** `internal/command/process_other.go` (no-op `configureCommandCleanup` for `!unix`).

**Problem:** On non-unix builds, process-group kill is unavailable, so timed-out/cancelled commands may
leave child process trees running. Releases target linux/darwin only, so this never ships — but it
could be built by accident.

**Fix:** Consider making the non-unix path a build-time failure (build constraint that fails to
compile, or a documented unsupported target) rather than a degraded runtime, so a misconfigured build
can't silently lose process-group cleanup.

### M6 — Protocol `message` is one struct with all-optional fields — **Low (fine at current scope)**

**Where:** `internal/tunnel/protocol.go:17-25`.

**Problem:** A single struct with `omitempty` everywhere serves request/output/exit/error. Nothing
enforces that an `output` has a `Stream` or a `commandRequest` has a `Command`; validation is scattered
across handlers (e.g. `host.go:222`). Acceptable now; if the protocol grows, per-type structs or an
explicit validation step would prevent malformed-message bugs.

**Fix:** No action required for v1. Revisit if the message set expands.

---

## Dependency, toolchain & process findings

### D1 — Go 1.23 is end-of-support; `x/crypto` / `x/sys` pinned to 2020–2021 snapshots — **High**

**Where:** `go.mod`.

**Current state:**
- `go 1.23` declared. (Toolchain available in this environment is **go1.26.0**, so an upgrade is
  unblocked.)
- `golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2` (2021) and
  `golang.org/x/sys v0.0.0-20201119102817-f84b799fce68` (2020), both pulled in indirectly via
  `flynn/noise`. Running 4–5-year-old crypto support libraries is the single highest-signal red flag in
  the dependency set for a security product.
- `flynn/noise v1.1.0` and `gorilla/websocket v1.5.3` are current/reasonable — keep.

**Fix:**
- Bump the `go` directive to a supported release (1.25+; 1.26 is available here) and update CI's
  `go-version` (`.github/workflows/ci.yml`, `.github/workflows/release.yml`) and the Docker builder
  base image (`deploy/docker/Dockerfile` uses `golang:1.23`).
- `go get -u golang.org/x/crypto golang.org/x/sys && go mod tidy`, then run the full verification suite
  (the module proxy is reachable from this environment; latest `x/crypto` observed is ≥ v0.36.0).
- Verify nothing in `flynn/noise` requires the old pins (it should accept current `x/crypto`).

### D2 — No `govulncheck`, no linter in CI — **Medium**

**Where:** `.github/workflows/ci.yml`.

**Problem:** CI runs test/vet/tidy/race/build but has no `govulncheck` and no `golangci-lint`, despite
the repo's own `.agents/skills/golang-pro` skill listing both as required steps and `golang-patterns`
shipping a recommended `.golangci.yml`. For a security product, `govulncheck ./...` on every PR is the
highest-leverage addition (it could not be run during review — not installed — so wire it into CI).

**Fix:**
- Add a CI step running `govulncheck ./...`.
- Add `golangci-lint run` with the `.golangci.yml` from the `golang-patterns` skill (errcheck,
  gosimple, govet, ineffassign, staticcheck, unused, gofmt, goimports, misspell, unconvert, unparam).
- Fix anything they surface.

### D3 — Planning/spike artifacts in the shipping tree — **Low**

**Where:** `plan.md` (71 KB) at repo root; `docs/superpowers/{plans,specs,spikes}`.

**Problem:** Planning/spike material lives alongside shipping code, diluting "what is the product" for
new contributors and inflating the triage surface.

**Fix:** Move under a clearly non-shipping directory or out of the public repo. No code impact.

---

## What's already good (preserve these properties)

- **Crypto:** per-session X25519 host keypair; 256-bit PSK; NKpsk0; prologue binds
  app/version/session/relay-origin/permission-mode/feature-set so a frame can't be replayed into a
  different context; `subtle.ConstantTimeCompare` for the host-key check; invite rejects an all-zero
  secret.
- **Boundaries:** clean package separation; `securechannel/doc.go` explicitly forbids importing relay /
  CLI / command-runner / artifact code.
- **Resource limits (host side):** command output is size-capped with in-band truncation signalling;
  command timeout and idle timeout both enforced; process-group kill (SIGTERM → grace → SIGKILL) on
  unix.
- **Deployment hardening:** distroless `nonroot` image; systemd unit with `NoNewPrivileges`,
  `PrivateTmp`, `ProtectSystem=full`, `ProtectHome`; release workflow refuses `dev`/prerelease/tag
  mismatch publishes.
- **Tests:** every package has tests including relay routing and full encrypted tunnel round-trips; CI
  runs `-race`.

---

## Suggested implementation order

Group the work so each group ends green on the verification suite.

1. **D1** — bump Go + refresh `x/crypto`/`x/sys` (and CI/Dockerfile Go versions). Fast, high-impact,
   self-contained.
2. **M1 + M2** — delete the duplicate handshake path and the dead `PatternXXpsk3`; rewrite affected
   tests against the split handshake. Self-contained, reduces the security-sensitive surface.
3. **S1 + S3 + S4** — host-loop resilience to bad client connections, relay session caps + server
   timeouts + read limits + max frame size. These together close the practical DoS surface; they touch
   overlapping code (`host.go` loop, `relay/server.go`, `main.go` server config).
4. **S2** — move `role`/`session` to request headers; update relay, dialers, and all affected tests;
   update security docs.
5. **S6** — reject upgrades carrying an `Origin` header; add tests; document non-browser stance.
6. **S5** — `OPENTUNNEL_INVITE` env-var support + `--invite-stdin`; change the `create` prompt to the
   env-var form; update prompt tests, README, and docs.
7. **M3** — refactor `host.go` into a `hostSession` type (do alongside or right after S1, since the
   loop is already being edited).
8. **M4** — extract shared origin validation; inline `validatePublicURL`.
9. **D2** — add `govulncheck` + `golangci-lint` to CI; fix what they surface.
10. **D3** — relocate planning artifacts. **M5/M6** — optional, low priority.

## Cross-cutting test-impact summary

These existing tests encode assumptions that several fixes will change — update them deliberately:

- `internal/relay/server_test.go`: `tunnelURL` query-string builder (`:451`) and the
  `r.URL.Query().Get("role")` read in `CheckOrigin` (`:225`) → **S2** (headers). Add **S3/S4/S6**
  tests (session cap, reservation expiry, oversized frame, `Origin`-present rejection).
- `internal/tunnel/tunnel_test.go`: `tunnelEndpoint`-based dialing and `testChannels` helper (`:721`)
  → **S2** (headers) and **M1** (split handshake). Add an **S1** rogue-client-survival test.
- `internal/securechannel/channel_test.go`: six tests call the removed functions, plus the XX test →
  **M1 / M2** (rewrite against split handshake; delete XX test).
- `cmd/opentunnel/main_test.go`: `TestWriteCreateReadyPrintsPublicAgentPrompt` (exact prompt text,
  invite-appears-twice) and `TestParseArgsExec` → **S5** (env-var prompt + fallback). Add an
  `OPENTUNNEL_INVITE` fallback test.
