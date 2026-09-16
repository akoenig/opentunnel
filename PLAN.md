# OpenTunnel v2 (beta.opentunnel.sh) — Implementation Plan

Temporary, self-terminating command tunnel between a remote machine (host) and a coding agent on another machine, built on [tailcat](https://github.com/tailscale/tailcat).

Status: design approved, ready for implementation. Everything in "Verified facts" was tested against the real binary on 2026-09-16; do not re-derive it. Section 16 records the product decisions taken on 2026-09-16 that changed the original draft.

---

## 0. Relationship to 1.x

- `main` is the v2 line. The former `main` (Go relay + CLI) lives on branch `1.x`, created from `fcde5b2`, and stays releasable: its `scripts/release.sh` and workflows must be re-pointed from `main` to `1.x` in one commit on that branch.
- v2 is a breaking change in implementation and trust model, not in UX. The three-step flow stays: run one command on the remote machine, paste the printed prompt into the agent, Ctrl+C to end.
- v2 is published at `https://beta.opentunnel.sh` during beta. `https://opentunnel.sh` keeps serving 1.x until v2 goes GA.
- The Go code (`cmd/`, `internal/`, `deploy/`, `docs/public-v1/`, `scripts/release.sh`, `go.mod`, `go.sum`) is removed from `main`. Kept: `LICENSE`, `VERSION`, `README.md`, `AGENTS.md` (rewritten), `website/`.

---

## 1. Goal

```
host$ curl -fsSL https://beta.opentunnel.sh | sh
# downloads tailcat to a temp dir, starts a tunnel, prints a prompt

# user pastes the prompt into a coding agent on another machine; the agent runs:
agent$ curl -fsSL https://beta.opentunnel.sh/agent | sh -s -- tcXXXX...
agent$ /tmp/opentunnel-agent.abc123/remote 'uname -a'
```

Non-goals for v2: Windows host, sandboxing the remote shell, own DERP relay, interactive shells (PTY), multiple clients per tunnel, accounts, dashboards, package-manager distribution, install-to-system, daemon mode, approval workflows, MCP.

Explicitly in scope for v2 (lifted from the 1.x non-goals): file transfer, concurrent commands, SSH as transport, a host-local session-scoped audit log.

---

## 2. Verified facts about tailcat (tested 2026-09-16)

| Fact | Consequence |
|---|---|
| Latest release is v0.6.0. ForceCommand (`serve no-auth-ssh -- cmd`) and the `exec` service exist only on `main` (commit `79dc7eff30d78fbe9a2c69c725eb05c82d0d542d`). v0.6.0 rejects `--`. | Build our own binaries from the pinned main commit. |
| Prebuilt binaries: linux amd64/arm64/armv7, windows amd64/arm64. No darwin. | Own build pipeline is required anyway. |
| `tailcat serve` with no args accepts exactly one connection on any port, pipes it to stdout, exits. | Used as the one-shot "claim" channel. |
| `serve --allow=nodekey:...` restricts the tunnel at the WireGuard layer; other peers' handshakes are silently ignored. | Peer pinning after claim. |
| `genkey --key=<path-with-slash>` writes the key to that exact path and prints the tailcat address to stdout. `--embed-derp-map` makes the address self-contained (no DERP map fetch on the client). | Address is known before anything listens, so phase 1 and phase 2 share it. |
| `genkey --client --key=<path>` prints `nodekey:<64 hex>`. `printpub` prints the same for the key that would be used. Client modes take `--key=<path>` too. | Agent identity is stable across commands. |
| With `--key=new` every client invocation has a different node key. | Agent must always pass its saved key. |
| `TAILCAT_PEER_KEY` and `TAILCAT_REMOTE_ADDR` are exported into the SSH session / ForceCommand. | Audit log can record the peer. |
| `tailcat ssh <addr> -- <cmd>` execs the system `ssh` with a ProxyCommand; no host-key prompt appeared; stdin/stdout are forwarded. Remote shell ran as the host user in `$HOME`. | Agent needs an `ssh` client installed. stdin forwarding enables file transfer. |
| `XDG_CONFIG_HOME` is honored on Linux, but Go's `os.UserConfigDir` ignores it on macOS. | Always use explicit `--key=<path>`; never rely on the config dir. |
| No idle-timeout, TTL, or max-connection flags. | Lifetime is enforced by a supervisor around the process. |
| `ping --until-direct` reported a direct path in the first pong on a LAN test. Public DERP relays are rate-limited and revocable. | Acceptable for v2; own `derper` is a one-flag change later (`genkey --region=derp.example.com`). |
| SECURITY.md: threat model assumes both ends are the same person. | Matches our use. Do not offer the tunnel to third parties. |
| GNU `timeout` and GNU `stat -c` are missing on macOS; `/bin/bash` on macOS is 3.2. | Scripts must be bash-3.2 compatible and avoid GNU-only tools. |
| tailcat is licensed BSD-3-Clause. | Compatible with MIT. Redistributed binaries ship with `LICENSE.tailcat`; README credits tailcat. |

Not yet verified (check in milestone 3, design does not depend on the answer):
- Whether the claim client (`printf key | tailcat ADDR`) exits on its own once the host reads the key and closes. See section 10 step 4.
- Whether `sftp` (SSH subsystem request) reaches ForceCommand. `scp`/`rsync` go through `SSH_ORIGINAL_COMMAND` and should work unchanged.

---

## 3. Architecture

```
                    beta.opentunnel.sh (Cloudflare Worker, static assets)
                    ├── /            host.sh
                    ├── /agent       agent.sh
                    └── /bin/<ver>/  tailcat_<os>_<arch>, SHA256SUMS, LICENSE.tailcat

HOST (remote machine)                       AGENT MACHINE
host.sh                                     agent.sh
 ├─ download+verify tailcat                  ├─ download+verify tailcat
 ├─ genkey --key=$WORK/session.json          ├─ genkey --client --key=$WORK/client.json
 │    → ADDR (printed in prompt)             │
 ├─ PHASE 1  serve (one-shot)   <──────────  ├─ echo nodekey:.. | tailcat --key=.. ADDR
 │    reads nodekey, validates               │
 ├─ PHASE 2  serve --allow=PEER \            ├─ poll: ping until up, ssh -- true
 │     no-auth-ssh -- ot-exec.sh   <───────  ├─ remote 'cmd'   (tailcat ssh ADDR -- cmd)
 │       └─ ot-exec.sh: audit, activity,     ├─ remote --put/--get (cat over stdin/stdout)
 │          bash -lc "$SSH_ORIGINAL_COMMAND" │
 └─ supervisor: claim/idle/[ttl] → end,      └─ remote --close → rm -rf $WORK
    trap → rm -rf $WORK
```

Security model in one paragraph: the tailcat address is a bearer secret and it will end up in the agent's LLM context and provider logs. Therefore the address must stop being sufficient as early as possible. Phase 1 accepts exactly one connection, which delivers the agent's public key. Phase 2 restarts on the same address with `--allow=<that key>`, so from then on possession of the address is useless without the agent's private key, which never leaves the agent's temp dir. The unclaimed window is bounded (default 5 min), inactivity ends the session (default 30 min), an optional hard TTL caps it, and Ctrl+C ends it at any time. The server key is ephemeral and deleted on exit, so the address is useless forever afterwards.

Why ForceCommand: `serve no-auth-ssh` alone hands the pinned peer an unrestricted login shell and tailcat offers no idle, TTL, or audit hooks. `-- ot-exec.sh` makes the server run our wrapper for every session with `SSH_ORIGINAL_COMMAND` set. That single hook refuses interactive shells, writes the session markers and activity timestamps the supervisor needs for idle detection, records the audit line with the peer key, fixes cwd and login environment, and is the future place for command policy. It is also the sole reason for pinning an unreleased tailcat commit.

---

## 4. Repository layout (`main`)

```
opentunnel/
├── README.md
├── AGENTS.md                   rewritten for v2 (section 15)
├── LICENSE                     MIT
├── VERSION                     SemVer, `dev` between releases (as in 1.x)
├── PLAN.md                     (this file)
├── scripts/
│   ├── host.sh                 served at /
│   ├── agent.sh                served at /agent
│   └── ot-exec.sh              ForceCommand wrapper; embedded into host.sh at build time
├── build/
│   ├── build-tailcat.sh        clone pinned commit, cross-compile, write SHA256SUMS + LICENSE.tailcat
│   ├── embed.sh                inlines ot-exec.sh + version + checksums into scripts → dist/
│   └── TAILCAT_COMMIT          pinned upstream commit
├── site/
│   ├── wrangler.jsonc          Worker `opentunnel-beta`, assets dir = ./dist, custom domain beta.opentunnel.sh
│   └── worker/index.js         maps / → host.sh, /agent → agent.sh, everything else → assets
├── test/
│   ├── unit/*.bats             pure-function tests (arch mapping, validation, prompt rendering)
│   └── e2e.sh                  full host↔agent run on one machine
├── website/                    opentunnel.sh (Astro Starlight); copy rewritten for v2 (section 14)
└── .github/workflows/
    ├── ci.yml                  shellcheck + bats + e2e
    ├── release.yml             build binaries, embed, deploy dist/ to the beta Worker
    └── deploy-website.yml      unchanged trigger (main + website/**)
```

`dist/` is what gets deployed: `host.sh`, `agent.sh`, `bin/<VERSION>/…`.

---

## 5. Build pipeline (`build/`)

### 5.1 Versions
- `VERSION` (repo root): SemVer, `dev` between releases, same convention and release ritual as 1.x (bump, verify, commit, tag, reset to `dev`). A new `scripts/release.sh` adapted from 1.x runs the v2 verification set (shellcheck, bats, `bash -n` on dist scripts).
- `build/TAILCAT_COMMIT`: `79dc7eff30d78fbe9a2c69c725eb05c82d0d542d`.
- Binaries and inline checksums are keyed by `VERSION`; development builds use `dev`.

### 5.2 `build/build-tailcat.sh`
- `git clone https://github.com/tailscale/tailcat && git checkout $TAILCAT_COMMIT`
- For each target in `linux/amd64 linux/arm64 darwin/amd64 darwin/arm64`:
  `CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags="-s -w" -o dist/bin/$VERSION/tailcat_${os}_${arch} ./cmd/tailcat`
- Copy the upstream `LICENSE` to `dist/bin/$VERSION/LICENSE.tailcat`.
- Write `dist/bin/$VERSION/SHA256SUMS` (`sha256sum` format, filenames only).
- Go version: whatever `go.mod` in the tailcat repo requires (`actions/setup-go` with `go-version-file`).
- Smoke test the linux/amd64 binary: `./tailcat version` and `./tailcat serve --help | grep -q -- '-- '` (proves ForceCommand support).

Notes: `-s -w` is fine; binary is ~18 MB, under the 25 MiB per-asset limit of Workers static assets. Darwin binaries downloaded via curl do not get the quarantine xattr, so no signing is needed.

### 5.3 `build/embed.sh`
Produces `dist/host.sh` and `dist/agent.sh` from `scripts/` by replacing marker lines:
- `# @@VERSION@@` → `VERSION=...`
- `# @@CHECKSUMS@@` → a `case "$os_$arch"` block mapping each target to its sha256 (pinned inline; do **not** rely only on the downloaded SHA256SUMS file).
- `# @@OT_EXEC@@` (host.sh only) → `cat > "$WORK/ot-exec.sh" <<'OT_EXEC_EOF' … OT_EXEC_EOF`
- `# @@BASE_URL@@` → `BASE_URL="${OPENTUNNEL_BASE_URL:-https://beta.opentunnel.sh}"`

`embed.sh` must fail if any marker is missing or left unreplaced.

---

## 6. Shared script conventions (`host.sh` and `agent.sh`)

- Invocation is `curl … | sh` (and `| sh -s -- <addr>`), exactly like 1.x. The served script starts with a POSIX shim: if `$BASH_VERSION` is empty, it saves the rest of stdin to `$WORK/self.sh` and `exec bash "$WORK/self.sh" "$@"`. Under bash the shim is a no-op. `| bash` works too.
- Body: `set -euo pipefail`. bash 3.2 compatible: no associative arrays, no `mapfile`, no `${var,,}`, no `read -t` reliance for logic.
- All state under `WORK=$(mktemp -d "${TMPDIR:-/tmp}/opentunnel-host.XXXXXX")` (agent: `opentunnel-agent.XXXXXX`), `chmod 700`.
- `trap cleanup EXIT INT TERM`; cleanup kills child processes, then `rm -rf "$WORK"`.
- Platform detection:
  - `uname -s`: `Linux`→`linux`, `Darwin`→`darwin`, anything else → error "unsupported OS".
  - `uname -m`: `x86_64|amd64`→`amd64`, `aarch64|arm64`→`arm64`, else error.
- Download: `curl -fsSL --proto '=https' --tlsv1.2 --retry 3 -o "$WORK/tailcat" "$BASE_URL/bin/$VERSION/tailcat_${os}_${arch}"`, then verify against the inline pinned sha256 using `sha256sum` if present else `shasum -a 256`. Mismatch → delete file, exit 1. `chmod 700`.
- Required tools check up front: `curl`, `sha256sum|shasum`, `mktemp`, `uname`, plus `ssh` on the agent side. Print one line per missing tool and exit 1.
- Logging: status lines to stderr prefixed `[opentunnel]`. Only the prompt block (host) and the helper path line (agent) go to stdout.
- Tunables via env, all with defaults, all validated as non-negative integers (seconds):

| Var | Default | Used by |
|---|---|---|
| `OPENTUNNEL_CLAIM_TIMEOUT` | 300 | host: phase 1 must complete within this |
| `OPENTUNNEL_IDLE` | 1800 | host: no running session and no command for this long |
| `OPENTUNNEL_TTL` | 0 (off) | host: optional hard cap from start of phase 1 |
| `OPENTUNNEL_CWD` | `$PWD` at launch | host: working dir of remote commands |
| `OPENTUNNEL_KEEP_AUDIT` | unset | host: `1` copies the audit log to `$PWD` on exit |
| `OPENTUNNEL_BASE_URL` | `https://beta.opentunnel.sh` | both |
| `OPENTUNNEL_CONNECT_TIMEOUT` | 90 | agent: max wait for phase 2 to come up |
| `OPENTUNNEL_ALLOW_HTTP` | unset | both, tests only: drop `--proto '=https'` |

---

## 7. `host.sh` — detailed behavior

1. Preamble: shim, tool check, platform detection, `WORK`, download + verify.
2. Write `$WORK/ot-exec.sh` from the embedded heredoc, `chmod 700`. Write `$WORK/env` with `OPENTUNNEL_WORK` and `OPENTUNNEL_CWD` using `printf 'NAME=%q\n'` so paths with spaces survive `source`. (ForceCommand does not inherit the script's shell variables reliably, so pass via file, not env.)
3. Generate the server key:
   `ADDR=$("$TC" genkey --key="$WORK/session.private.json" --embed-derp-map 2>"$WORK/genkey.log")`
   Validate `ADDR` matches `^tc[A-Za-z0-9_-]{40,}$`. Record `START=$(date +%s)`.
4. Print the prompt (section 9) to stdout, then to stderr: `[opentunnel] waiting for the agent to claim (timeout 5m). Ctrl-C to abort.`
5. **Phase 1 (claim):**
   - Start `"$TC" --key="$WORK/session.private.json" serve >"$WORK/claim.txt" 2>"$WORK/phase1.log" &`, save PID.
   - Wait loop (1 s tick) until the process exits or `now - START > OPENTUNNEL_CLAIM_TIMEOUT` (then kill it and exit 1 with "no agent claimed the tunnel").
   - Read `claim.txt`; trim whitespace; must match `^nodekey:[0-9a-f]{64}$`. Anything else → log `claim rejected: <first 40 chars, sanitized>` and **exit 1**. Fail closed: the host does not go back to waiting for another claim. A foreign or malformed claim is a signal; the user reruns the host command for a fresh address.
   - Log `[opentunnel] claimed by nodekey:<first 12 hex>…`.
6. **Phase 2 (serve):**
   - `"$TC" --key="$WORK/session.private.json" serve --allow="$PEER" no-auth-ssh -- "$WORK/ot-exec.sh" 2>"$WORK/server.log" &`, save PID.
   - Wait until `server.log` contains `Server listening` (max 30 s) else fail.
   - Log `[opentunnel] active. idle timeout 30m, cwd <OPENTUNNEL_CWD>` (append `, hard limit Xm` when TTL is set).
7. **Supervisor loop** (in the foreground, 2 s tick):
   - If the tailcat process is gone → log reason from tail of `server.log`, exit 1.
   - `active` = number of `sessions/*` markers whose PID is alive (`kill -0`); `last=$(cat "$WORK/activity" 2>/dev/null || echo $START)`.
   - If `OPENTUNNEL_TTL > 0 && now - START >= OPENTUNNEL_TTL` → end with reason `ttl`.
   - Else if `active == 0 && now - last >= OPENTUNNEL_IDLE` → end with reason `idle`.
   - Every 60 s log a heartbeat `[opentunnel] alive, sessions=N, commands=M` (append `, expires in Xm` when TTL is set; the last heartbeat before expiry says so explicitly).
   - End = `kill -TERM $PID`, wait ≤5 s, `kill -KILL`, log `[opentunnel] session ended (<reason>). audit log was $WORK/audit.log (removed with the temp dir)`. Before removing, if `OPENTUNNEL_KEEP_AUDIT=1`, copy `audit.log` to `$PWD/opentunnel-audit-<timestamp>.log`.
8. `cleanup` (trap): kill phase 1/phase 2 PIDs if alive, kill any children of the tailcat process (`pkill -P`), `rm -rf "$WORK"`.

Concurrency: several SSH sessions may run at once. The supervisor only counts them; nothing serializes them.

---

## 8. `ot-exec.sh` — ForceCommand wrapper

Runs once per SSH session on the host, as the host user. Inputs: `$SSH_ORIGINAL_COMMAND`, `$TAILCAT_PEER_KEY`, `$TAILCAT_REMOTE_ADDR`; reads `OPENTUNNEL_WORK`/`OPENTUNNEL_CWD` from the env file located next to itself (`$(dirname "$0")/env`).

```
1. source "$(dirname "$0")/env"
2. if SSH_ORIGINAL_COMMAND is empty: print "interactive shells are disabled; pass a command" to stderr, exit 2
3. mkdir -p "$OPENTUNNEL_WORK/sessions"; touch "$OPENTUNNEL_WORK/sessions/$$"; date +%s > "$OPENTUNNEL_WORK/activity"
4. trap 'rm -f "$OPENTUNNEL_WORK/sessions/$$"; date +%s > "$OPENTUNNEL_WORK/activity"' EXIT
5. append to audit.log: "<iso8601>\t<peer key>\t<remote addr>\t<command, newlines escaped>"   (command line only, never stdin payload)
6. cd "$OPENTUNNEL_CWD" || cd "$HOME"
7. bash -lc "$SSH_ORIGINAL_COMMAND"   (not exec — the trap must run)
8. append "<iso8601>\texit=<rc>" to audit.log; exit rc
```

Rationale: the wrapper is the activity source (markers = running sessions, `activity` = last start/end) and the audit trail. No command filtering in v2. `bash -lc` gives the agent the user's normal login environment (PATH, nvm, etc.); profile files that print to stdout will pollute command output, including `--get` transfers. Document this; it is the user's own machine. Audit and activity writes are single-line appends and safe under concurrent sessions.

---

## 9. Prompt printed by `host.sh`

Printed to stdout between two lines of `────`, before the first stderr log line (as in 1.x). Placeholders filled from the host: `ADDR`, `USER`, `HOSTNAME`, `OS` (`uname -srm`), `CWD`, timeouts in minutes.

```
I opened an OpenTunnel session for you: temporary command access to a remote machine through an encrypted tunnel.

Setup (run once, on this machine):

    curl -fsSL https://beta.opentunnel.sh/agent | sh -s -- <ADDR>

The installer prints the path of a `remote` helper, e.g. /tmp/opentunnel-agent.XXXX/remote.
Use it for every remote command:

    <path>/remote 'uname -a && pwd'
    <path>/remote --put <local file> <remote path>
    <path>/remote --get <remote path> <local file>

Remote host: <USER>@<HOSTNAME> (<OS>), working directory <CWD>.

Rules:
- Commands run non-interactively through SSH as that user with their login environment. No TTY, no interactive programs, no editors, no sudo prompts. Several commands may run at the same time.
- Always ask me to confirm before running anything destructive or irreversible.
- The session ends after <IDLE> minutes without a command[ and <TTL> minutes in total]. A connection error means it has ended: report that to me and stop; do not retry in a loop.
- Do not copy the address into shared logs, tickets, summaries, or long-lived notes. Do not persist the helper or its keys anywhere else. When finished, run `<path>/remote --close`.

Task:
```

The user appends the task after pasting. Keep the prompt free of the words "password"/"token" so secret-scanners in agent tooling do not redact the address. The TTL clause is only rendered when `OPENTUNNEL_TTL` is set.

---

## 10. `agent.sh` — detailed behavior

Invocation: `curl -fsSL https://beta.opentunnel.sh/agent | sh -s -- <ADDR>`.

1. Preamble: shim, tool check (incl. `ssh`), platform detection, `WORK=$(mktemp -d "${TMPDIR:-/tmp}/opentunnel-agent.XXXXXX")`, download + verify.
2. Validate `ADDR` (`^tc[A-Za-z0-9_-]{40,}$`), else exit 2 with usage. Optional sanity: `"$TC" parse "$ADDR" >/dev/null`.
3. `PUB=$("$TC" genkey --client --key="$WORK/client.private.json" 2>/dev/null | grep -o 'nodekey:[0-9a-f]\{64\}')`.
4. **Claim:** `printf '%s\n' "$PUB" | "$TC" --key="$WORK/client.private.json" "$ADDR" &` in the background. Its exit code is **not** the success signal, because it is unverified whether the client exits on its own after the host reads the key. Success is defined by step 5.
5. **Wait for phase 2:** loop up to `OPENTUNNEL_CONNECT_TIMEOUT` (2 s tick): `"$TC" --key=… ping --timeout=3s "$ADDR"` until success, then `"$TC" --key=… ssh "$ADDR" -- true` until exit 0. On success kill the claim client if still running. On timeout kill it and exit 1 with "could not reach the host; the tunnel may have ended or already been claimed".
6. Write `$WORK/remote` (`chmod 700`):
   ```
   #!/usr/bin/env bash
   set -u
   WORK="<absolute $WORK>"; ADDR="<ADDR>"
   TC="$WORK/tailcat --key=$WORK/client.private.json"
   case "${1:-}" in
     --close) rm -rf "$WORK"; echo "tunnel client removed"; exit 0 ;;
     --put)   [ $# -eq 3 ] || { echo "usage: remote --put <local> <remote>" >&2; exit 2; }
              exec $TC ssh "$ADDR" -- "cat > $(printf '%q' "$3")" < "$2" ;;
     --get)   [ $# -eq 3 ] || { echo "usage: remote --get <remote> <local>" >&2; exit 2; }
              exec $TC ssh "$ADDR" -- "cat $(printf '%q' "$2")" > "$3" ;;
     "")      echo "usage: remote '<command>' | remote --put <local> <remote> | remote --get <remote> <local> | remote --close" >&2; exit 2 ;;
   esac
   exec $TC ssh "$ADDR" -- "$@"
   ```
   Do **not** install a trap that deletes `$WORK` in agent.sh (the helper must outlive the installer). The installer's cleanup only runs on failure.
7. Print to stdout exactly one line the agent can parse: `remote helper: <absolute path>/remote`, then a short reminder of the idle rule to stderr.

Exit codes of `remote` are the remote command's exit codes (ssh passes them through). Connection failures surface as ssh exit 255. `--put`/`--get` are plain `cat` over the forwarded stdin/stdout.

8. Also write `$WORK/ssh` (`chmod 700`), a drop-in `ssh` for `scp` and `rsync`. tailcat's client mode is a stdio pipe to the phase 2 SSH service, so it serves as `ProxyCommand`; the host argument is ignored and `ADDR` is baked in:
   ```
   #!/usr/bin/env bash
   exec ssh -o ProxyCommand="$WORK/tailcat --key=$WORK/client.private.json $ADDR" \
            -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$@"
   ```
   Usage from the agent machine: `rsync -av -e "$WORK/ssh" ./dir remote:/path` and `scp -O -S "$WORK/ssh" file remote:/path`. On the host these arrive as `rsync --server …` / `scp -t …` through ForceCommand, so no host changes are needed; `rsync` must exist on both ends. `scp` needs `-O` because OpenSSH 9+ defaults to the SFTP subsystem, which is not expected to reach ForceCommand (verify in milestone 3). Both commands are printed by the installer (stderr) and included in the prompt's file-transfer lines. e2e gains an `rsync` round-trip case.

---

## 11. Hosting (`site/`, Cloudflare)

- One Worker `opentunnel-beta` with static assets from `site/dist` (copied from `dist/` at deploy time), custom domain `beta.opentunnel.sh`. `worker/index.js` rewrites `/` → `host.sh` and `/agent` → `agent.sh` via `env.ASSETS.fetch`, everything else falls through to assets. No proxying, no redirects to other hosts.
- Scripts: `Content-Type: text/plain; charset=utf-8`, `Cache-Control: no-cache`.
- `/bin/<VERSION>/…`: `Cache-Control: public, max-age=31536000, immutable`.
- `release.yml` runs on tag push: build binaries, embed, `wrangler deploy` from `site/`. Keep previous `bin/<version>` directories in the assets so an already-printed prompt keeps working during a deploy (assets are additive per deploy; the workflow downloads the previous release's `bin/` directories before deploying).
- Secrets: reuse `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` from the website deploy.
- At GA, the `opentunnel.sh` Worker (`website/worker/index.js`) switches its `/` curl handler and `/cli` to the v2 scripts; not part of beta.

---

## 12. Tests

### Unit (`test/unit`, bats)
- Platform mapping table (all accepted/rejected `uname` values).
- Address validation and nodekey validation (accept/reject cases incl. trailing newline, CRLF, injected text).
- Prompt rendering contains address, cwd, correct minute values for custom `OPENTUNNEL_IDLE`, and the TTL clause only when `OPENTUNNEL_TTL` is set.
- `env` file round-trips a cwd with spaces and quotes through `source`.
- `embed.sh` fails on missing markers and produces scripts that pass `bash -n` and shellcheck.
- The POSIX shim: piping the script into `sh` and into `bash` both reach the bash body with arguments intact.

### End-to-end (`test/e2e.sh`, Linux, needs internet for DERP)
Serve `dist/` locally (`python3 -m http.server` in background), `OPENTUNNEL_BASE_URL=http://127.0.0.1:8000`, `OPENTUNNEL_ALLOW_HTTP=1`.
1. Start `host.sh` in background with `OPENTUNNEL_IDLE=20 OPENTUNNEL_CWD=$(mktemp -d)`, capture stdout, extract `ADDR` from the prompt.
2. Run `agent.sh -- ADDR` via `sh`, capture helper path.
3. `remote 'echo hi; pwd'` → output `hi` and the cwd; exit 0. `remote 'exit 7'` → exit 7.
4. File transfer: `remote --put f remote-f`, `remote --get remote-f f2`, `cmp f f2`; and the raw form `remote 'cat > g' < f`.
5. Concurrency: two `remote 'sleep 3; echo $$'` in parallel finish in ~3 s total, both exit 0; heartbeat reported `sessions=2`.
6. Unauthorized peer: fresh client key `tailcat --key=new ssh ADDR -- true` must fail within 15 s.
7. Interactive attempt: `tailcat ssh ADDR` with no command → exit 2 and the usage message.
8. Idle: sleep 25 s → host process has exited with reason `idle`; `remote true` fails; host `WORK` dir is gone.
9. TTL: new host with `OPENTUNNEL_TTL=10` and an agent running `remote 'sleep 60'` → host ends with reason `ttl` within 15 s.
10. Claim timeout: new host with `OPENTUNNEL_CLAIM_TIMEOUT=5`, no agent → exits 1 within 10 s, dir gone.
11. Bad claim: send `garbage` to a fresh phase 1 → host exits 1 with "claim rejected", does not reopen.
12. `remote --close` removes the agent dir.

### CI
`ci.yml`: shellcheck (`-s bash`), bats, then e2e on `ubuntu-latest` over public DERP. If DERP rate limits make it flaky, e2e moves to a nightly workflow; unit tests stay on every push.

---

## 13. Milestones

1. **Repo reset** — delete Go code and 1.x-only docs/scripts from `main`; re-point `1.x` release tooling to its branch; rewrite `AGENTS.md` (section 15).
2. **Build + hosting** — `build/`, `site/`, `release.yml`; binaries reachable at `https://beta.opentunnel.sh/bin/<ver>/…` with checksums and `LICENSE.tailcat`. Done when `curl` of each binary verifies and `serve --help` shows `--` support.
3. **host.sh** — sections 6–9; manual test with a hand-run `tailcat` client on a second machine. Verify the two open facts in section 2.
4. **agent.sh + remote** — section 10; manual two-machine test with a real coding agent, including `--put`/`--get` and parallel commands.
5. **Tests + CI** — section 12 green.
6. **README** — usage, tunables, security notes (section 14), tailcat credit, how to cut a release.
7. **Website copy** — `website/` rewritten for v2 (section 14.2); deployed to opentunnel.sh only when v2 goes GA, prepared on `main` before that.

---

## 14. Documentation

### 14.1 Security notes for the README
Mitigated: address leakage after claim (pinned peer), stale addresses (ephemeral key deleted on exit), unattended tunnels (claim window, idle timeout, optional TTL, Ctrl+C), binary tampering (pinned commit + inline sha256 + TLS), interactive shells (ForceCommand refuses empty command).

Residual, by design:
- Race during the unclaimed window: whoever connects first is pinned. Window ≤ 5 min and the address exists only in the user's clipboard and the agent's context. If the legit agent then fails to claim, the host log shows a foreign key prefix and has already exited; the user reruns for a fresh address.
- The agent machine (and its LLM session) has full access as the host user for the session lifetime, including file transfer. There is no command allowlist. `audit.log` records every command line (never payloads) for the session and is removed with the temp dir unless `OPENTUNNEL_KEEP_AUDIT=1`.
- Without `OPENTUNNEL_TTL`, an agent that keeps issuing commands keeps the session alive; the heartbeat and audit log make that visible, and Ctrl+C ends it.
- Public Tailscale DERP relays can rate-limit or disappear. Switch to a self-hosted `derper` by changing the `genkey --region` argument.
- Upstream tailcat is experimental and its threat model assumes a single trusted operator on both ends.

### 14.2 Website (`website/`)
The three-step UX and the core positioning stay: tool calls as if local, ephemeral, end-to-end encrypted, no accounts, no standing access, Ctrl+C revokes. Everything built on "the relay" is replaced by the claim-then-pin story.

| Page | Change |
|---|---|
| `index.mdx` hero, "Three steps" | Keep. Drop "No SSH" (4×); example prompt popover shows the section 9 prompt. |
| `index.mdx` "You don't have to trust the relay", "Use mine, or run your own" | Replace with "The address stops working the moment your agent connects" (peer pinning, ephemeral key, lifetimes). Self-hosting section removed for beta. |
| `getting-started.mdx` | Agent step becomes one-time `agent` install + `remote`; remove "Prefer your own relay?". |
| `concepts/how-it-works.md` | Actors: host, agent, tailcat (WireGuard, DERP fallback). Claim phase and pinning as the core. |
| `concepts/security-model.md` | Bearer address until claim, pinned peer, claim/idle/TTL, local audit log, DERP sees ciphertext only, tailcat threat model. |
| `reference/cli.md` | Host script, agent script, `remote` helper (`--put`, `--get`, `--close`), `OPENTUNNEL_*` tunables, exit codes. |
| `reference/scope-and-non-goals.md` | Lift file transfer, concurrent commands, SSH transport, session-scoped audit log. Keep the rest of the exclusions. Relay items removed. |
| `guides/self-hosting.md`, `guides/relay-operations.md` | Removed. Optional later: self-hosted `derper` guide. |
| `worker/index.js` | At GA: `/` (curl) and `/cli` serve/redirect to the v2 host script, add `/agent`. Untouched during beta. |

`AGENTS.md` writing-style rules apply (no em dashes, "remote machine", "expires"/"ends" wording). Keep `README.md` consistent with the website.

---

## 15. `AGENTS.md` for v2 (summary of the rewrite)

- Project context: bash scripts + pinned tailcat build; no Go module. `website/` section unchanged.
- Shell style: bash 3.2 compatible, `set -euo pipefail`, shellcheck clean, no GNU-only tools, POSIX shim at the top of served scripts.
- Verification: `shellcheck -s bash scripts/*.sh build/*.sh test/e2e.sh`, `bats test/unit`, `build/embed.sh && bash -n dist/*.sh`, e2e when network is available.
- Design constraints: one host process, one pinned client, claim-then-pin, ephemeral keys, temp-dir-only state, host-local session-scoped audit log, concurrent commands and file transfer allowed. Still excluded without approval: accounts, dashboards, package-manager distribution, install-to-system, daemon mode, PTY, multiple clients, approval workflows, MCP.
- Writing style and commit conventions carried over verbatim.

---

## 16. Decision log (2026-09-16)

| Topic | Decision |
|---|---|
| Branching | Former `main` → `1.x` (done, pushed). `main` = v2. `1.x` stays releasable. |
| Domain | `https://beta.opentunnel.sh` for beta; `opentunnel.sh` switches at GA. |
| Hosting | Cloudflare Worker with static assets (`site/`), deployed by `release.yml`. |
| Versioning | SemVer `VERSION` + tag, as in 1.x. Upstream commit pinned in `build/TAILCAT_COMMIT`. |
| Invocation | `curl … \| sh` stays (POSIX shim re-execs under bash); `\| bash` also works. |
| Naming | `opentunnel-host.XXXXXX` / `opentunnel-agent.XXXXXX`, `[opentunnel]` log prefix, `OPENTUNNEL_*` env, helper `remote`. |
| Go code | Removed from `main` together with `deploy/`, `docs/public-v1/`, `scripts/release.sh`. |
| Claim | Exactly one claim, then pinned. Bad or foreign claim → exit 1, no reopen. |
| Lifetime | Idle 30 min default; TTL off by default (`OPENTUNNEL_TTL` opt-in); Ctrl+C always. |
| Claim client | Exit code not used as signal; success = phase 2 reachable; watchdog kills it. |
| Shell | `bash -lc` (login environment), stdout noise from profiles documented. |
| Env file | Written with `printf %q`. |
| File transfer | In scope: `remote --put/--get` plus raw `cat` over stdin/stdout. |
| Concurrent commands | In scope; supervisor counts sessions, nothing serializes. |
| Multiple clients | Out of scope; `--allow` gets exactly one key. |
| Audit log | Host-local, session-scoped, command lines only; `OPENTUNNEL_KEEP_AUDIT=1` copies it out. |
| DERP | Public Tailscale DERP for beta. |
| License | tailcat BSD-3-Clause shipped as `LICENSE.tailcat`, credited in README. |
| Builds | linux/darwin × amd64/arm64 only; no armv7, no Windows. |
| Per-command timeout | None. |
| Website | Copy rewritten for v2 (section 14.2), published at GA. |

---

## 17. Implementation notes (2026-09-16, after building it)

The design holds. Four things were wrong in the draft and are corrected in the code; both open questions from section 2 are answered.

| Topic | What the plan said | What the implementation does, and why |
|---|---|---|
| POSIX shim | The served script saves the rest of stdin to `$WORK/self.sh` and re-execs bash. | Does not work: dash reads the whole script from the pipe into its buffer, so `cat` gets nothing and the script exits silently. `build/embed.sh` wraps the bash body in a quoted heredoc instead: the shim writes the body out and always re-execs bash. Verified under `sh`, `dash`, and `bash`, piped and as a file. The shim also creates the temp directory and passes it as `OPENTUNNEL_WORK_DIR`, so there is still exactly one directory per process. |
| Client command form | `tailcat ssh <addr> -- <cmd>`. | tailcat passes `--` through as part of the command, so the remote `bash -lc` receives `-- true` and exits 2. The correct form is `tailcat ssh <addr> <cmd>`, with no separator. |
| One tailcat client per command | `remote` runs `tailcat ssh` per invocation. | A tailcat client holds one connection per node key: three concurrent client processes sharing the agent's key produced one success, one dial timeout, and one process that hung indefinitely. The agent now opens one shared SSH connection (`ControlMaster`, `ControlPersist`, `ControlPath` in the temp directory) over a single tailcat client used as `ProxyCommand`, and every `remote` call, `scp`, and `rsync` rides on it. Four concurrent commands verified: all exit 0, the host counts four sessions. |
| ProxyCommand port | `tailcat --key=… <addr>` (default port). | The forced-command SSH service listens on port 22, so the ProxyCommand is `tailcat --key=… <addr> 22`. Without the port, scp and rsync fail with a dial timeout. |

Answers to the open questions:

- **The claim client exits on its own** once the host has read the key and closed the one-shot connection (verified: phase 1 exits after about 2 s, the client right after). The agent still waits for it and kills it if it lingers, because a second client process with the same key would fight over the tunnel connection.
- **SFTP does not reach ForceCommand.** `sftp` fails with "subsystem request failed", so `scp` needs `-O` and `rsync` works unchanged. Both are documented and covered by the end-to-end test.

Smaller decisions taken while implementing:

- The generated `ssh` wrapper sets `BatchMode`, `ConnectTimeout`, and `ServerAlive*` so a command against an ended session fails instead of hanging.
- `deploy-website.yml` on `main` is `workflow_dispatch` only during the beta, because the v2 copy prepared on `main` must not go live on the apex domain before GA. The `1.x` branch deploys the apex site in the meantime.
- `scripts/release.sh` exists on `main` with the v2 verification set (shellcheck, bats, embed, `bash -n`); `1.x` keeps its own, re-pointed at the `1.x` branch.
