## Agent Handoff Brief

OpenTunnel is a product concept for ephemeral, relay-routed, end-to-end encrypted remote command execution for AI agents.

The goal of this note is to let another agent continue from here without needing the original conversation.

## One-Sentence Product Definition

OpenTunnel lets a user run one temporary command on a host, paste the generated prompt into an AI agent, and allow that agent to execute commands on the host through an end-to-end encrypted relay tunnel.

## Core Product Principle

> Zero persistent relay state, one foreground host process, one client, one temporary CLI on each side.

## First Major Version Direction

Build the first major version around the full intended product experience, not a throwaway prototype:

- Host starts a foreground tunnel with `create`.
- Client runs one-off remote commands with `exec`.
- Both sides use a temporary `opentunnel` CLI binary downloaded from the relay’s `/cli` endpoint.
- Relay persists nothing.
- Relay routes only opaque encrypted packets.
- Only one client can connect to a tunnel.
- No login, accounts, dashboard, daemon mode, MCP, raw SSH, approval workflow, or multi-client support in the first major version unless explicitly re-scoped later.

## User Problem

An AI agent often runs on one machine, while the bug or system state it needs to inspect exists on another host.

The user wants to grant the agent ad-hoc access to the affected host without:

- configuring SSH,
- installing permanent software,
- opening inbound firewall ports,
- logging in on both sides,
- setting up MCP servers,
- using a persistent relay service with stored sessions or logs.

## Target User Experience

### Host Side

The user runs this on the host that should be debugged:

```bash
curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- create
```

Expected behavior:

1. `/cli` returns a small installer script.
2. The installer infers the relay origin from the URL that served `/cli`.
3. The installer downloads the matching `opentunnel` binary into the system temp directory.
4. The installer passes the inferred relay origin to the binary as hidden runtime context, not as a user-facing flag.
5. The binary starts a foreground tunnel session.
6. The host connects outbound to the same relay origin that served the installer.
7. The command prints an agent-ready prompt.
8. The command stays running.
9. `Ctrl+C` immediately closes the tunnel.
10. If the process exits or the OS terminates it, the tunnel ends.
11. Temporary files are cleaned up best-effort, but it is acceptable to rely on the system temp directory being cleared later.

### Agent/Client Side

The host command prints a prompt that the user pastes into the agent.

The prompt tells the agent to run commands like this:

```bash
curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- exec \
  --invite 'ot1_b6Q9nF3sV2mK8pL4rT7xA0cH5z...' \
  -- 'hostname && uname -a && pwd'
```

The client-side `opentunnel` binary should be cached in the system temp directory for the session, so repeated commands do not re-download it.

Do **not** expose a `--cache-session` flag. Caching is default behavior.

## Example Generated Agent Prompt

The host should print something close to this:

```text
I opened an OpenTunnel session for you.

Run commands on my host with:

curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- exec \
  --invite 'ot1_b6Q9nF3sV2mK8pL4rT7xA0cH5z...' \
  -- '<COMMAND>'

Start with:

curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- exec \
  --invite 'ot1_b6Q9nF3sV2mK8pL4rT7xA0cH5z...' \
  -- 'hostname && uname -a && pwd'

Commands execute without per-command approval while this foreground session is running.
Treat the invite as bearer-secret material. Do not copy it into shared logs, tickets, summaries, or long-lived notes. The host owner can revoke access with Ctrl+C.

Host:
api-staging

Notes:
- Use non-interactive commands.
- No PTY or interactive stdin is available in the first major version.
- Avoid sudo unless it is passwordless and non-interactive.
- Avoid long-running commands unless necessary.
- Only one client can connect to this tunnel at a time.
- Only one command runs at a time.
- If shell quoting becomes awkward, use the safer encoded-command form shown by the CLI help instead of hand-escaping complex multiline commands.
- The temporary OpenTunnel CLI is cached in the system temp directory during the session.
```

The host-side terminal should also stay readable for humans. It should print compact local status lines for client connection, command start, command finish, timeout, truncation, and session close. These local logs are for the host owner only and must not be sent to the relay as plaintext.

## Naming And Commands

Product name: `OpenTunnel`

CLI binary name:

```text
opentunnel
```

Canonical installer URL:

```text
/cli
```

First major version host command:

```bash
curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- create
```

First major version client command pattern:

```bash
curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- exec \
  --invite '<inviteCode>' \
  -- '<command>'
```

Do not implement for the first major version:

```text
opentunnel stop
opentunnel list
opentunnel logs
opentunnel create --yolo
```

Reason: `create` is a foreground long-running process. The session exists while the process exists. `Ctrl+C` is the stop mechanism. Command execution without per-command approval is the default first-version behavior, so a separate `--yolo` flag adds cognitive overhead without adding safety.

## CLI Subcommands For The First Major Version

### `create`

Starts a host-side tunnel.

No mode flag is required in the first major version. Commands execute without per-command approval because the host owner deliberately started a temporary foreground session.

Do **not** expose a `--yolo` flag in the first major version.

Behavior:

- Generate session identity and invite code.
- Connect outbound to relay.
- Register active host connection in relay memory.
- Print agent-ready prompt.
- Print readable local status lines while the session is alive.
- Keep running until interrupted, idle timeout, or unrecoverable relay failure.
- On `Ctrl+C`, close tunnel, terminate any active command best-effort, and disconnect from relay.

### `exec`

Runs one command on a tunnel host.

Shape:

```bash
opentunnel exec --invite '<inviteCode>' -- '<command>'
```

Behavior:

- Parse invite code.
- Decode relay URL from invite code and connect to that relay.
- Establish E2E session with host.
- Send encrypted command payload.
- Stream encrypted stdout/stderr output as it arrives.
- Print streamed output immediately in agent-friendly terminal style.
- Exit with the remote command's exit code after the final encrypted `execExit` message.

## Relay Responsibilities

The relay is a self-contained process/service.

It should:

- serve `/cli`,
- serve matching CLI binaries/checksums/signatures,
- accept host tunnel connections,
- accept client connections,
- route opaque encrypted packets between host and client,
- keep only in-memory active connection state,
- reject a second client for the same session in the first major version,
- drop active tunnels on restart.

It should not:

- persist sessions,
- persist invite codes,
- persist command logs,
- persist audit logs,
- persist client metadata,
- persist payloads,
- require a DB,
- require Redis,
- terminate command encryption,
- read commands, outputs, exit codes, host metadata, client identity, or user-facing invite-code contents.

## Relay State Model

The relay persists nothing.

Allowed only in memory while active:

```text
sessionId -> hostConnection
sessionId -> clientConnection
connectionId -> sessionId
```

If the relay restarts:

- all active tunnels disappear,
- host/client connections fail,
- users can recreate tunnels.

This is acceptable for the first major version because all sessions are ephemeral.

Deployment config is allowed, for example:

- public URL,
- TLS config,
- embedded/released CLI binaries,
- optional signing key.

But runtime session state must not survive restart.

## Relay Liveness And Cleanup

Keep relay cleanup simple in v1.

Product rule:

> If the host closes the tunnel, or the relay connection is no longer reachable, the tunnel is cleaned up.

Required v1 behavior:

- When the host-side `create` process exits or disconnects, the relay removes the host route and any paired client route from memory.
- When the relay becomes unreachable, host and client commands fail and local processes should exit with a clear error.
- When a client connection closes after a command, the relay removes that client route from memory and the host session remains available for the next sequential `exec` while the host is still connected.
- When the relay restarts, all in-memory state is gone and all active tunnels disappear.
- Use a fixed v1 idle session timeout that closes forgotten sessions after no authenticated command activity. Do not expose this as a user-facing policy flag in v1.
- Do not add hidden complexity such as persistent cleanup jobs, databases, Redis, resumable sessions, or stored tunnel metadata.
- The implementation may use basic transport liveness detection such as WebSocket close/ping/pong or normal read/write failure handling, but only as connection cleanup, not as product-level expiry.

## Relay Operator Observability

Do not add Prometheus metrics, dashboards, or monitoring endpoints to the core v1 implementation.

For v1, simple relay stdout logging is acceptable and useful for operators running the relay directly, in Docker, or under systemd.

Recommended behavior:

- Log relay startup, public URL, listening address, and CLI artifact/version information.
- Log host tunnel connect/disconnect events without invite codes, host metadata, client identity, commands, output, or exit status.
- Log client connect/disconnect and second-client rejection events without relay-private or user-secret material.
- Periodically log aggregate relay stats such as active host sessions and active client connections.
- Keep stats aggregate-only. Do not log `sessionId`, route ids, invite-code prefixes, `clientSecret`, host labels, OS details, command content, command status, exit codes, or per-session labels.
- Prefer a compact human-readable line format for v1 rather than adding structured logging configuration.

Example periodic status line:

```text
opentunnel relay status activeHostSessions=3 activeClientConnections=1 uptimeSeconds=842
```

Prometheus metrics can be revisited later as an operator-only feature, ideally on a separate listener bound to localhost by default, but they are not part of the first major version.

Public relay abuse controls should be in-memory and privacy-preserving:

- per-IP active host/client connection caps,
- connection rate limits,
- short unauthenticated/handshake timeouts,
- maximum frame size,
- aggregate bandwidth caps,
- artifact download throttling,
- aggregate counters for rejected connections and routing failures.

These controls do not require accounts, persistent state, command logs, or invite-code logging.

## `/cli` Distribution

The relay should be self-contained and serve the CLI from `/cli`.

Example hosted relay:

```bash
curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- create
```

Example self-hosted relay:

```bash
curl -fsSL https://relay.acme.internal/cli | sh -s -- create
```

`/cli` should return a small shell installer that:

1. detects OS and architecture,
2. determines `relayOrigin` from the request URL used to serve `/cli`,
3. downloads the matching `opentunnel` binary from that relay origin,
4. verifies checksum/signature,
5. stores it under the system temp directory,
6. runs it with the passed arguments and hidden relay-origin context.

Relay origin inference is required for the first major version. A user should not have to pass `--relay`, `--relay-url`, or any similar option when creating a tunnel. Hosted and self-hosted installs should both work from the install URL alone:

```bash
curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- create
curl -fsSL https://relay.acme.internal/cli | sh -s -- create
```

Recommended implementation:

- The relay serves a generated bootstrapper with a canonical `relayOrigin` value embedded, for example `https://relay.opentunnel.dev`.
- Production/self-hosted relays should prefer an explicit configured public URL, for example an operator-only `--public-url` flag on `opentunnel relay`.
- `relayOrigin` is computed from the actual request origin only when safe, with trusted reverse-proxy headers only if the relay has been explicitly configured to trust that proxy.
- The bootstrapper passes `relayOrigin` to the binary via a private environment variable such as `OPENTUNNEL_RELAY_ORIGIN` or an internal hidden flag. This is implementation detail, not documentation surface.
- The binary uses that origin for host-side `create` registration, artifact downloads, and default generated client command URLs.
- The invite code still contains the relay URL so the client can connect to the correct relay when running `exec`.
- If a self-hosted relay is reached through an unexpected origin, the bootstrapper should fail with a clear configuration error rather than silently registering a tunnel against the wrong origin.

Possible HTTP endpoints:

```text
GET /cli
GET /cli/version
GET /cli/v0.1.0/linux/amd64/opentunnel
GET /cli/v0.1.0/linux/amd64/sha256
GET /cli/v0.1.0/linux/amd64/sig
```

Alternative endpoint shapes are acceptable if the relay stays self-contained.

## Temporary Binary Cache

Client-side binary caching is default and hidden.

Suggested cache key:

```text
relayUrl + cliVersion + binaryChecksum + sessionId
```

Suggested cache path:

```text
$TMPDIR/opentunnel/<relayHash>/<sessionId>/<version>/opentunnel
```

Expected behavior:

- Reuse cached binary during the session.
- Avoid re-downloading for every command.
- Keep all files in system temp storage.
- Clean up best-effort where simple.
- Rely on OS temp cleanup if cleanup cannot run.

Do not require users or agents to run explicit cleanup commands in the first major version.

## Architecture

```text
Host
  curl https://relay/cli | sh -s -- create
    |
    | temporary CLI connects outbound
    v
Relay
  /cli serves installer + CLI binary
  /tunnel routes opaque encrypted packets
  keeps only in-memory active connection state
    ^
    | temporary client CLI
    |
Agent/client
  curl https://relay/cli | sh -s -- exec --invite ... -- "docker ps"
```

Command execution flow:

```text
Agent proposes command through temporary CLI
→ client CLI encrypts command payload end-to-end
→ relay routes opaque packet
→ host CLI decrypts request
→ host CLI executes command locally
→ host CLI streams encrypted stdout/stderr chunks as they are produced
→ relay routes opaque packets back
→ client CLI prints streamed output immediately
→ host CLI sends encrypted final exit status
→ client CLI exits with the remote command's exit code
```

The command should feel like a local tool call from the agent's perspective. Output should stream instead of waiting for the full command to finish.

## Security And Privacy Requirements

Hard requirements:

- Command requests, stdout chunks, stderr chunks, and final command status are end-to-end encrypted between client CLI and host CLI.
- Relay routes opaque packets only.
- Relay persists nothing.
- Relay is application-layer blind: it cannot read commands, stdout, stderr, cwd, environment, exit codes, command status, host labels, OS details, client identity, or user-facing invite-code contents.
- The client must decode the invite code locally. The full invite code must never be sent to the relay.
- `clientSecret` must never be sent to the relay. It is used only inside the client-host E2E handshake as the authorization secret.
- Host opens only an outbound connection.
- No inbound firewall setup on host.
- Session terminates when `opentunnel create` exits, the idle timeout fires, or the relay connection is lost.
- In first major version, exactly one client can connect at a time.
- The invite code is scoped to one live foreground session. If the host process exits, the idle timeout fires, or the relay connection is gone, the invite code no longer works.

Relay-visible information should be limited to what is unavoidable for routing and transport. The relay is not metadata-blind; it can still observe routing handles, connection state, timing, packet sizes, source/destination network metadata, and aggregate traffic volume:

```text
opaque sessionId or routing id
connection state
source/destination network metadata visible to the relay transport
timestamps
packet sizes
```

The relay must not receive the full invite code, `clientSecret`, `hostPubKey`, `permission`, or host metadata such as `hostLabel` or `os` as plaintext application fields. Those values can be used locally by the client or host, printed locally in the generated prompt, or included only inside encrypted/authenticated client-facing material.

## Invite Code And Privacy Model

The user-facing invite should be a single opaque code, not a visible JSON payload and not a URL-like mini protocol.

Preferred first major version shape:

```text
ot1_<base64urlNoPadding>
```

Example:

```text
ot1_b6Q9nF3sV2mK8pL4rT7xA0cH5z...
```

The code is intentionally boring: it is easy to copy, paste, quote in a shell command, recognize if accidentally shown, and explain to an AI agent. Avoid formats like:

```text
opentunnel://pair/v1/...
https://relay.example/connect/<sessionId>#...
```

Those look clever, but they increase cognitive load and create browser/history/query/fragment confusion. OpenTunnel v1 does not need a clickable browser invite.

The invite code is bearer-secret material. Anyone who gets the full code while the host process is alive can attempt to become the single client. The first major version should therefore treat the invite code as a high-entropy capability, not as a user identity.

Recommended internal encoding:

```text
ot1_ + base64urlNoPadding(cborInviteBytes)
```

The decoded invite object should contain the material needed by the client CLI:

```json
{
  "v": 1,
  "relay": "https://relay.opentunnel.dev",
  "sessionId": "stn_7K2P",
  "hostPubKey": "x25519:...",
  "clientSecret": "base64url-32-random-bytes",
  "permission": {
    "mode": "yolo"
  },
  "protocol": "Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s"
}
```

Notes:

- `ot1` means OpenTunnel invite format version 1. It is the only user-visible version marker needed.
- `relay` should always be encoded in the invite code, including for the hosted default. This keeps client behavior deterministic and makes self-hosted relays no harder to use.
- `clientSecret` is the invite's bearer capability: possession of the full invite code implies possession of `clientSecret`, and possession of `clientSecret` is the v1 proof that the client may attempt to become the one client for this live tunnel.
- `clientSecret` should be exactly 32 random bytes, encoded as base64url without padding in human-facing examples. Generate it with a CSPRNG for every `create` run and never reuse it.
- `clientSecret` is not a user identity, account token, API key, relay credential, or long-lived device credential. It is a one-session shared secret between the temporary host CLI and the temporary client CLI.
- `hostPubKey` is the public half of a host session key pair generated fresh by the host-side `create` process for this live foreground session. It is not a persistent machine identity, account key, relay key, or long-lived host credential. The corresponding private key lives only in memory inside the running host process and disappears when `create` exits.
- The relay remains stateless with respect to this key. It does not store, validate, or need to understand `hostPubKey`; the client uses it locally to verify that the encrypted channel terminates at the host process that created the invite.
- `permission` should be included in the internal invite payload. For the first major version, it contains `mode: "yolo"` as the only valid permission mode.
- `permission.mode` means command execution without per-command approval while the foreground host session is alive.
- `permission` is internal protocol data, not user-facing configuration. Do not expose `--yolo`, a mode selector, or approval-mode configuration in the first major version.
- Do not require `expiresAt` in the first major version invite payload. The host foreground process is the validity boundary, and `Ctrl+C` is the revocation mechanism.
- A later version may add an optional `expiresAt` or `maxSessionSeconds` safety limit if abandoned foreground sessions become a practical problem, but it should not be required for the first major version.
- Do not include `maxClients` in the first major version. Exactly one client is allowed.
- Do **not** include `hostLabel`, `os`, shell, cwd, environment, or other host metadata in relay-visible fields.
- Prefer not to include `hostLabel` or `os` in the invite code at all. If the client needs those values, the host should send them after the E2E handshake inside an encrypted `helloMetadata` frame.
- The relay should only see the routing value that the client presents for connection setup, such as `sessionId`, plus unavoidable transport metadata. The relay must not need to parse the full invite code.
- If the invite code becomes too long for comfortable copy/paste, solve that by compact binary encoding and short field keys internally, not by exposing a multi-part URL or separate flags.

### Client Secret Semantics

`clientSecret` is the authorization secret inside the opaque invite code. It answers one question: does this connecting client possess the secret that the host generated for this live foreground session?

Keep the responsibilities separate:

```text
sessionId     = routing handle, visible enough for the relay to connect host and client
hostPubKey    = host authenticity check, used by the client during the E2E handshake
clientSecret  = client authorization secret, mixed into the E2E handshake as the PSK
```

Do not use `sessionId` as the authorization secret. The relay needs a routing handle and may see `sessionId` or a derived route id. Authorization should remain end-to-end between the client and host.

### Relay Must Not Receive The Invite Code

The client CLI must parse the invite code locally before opening the relay connection. It should extract only the relay origin and routing material needed for connection setup, then keep the rest of the invite payload local.

Relay connection setup should look conceptually like this:

```text
client CLI decodes ot1_<...> locally
client CLI connects to relay from invite.relay
client CLI presents sessionId or derived routeId for routing
relay pairs client connection with active host connection
client and host perform E2E Noise handshake through opaque relay frames
```

The relay must not receive these values as plaintext application data:

```text
full invite code
clientSecret
hostPubKey
permission
permission.mode
protocol selection, unless needed as non-secret negotiation metadata
commands
stdout or stderr
exit codes or command status
```

If an implementation sends the full invite code to a relay endpoint such as `/connect`, `/join`, `/session`, or `/tunnel`, it violates the v1 privacy model. The relay would then learn `clientSecret`, which would allow it to act as an authorized client or race the intended client. Even if a correct Noise handshake still prevents passive decryption without ephemeral private keys, giving the relay `clientSecret` breaks the intended relay-blind authorization boundary.

Keep the responsibilities separate:

```text
sessionId or routeId = relay routing handle
clientSecret        = end-to-end client authorization secret
hostPubKey          = end-to-end host authenticity check
permission.mode     = end-to-end command-execution semantics
```

Expected lifecycle:

1. Host `create` generates a fresh `sessionId`, host session key pair, and 32-byte `clientSecret`.
2. Host encodes `clientSecret` inside the opaque invite code.
3. User gives the invite code to the agent or client.
4. Client decodes the invite code locally and sends only `sessionId` or a derived `routeId` to the relay for routing.
5. Client mixes `clientSecret` into the Noise handshake as the PSK.
6. Host accepts only a client that proves possession of the same `clientSecret` during the E2E handshake.
7. Relay only routes by `sessionId` or route id and does not parse or validate `clientSecret`.

Security implications:

- Treat the full invite code as secret-like. Do not log it, echo it in analytics, preserve it in summaries, or expose it in relay-visible URLs.
- Never send the full invite code, `clientSecret`, or `permission` object to the relay.
- If another party obtains the invite code before the intended client connects, that party can race to become the active client.
- For the first major version, the invite is a reusable bearer capability for sequential `exec` commands while the host-side `create` process remains alive. This is an accepted product tradeoff for a low-friction agent debugging loop.
- Exactly one client connection and one command may be active at a time, but the next sequential command can be run by anyone who still possesses the invite code.
- First-client pinning and single-use continuation tokens are deliberately deferred. Revisit them if transcript leakage becomes a practical concern.
- If the host foreground process exits, `clientSecret` becomes useless because the live session is gone.
- Redact example or captured invite codes as `ot1_[REDACTED]`; never preserve concrete bearer invite values in documentation or reports.

## Command Payload

Use `camelCase` for attributes.

Example request:

```json
{
  "type": "exec",
  "sessionId": "stn_7K2P",
  "command": "docker ps",
  "cwd": "/srv/app",
  "timeoutSeconds": 30
}
```

Example stream protocol messages:

```json
{
  "type": "execStart",
  "sessionId": "stn_7K2P",
  "commandId": "cmd_9R4M"
}
```

```json
{
  "type": "execOutput",
  "sessionId": "stn_7K2P",
  "commandId": "cmd_9R4M",
  "stream": "stdout",
  "data": "...",
  "encoding": "utf8"
}
```

```json
{
  "type": "execOutput",
  "sessionId": "stn_7K2P",
  "commandId": "cmd_9R4M",
  "stream": "stderr",
  "data": "...",
  "encoding": "utf8"
}
```

```json
{
  "type": "execExit",
  "sessionId": "stn_7K2P",
  "commandId": "cmd_9R4M",
  "exitCode": 0,
  "durationMs": 842,
  "truncated": false
}
```

The client CLI should print `execOutput` chunks as they arrive and then exit with the `execExit.exitCode` value.

## Command Execution Mode

The first major version has one execution mode: `yolo`.

`yolo` is the default and only valid value for `permission.mode` in the first major version. It means command execution without per-command approval.

```bash
curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- create
```

This means:

- commands execute without per-command approval,
- the target owner explicitly chose to open this foreground session,
- the session is temporary,
- the session is foreground-bound,
- `Ctrl+C` revokes access immediately.
- the host terminal remains readable and shows command start/end status locally.

Do not implement approval workflows or expose a mode flag in the first major version. The permission object belongs in the invite-code payload so the protocol is explicit, but users should not have to choose it.

### Structured Host-Side Runtime Log

The host-side `create` process should make the session easy to supervise without becoming noisy. Use structured, scan-friendly local log lines by default: one event per line, stable event names, key-value fields, and readable values. Avoid raw JSON for the default host UX because it is harder for a human to scan in a terminal, but keep the shape regular enough that it can be parsed later.

Recommended default local log style:

```text
opentunnel event=sessionOpen relay=https://relay.opentunnel.dev host=api-staging idleTimeout=30m
opentunnel event=waiting message="waiting for agent command" hint="Ctrl+C closes the tunnel"
opentunnel event=clientConnected route=active
opentunnel event=commandStart commandId=cmd_9R4M timeout=120s cwd=/srv/app command="docker ps"
opentunnel event=commandFinish commandId=cmd_9R4M exitCode=0 duration=842ms outputBytes=3891 truncated=false cleanup=ok
opentunnel event=waiting message="waiting for next command" hint="Ctrl+C closes the tunnel"
opentunnel event=sessionClose reason=ctrlC activeCommand=false cleanup=ok
```

Requirements:

- One event per line.
- Prefix every line with `opentunnel` so logs are easy to filter.
- Use stable `event=<name>` values such as `sessionOpen`, `waiting`, `clientConnected`, `commandStart`, `commandFinish`, `commandTimeout`, `outputTruncated`, `cleanupWarning`, `idleTimeout`, and `sessionClose`.
- Use `camelCase` field names.
- Quote only values that contain spaces or shell-sensitive characters.
- Show relay connection, waiting state, client connect/disconnect, command start, command finish, timeout, output truncation, cleanup failure, idle timeout, and session close.
- Show the command text locally, truncated safely if it is very long.
- Never print the full invite code after the initial generated prompt unless the user explicitly reruns a local display command in a later version.
- Never send command text, output, exit code, host label, or invite material to the relay as plaintext.
- Keep JSON logs as a future or operator option, not the default host-side v1 UX.

## Session And Command Execution Semantics

The first major version should optimize for the real agent debugging loop: the agent usually needs a sequence of small inspection commands, not exactly one command.

Product decision:

> A foreground host tunnel may serve repeated sequential `exec` commands while the `create` process remains alive, but only one command may run at a time. The invite remains a reusable bearer capability during that live foreground session.

V1 lifecycle:

```text
host starts create
→ relay registers active host route in memory
→ client runs one exec command with the invite
→ client decodes invite locally
→ client connects to relay with routing material only
→ client and host complete E2E handshake
→ host executes one command
→ client receives streamed output and final exit code
→ client connection closes after the command
→ host session remains alive for another exec command
→ host Ctrl+C, idle timeout, or host process exit closes the full tunnel
```

This means the v1 CLI UX remains one remote command per `exec` invocation:

```bash
curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- exec \
  --invite '<inviteCode>' \
  -- 'docker ps'
```

The same invite may be used for additional sequential commands until the host-side `create` process exits or the idle timeout closes the session:

```bash
curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- exec \
  --invite '<inviteCode>' \
  -- 'docker logs app --tail=100'
```

V1 command concurrency rules:

- Exactly one remote command may run at a time for a live tunnel.
- Concurrent command execution is a future feature, not part of v1.
- If another `exec` attempts to run while a command is active, v1 should reject it with a simple busy response such as `CommandAlreadyRunningError` rather than queuing, multiplexing, or cancelling commands.
- The relay should also reject concurrent second client connections for the same `sessionId` with atomic first-active-client-wins behavior.
- V1 does not support multiplexed concurrent commands, command cancellation, background command management, or long-lived interactive sessions.
- The generated host prompt should tell the agent that only one command runs at a time.

V1 command defaults:

```json
{
  "timeoutSeconds": 120,
  "maxOutputBytes": 10485760,
  "pty": false,
  "cwd": null
}
```

Semantics:

- `timeoutSeconds` is fixed in v1 and should not be user-configurable.
- `maxOutputBytes` is a combined stdout/stderr limit in v1.
- `pty` is always `false` in v1. PTY support is explicitly desirable for a later version, but not part of the first major version.
- `cwd: null` means the command starts in the working directory of the host-side `create` process.
- The client CLI exits with the remote command's exit code.
- If the host-side `create` process receives `Ctrl+C`, it terminates the tunnel and any currently running command best-effort.
- If the client-side `exec` process disconnects, the host may terminate the currently running remote command best-effort as connection cleanup. Explicit command cancellation is not a v1 protocol feature.
- Run every command in its own process group/session where the platform supports it.
- On timeout, client disconnect, idle shutdown, or host `Ctrl+C`, attempt graceful termination of the process group first, then force termination after a short grace period.
- Backgrounding or daemonizing commands are unsupported in v1; the host log should warn if cleanup may have left child processes behind.

Shell behavior:

- V1 `exec -- '<command>'` runs through the same kind of shell used by the user who created the tunnel.
- On Unix-like systems, this should use the host user's configured shell where practical, falling back to `/bin/sh` if no usable shell is available.
- The shell invocation must be non-interactive.
- The protocol should still keep room for future structured execution fields such as `argv`, explicit environment allowlists, stdin, and PTY support.

Safer command input for agents:

- Keep `-- '<command>'` as the primary simple UX.
- Add a safer path for complex quoting and multiline commands. Preferred v1 candidate names are `--command-base64` for encoded command text and `--command-file -` for reading command text from stdin.
- Naming is not fully settled; optimize for agent readability over clever brevity. `--command-base64` is explicit but long, while `--cmd-b64` is shorter but less self-explanatory.
- The generated host prompt should mention the safer form briefly so agents do not hand-escape complex shell snippets incorrectly.
- The decoded command still runs through the same non-interactive host shell and inherits the same timeout/output limits.

## Idle Session Timeout

The first major version should include a simple inactivity timeout so a forgotten foreground session does not remain open indefinitely.

Recommended v1 default:

```json
{
  "idleSessionTimeoutSeconds": 1800
}
```

Semantics:

- The idle timeout is fixed in v1 and should not require a user-facing flag.
- The timer starts when the host-side `create` session becomes ready.
- The timer is reset by authenticated command activity, such as a successful client handshake, command start, command completion, or clean client disconnect after a command.
- The timer is paused while a command is running; command runtime remains governed by `timeoutSeconds`.
- If no authenticated command activity happens before the idle timeout, the host closes the tunnel and the relay removes the in-memory route.
- The host log should show the idle timeout and print a readable close reason when it fires.
- This is product-level safety, not persistent relay state. It must not require databases, Redis, stored tunnel metadata, or resumable sessions.

## Error Types

Protocol and command failures should be represented as semantic error types.

Naming rules:

- Error names use `PascalCase`.
- Error names are semantic, not transport-specific.
- Error names are suffixed with `Error`.

Initial v1 error types:

```text
InvalidInviteError
HostUnavailableError
ClientAlreadyConnectedError
HandshakeFailedError
CommandAlreadyRunningError
CommandTimeoutError
MaxOutputExceededError
CommandStartFailedError
IdleSessionTimeoutError
InteractiveCommandUnsupportedError
ProtocolError
```

Encrypted error payloads should use `type: "error"` plus an `errorType` value:

```json
{
  "type": "error",
  "errorType": "CommandAlreadyRunningError",
  "message": "Another command is already running for this tunnel.",
  "commandId": "cmd_9R4M"
}
```

The relay may have its own transport-level rejection responses for routing failures, but command/protocol errors that depend on invite contents, permissions, command state, or host execution should remain inside the encrypted client-host channel whenever possible.

## Implementation Direction

OpenTunnel v1 should be implemented as a single Go monobinary named `opentunnel`.

The same binary should be capable of acting as:

```text
opentunnel relay
opentunnel create
opentunnel exec
```

Rationale:

- André can review Go code more comfortably than Rust code.
- The product benefits from one self-contained artifact that can run the relay, host CLI, and client CLI roles.
- The relay can serve the same monobinary from `/cli` for temporary host/client use.
- Shared invite, framing, error, and protocol code reduces drift between relay, host, and client behavior.
- Go has a strong static-binary, cross-compilation, HTTP/WebSocket, and container deployment story.

The monobinary does not expand the first-version product surface. Public examples should still focus on:

```bash
curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- create
curl -fsSL https://relay.opentunnel.dev/cli | sh -s -- exec --invite '<inviteCode>' -- '<command>'
```

Operator/self-hosted relay usage can be documented separately:

```bash
opentunnel relay --listen :8080 --public-url https://relay.opentunnel.dev
```

### Go Noise Library Direction

Go is the preferred implementation language for v1 because of reviewability and distribution fit, but the Go Noise library ecosystem appears less fresh than Rust's `snow` ecosystem.

Use `github.com/flynn/noise` as the intended Go Noise implementation for v1 unless the required spike shows that it cannot cleanly support the protocol requirements.

Before committing to the full implementation, the first engineering milestone must be a focused Noise spike with `github.com/flynn/noise`.

The spike must prove that the library cleanly supports:

- `Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s` and `Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s`, choosing the cleaner implementation after the spike,
- 32-byte raw `clientSecret` mixed as the PSK,
- canonical prologue binding using length-prefixed fields or canonical CBOR, not ad-hoc string concatenation,
- host session public key verification against `hostPubKey` from the invite,
- multiple encrypted transport frames after handshake,
- clear failure on wrong `clientSecret`, wrong `hostPubKey`, wrong prologue, or replayed ciphertext.

If the Go spike cannot support these requirements cleanly, the implementation choice should be revisited. Do not plan to use Rust `snow` from Go in v1 unless the project explicitly accepts cgo/FFI complexity. Rust with `snow` remains only a fallback for the security-bearing protocol implementation, despite the reviewability tradeoff.

Noise-specific code should be isolated behind a narrow internal secure-channel interface so the rest of the product logic remains reviewable and the library can be replaced later if needed.

Suggested monobinary code layout:

```text
cmd/opentunnel/
  main.go

internal/app/
  relay.go
  create.go
  exec.go

internal/relay/
  server.go
  cliBootstrap.go
  routing.go
  websocket.go

internal/tunnel/
  host.go
  client.go
  frames.go
  protocol.go
  errors.go

internal/invite/
  encode.go
  decode.go

internal/securechannel/
  handshake.go
  transport.go
  prologue.go

internal/command/
  runner.go
  shell.go
  limits.go

internal/artifacts/
  manifest.go
  cache.go
  platform.go
```

## Implementation Sketch

A practical implementation can be split into three role areas inside the monobinary.

### 1. Relay

Build a server that exposes:

```text
GET /cli
GET /cli/version or equivalent metadata endpoint
GET /cli/<version>/<os>/<arch>/opentunnel
WebSocket or HTTP upgrade endpoint for tunnel traffic
```

Relay in-memory maps:

```text
hostsBySessionId: Map<string, HostConnection>
clientsBySessionId: Map<string, ClientConnection>
```

First major version behavior:

- Host connects with `sessionId` and auth material from invite-code/session creation.
- Client decodes the invite locally and connects with only the routing material needed by the relay.
- Relay validates only routing material that is strictly necessary and not user-identifying.
- Relay ensures only one client per session.
- Relay forwards encrypted packets between host and client.
- Relay does not receive host labels, OS information, commands, output, exit codes, or client identity as plaintext application data.
- Relay removes all in-memory state on disconnect.

### 2. Host CLI: `create`

Responsibilities:

- Generate session id.
- Generate host session key pair for E2E encryption.
- Generate client secret and invite-code material.
- Connect outbound to relay.
- Print agent prompt.
- Wait for client commands.
- Decrypt command payloads.
- Execute commands locally.
- Stream stdout and stderr chunks as encrypted `execOutput` messages while the command is running.
- Send a final encrypted `execExit` message with exit code, duration, and truncation status.
- Keep running until interrupted.
- Exit on `Ctrl+C` and close relay connection.

### 3. Client CLI: `exec`

Responsibilities:

- Parse invite code.
- Resolve relay URL.
- Connect to relay.
- Perform E2E handshake with host.
- Send one encrypted command request.
- Stream encrypted stdout/stderr output as it arrives.
- Exit with the remote command's exit code after the final encrypted `execExit` message.
- Reuse temporary cached binary where possible.

## Interactive Commands And PTY Scope

Interactive command support is deliberately out of scope for v1 because it changes the product from one-shot command execution into terminal session management.

To support interactive commands well, OpenTunnel would need at least:

- PTY allocation on the host,
- bidirectional stdin streaming,
- terminal resize messages,
- terminal mode handling,
- long-lived session state beyond one command request/response,
- better cancellation and detach semantics,
- stronger backpressure handling,
- clear behavior for password prompts, pagers, editors, REPLs, and curses-style programs,
- a much stronger local visibility story because the agent would effectively operate a live terminal.

This is a scope and complexity expansion. Keep v1 non-interactive and do not add PTY or interactive command support to the first major version. Add PTY later only as an explicit future feature with its own lifecycle and safety design.

## Research Findings From Comparable Products

OpenTunnel should position itself as: tmate-like invitation UX, Tailscale-like relay-blind encryption principle, ngrok-like outbound connectivity, but scoped to one ephemeral CLI-to-CLI command session with no account, daemon, dashboard, sshd, or inbound firewall.

Comparable products and lessons:

| Product | Useful pattern | Avoid for OpenTunnel v1 |
|---|---|---|
| tmate | Foreground process prints an invite; session lifetime is easy to understand | Full interactive terminal sharing, multi-client collaboration, relay-visible terminal trust model |
| Teleconsole | Support-session style UX: run a command, share an invite | Older maintenance posture, browser/web terminal complexity, unclear relay opacity |
| Cloudflare Tunnel / Access | Outbound-only host connection; strong docs around access policy | Account/dashboard/policy setup, named persistent tunnels, L7 visibility depending on mode |
| Tailscale SSH / Funnel | Relay-blind packet forwarding principle; strong identity model | Daemon, tailnet enrollment, persistent device identity, broader service exposure |
| ngrok TCP/SSH | Simple one-command exposure and clear CLI status | Exposing sshd or arbitrary TCP services, account tokens, broad network access |
| GitHub Codespaces / Dev Tunnels | Clear lifecycle and visibility labels such as private/public/temporary | Cloud workspace or account-centric tunnel objects rather than ad-hoc host access |

First major version implications:

- Keep the host lifecycle foreground-bound and obvious.
- Print exactly one agent-ready command.
- Do not expose sshd, arbitrary TCP, a web terminal, or port forwarding in v1.
- Show clear local status lines: relay connected, waiting for client, client connected, command running, exit code, session closed.
- Enforce one-client semantics in both relay admission and host-side protocol state.
- Treat the invite code as a bearer capability and make that explicit in the generated prompt.

## Recommended E2E Protocol

Use a Noise-based transport over a bidirectional relay transport such as WebSocket over TLS.

Recommended first major version protocol, pending the required Go Noise spike:

```text
Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s
```

Rationale: the client already knows the host session public key from the invite, and the client has no durable identity. `NKpsk0` maps naturally to anonymous client plus known host key plus invite PSK. If the Go library support, examples, or failure behavior are cleaner for `XXpsk3`, then `Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s` remains an acceptable fallback, but the tradeoff must be documented.

Roles:

```text
initiator = client CLI running `exec`
responder = host CLI running `create`
```

Inputs:

```text
host session key pair = generated by the host per live session and kept only in memory
hostPubKey = encoded in the invite code so the client can authenticate the live host process
clientSecret = 32-byte PSK encoded in the invite code
```

Handshake requirements:

- The client mixes the raw 32-byte `clientSecret` as the Noise PSK. This proves possession of the invite's bearer capability to the host without asking the relay to authenticate the client.
- The client verifies that the responder host session public key equals `hostPubKey` from the invite code.
- Bind public session context into the Noise prologue. Include the internal `permission.mode` value from the invite code even though v1 has only one valid value, so both sides cryptographically agree on the command-execution semantics.
- Build the prologue from a canonical encoded object, such as canonical CBOR or explicit length-prefixed fields. Do not use raw string concatenation.

Conceptual prologue fields:

```json
{
  "app": "OpenTunnel",
  "inviteVersion": 1,
  "noiseProtocol": "Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s",
  "sessionId": "stn_...",
  "relayOrigin": "https://relay.opentunnel.dev",
  "permissionMode": "yolo",
  "commandDefaults": {
    "timeoutSeconds": 120,
    "maxOutputBytes": 10485760,
    "pty": false,
    "idleSessionTimeoutSeconds": 1800
  },
  "features": ["exec.v1", "stdoutStderr.v1"]
}
```

- Abort if the transcript does not match the invite-code context.
- Do not implement ad-hoc X25519 plus AEAD framing if a maintained Noise library is available.

Fallback if the implementation library supports it more cleanly:

```text
Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s
```

Use `XXpsk3` only if library support, examples, and reviewability are better after the spike. If selected, document that the host session public key is still ephemeral and per-session, not a persistent host identity.

Do not use `IK` unless the product intentionally adds a real client static key. In OpenTunnel v1, possession of the invite-code PSK is the client authentication primitive.

Avoid these as the primary v1 tunnel protocol:

- libsodium sealed boxes alone: good for one-shot encrypted blobs, not a streaming transport.
- age-style encryption: excellent file/message format, not an interactive command tunnel.
- WebRTC data channels: useful later for P2P/browser scenarios, too much signaling and ICE complexity for relay-first v1.
- Full SSH reuse: proven command/channel semantics, but larger attack surface and awkward invite-code PSK integration.
- Magic Wormhole/PAKE: useful later for short human codes, unnecessary when the invite code already contains a high-entropy secret.

## Replay Prevention And Session Admission

Use layered replay prevention:

1. The host generates a new `sessionId`, host session key pair, and `clientSecret` for each `create` run. `sessionId` is routing context; `clientSecret` is the one-session client authorization secret.
2. The relay accepts one active host connection per `sessionId`.
3. The relay admits exactly one client connection for that `sessionId` with atomic first-client-wins behavior.
4. For v1, the host accepts multiple successful E2E handshakes sequentially while the foreground session is alive, because each `exec` invocation is a fresh client process. It must accept only one active handshake/client at a time.
5. Noise ephemeral keys and transport cipher counters provide fresh connection keys and nonce management for each successful handshake.
6. The Noise prologue binds `sessionId`, relay origin, `permission.mode`, and feature set to the handshake transcript.
7. The host rejects duplicate `commandId` values.
8. If the host process exits, the relay connection drops, or the client disconnects after a terminal failure, the session is closed rather than resumable.
9. Do not rely on clock expiry inside the invite for v1 session validity. The live host process plus the fixed idle session timeout are the validity window. The relay may still enforce internal connection cleanup timeouts for resource protection, but those timeouts must not require persistent relay state.

Relay admission state is per live `sessionId` route entry and exists only in memory:

```text
idle       = host is connected, no client route is active
connecting = one client route is reserved while the E2E handshake is in progress
active     = one client route is admitted for the current command
closing    = route is being removed after command completion, disconnect, timeout, or failure
```

Admission rules:

- Only one client can be in `connecting` or `active` for a given `sessionId`.
- `connecting` must have a short timeout so a failed or malicious client cannot occupy the slot indefinitely.
- The relay validates only routing material. The host decides cryptographic admission by verifying the Noise handshake and PSK.
- On handshake failure, timeout, or disconnect, the relay returns the per-session route to `idle` if the host is still connected.
- The host must independently reject concurrent handshakes or commands even if the relay admission state has a bug.

## Encrypted Message Framing

The relay should only need to route opaque frames. It should not see whether a frame is a command, stdout, stderr, exit status, error, or keepalive.

The exact binary frame format should be finalized by a small required framing spike before implementation. Use the candidate rules below as the recommended direction: simple to review in Go, explicit size limits, one encrypted frame per WebSocket message where practical, and enough structure for future stdin support without adding PTY to v1.

Outer relay frame, conceptually:

```text
sessionId or connection route id
uint32 ciphertextLength
bytes noiseCiphertext
```

Inner encrypted frame, conceptually:

```text
version
frameType
streamId
seq
payloadLength
payload
```

Suggested encrypted frame types:

```text
helloMetadata
commandRequest
stdinData
stdinEof
stdoutData
stderrData
exitStatus
error
windowUpdate
keepalive
close
```

For the first major version, run only one command at a time. Still include `commandId`, sequence numbers, and stream identifiers so stdout/stderr ordering and future multiplexing do not require a protocol rewrite. Explicit cancellation is deferred to a later version.

Candidate v1 framing rules to validate in the framing spike:

- Use one Noise transport message per encrypted inner frame.
- Define maximum outer ciphertext size and maximum inner plaintext size.
- Use direction-specific monotonically increasing sequence numbers.
- Treat missing, duplicate, or out-of-order sequence numbers as `ProtocolError` unless the chosen transport layer already makes this impossible and the invariant is documented.
- Define terminal ordering: after `exitStatus` or terminal `error`, no more stdout/stderr frames are valid for that `commandId`.
- Define stdout/stderr chunk size, for example 16-64 KiB plaintext per frame.
- Define exact behavior when `maxOutputBytes` is reached: truncate, terminate the command if necessary, send a terminal encrypted status/error, and make the local host log readable.
- Keep WebSocket boundaries and protocol frame boundaries simple, ideally one encrypted frame per WebSocket message.
- Make `commandId` random with at least 128 bits of entropy or an equivalent collision-resistant construction.

Command request payload should prefer structured execution fields over shell-only text where possible:

```json
{
  "commandId": "cmd_9R4M",
  "command": "docker ps",
  "cwd": "/srv/app",
  "timeoutSeconds": 30,
  "maxOutputBytes": 10485760,
  "pty": false
}
```

First major version can keep the UX as `-- '<command>'`, but the encrypted payload should be ready for later `argv`, environment allowlists, and stdin support.

## `/cli` Distribution Security Research

A `curl | sh` flow is convenient, but it cannot be made fully trustworthy against compromise of the `/cli` response itself because the shell script executes before it can verify anything. OpenTunnel should present `/cli` as a small auditable bootstrapper, not as a perfect supply-chain boundary.

Because the relay serves the monobinary that also implements the relay, a user running `curl | sh` is explicitly trusting that relay origin to provide executable code. E2E encryption protects command traffic from an honest-but-curious relay that routes packets; it does not protect against a relay, CDN, TLS termination point, or self-hosted origin serving malicious endpoint code.

Recommended v1 distribution model:

- `/cli` returns a short, deterministic POSIX `sh` bootstrapper.
- The script is portable and avoids Bash-only features, `eval`, predictable public temp paths, and unquoted variables.
- The script passes user arguments exactly to the downloaded binary with `exec "$bin" "$@"`.
- The script creates a private temp/runtime cache with `umask 077` and `chmod 700`.
- The first version should prioritize a top-notch, low-friction UX with no additional host-side requirements beyond `curl` and `sh`.
- The relay may serve unsigned CLI artifacts in v1 to avoid requiring users to install `minisign`, package managers, or other verification tooling on the host.
- The bootstrapper should still verify SHA-256 hashes served by the relay when practical, but same-origin checksums are only corruption detection, not a supply-chain security boundary.
- Signed manifests and stronger artifact verification remain desirable for later public-hardening, but they must not add friction to the host-side first-version command.
- Do not expose unsigned mode as a normal user-facing flag, and do not require users to set `OPENTUNNEL_ALLOW_UNSIGNED=1` for the main v1 UX.

Recommended artifact endpoints:

```text
GET /cli
GET /cli/v1
GET /cli/v1.2.3
GET /cli/v1/manifest.json
GET /cli/v1/manifest.json.minisig
GET /cli/bin/opentunnel-v1.2.3-linux-amd64
GET /cli/bin/opentunnel-v1.2.3-linux-arm64
GET /cli/bin/opentunnel-v1.2.3-darwin-amd64
GET /cli/bin/opentunnel-v1.2.3-darwin-arm64
```

Artifact versioning requirements:

- Avoid host/client mismatches during live sessions. If the host starts with CLI version N, the generated prompt must not accidentally fetch an incompatible client version N+1 from a floating `/cli` endpoint.
- Prefer immutable artifact URLs for actual binary downloads.
- The generated prompt may use a stable `/cli` URL only if the bootstrapper resolves to a session-compatible binary version.
- If `/cli` can float, include a version selector such as `/cli?v=1.2.3` in generated prompts.
- Host and client must negotiate or verify protocol compatibility and fail clearly with `ProtocolError` if incompatible.

Recommended signing/public-hardening direction:

- For v1, keep the host-side UX frictionless and avoid additional host requirements such as `minisign`.
- Do not claim that same-origin checksums provide supply-chain security; they only detect corruption or incomplete downloads.
- Later public-hardening can add signed manifests, Minisign/Ed25519 verification, Cosign/Sigstore attestations, published signing keys, and manifest freshness fields such as `created`, `expires`, `version`, `channel`, and optionally `sequence`.
- If signed verification is added later, preserve the same top-level user command shape and keep any extra verification tooling bundled or handled by the downloaded monobinary/bootstrapper path rather than requiring manual host setup.

Checksums served from the same origin as the binary are useful for corruption detection, but they are not supply-chain security if the relay or CDN is compromised. Do not imply otherwise in docs or prompts.

Temporary cache guidance:

```text
$XDG_RUNTIME_DIR/opentunnel-cli/<relayHash>/<sessionId>/<version>/opentunnel
$TMPDIR/opentunnel-cli/<relayHash>/<sessionId>/<version>/opentunnel
```

Avoid predictable shared paths like `/tmp/opentunnel`. Handle `/tmp` mounted with `noexec` by falling back to another private runtime/cache directory or printing a clear error.

Cache safety requirements:

- Use `umask 077` and create private parent directories with mode `0700`.
- Include normalized relay origin, platform, CLI version, and binary checksum in the cache key.
- Download to a unique temporary file inside the private directory, verify it, `chmod 700`, then atomically rename it into place.
- Before executing a cached binary, verify it is a regular file, owned by the current uid, not world-writable, and still matches the expected checksum.
- Never reuse a cached binary when the expected checksum or protocol-compatible version changes.

## Research References

Primary references to use during implementation:

- Noise Protocol Framework: https://noiseprotocol.org/noise.html
- Noise spec source: https://github.com/noiseprotocol/noise_spec/blob/master/noise.md
- X25519/X448, RFC 7748: https://www.rfc-editor.org/rfc/rfc7748
- ChaCha20-Poly1305, RFC 8439: https://www.rfc-editor.org/rfc/rfc8439
- SSH architecture, RFC 4251: https://www.rfc-editor.org/rfc/rfc4251
- SSH transport, RFC 4253: https://www.rfc-editor.org/rfc/rfc4253
- SSH connection protocol, RFC 4254: https://www.rfc-editor.org/rfc/rfc4254
- WebRTC data channels, RFC 8831: https://www.rfc-editor.org/rfc/rfc8831
- SPAKE2, RFC 9383: https://www.rfc-editor.org/rfc/rfc9383
- OPAQUE, RFC 9380: https://www.rfc-editor.org/rfc/rfc9380
- libsodium authenticated encryption: https://doc.libsodium.org/public-key_cryptography/authenticated_encryption
- libsodium sealed boxes: https://doc.libsodium.org/public-key_cryptography/sealed_boxes
- age: https://age-encryption.org/v1
- Magic Wormhole docs: https://github.com/magic-wormhole/magic-wormhole/tree/master/docs
- Minisign: https://jedisct1.github.io/minisign/
- Cosign/Sigstore: https://docs.sigstore.dev/cosign/
- SLSA: https://slsa.dev/spec/v1.0/
- tmate: https://tmate.io/
- Cloudflare Tunnel: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/
- Tailscale SSH: https://tailscale.com/kb/1193/tailscale-ssh
- Tailscale DERP relays: https://tailscale.com/kb/1232/derp-servers
- ngrok SSH guide: https://ngrok.com/docs/using-ngrok-with/ssh/
- Microsoft Dev Tunnels: https://learn.microsoft.com/en-us/azure/developer/dev-tunnels/overview

## Suggested First Major Version Milestones

1. Run a focused Go Noise spike with `github.com/flynn/noise` against the required v1 handshake, PSK, canonical prologue-binding, host session-key verification, and encrypted-frame requirements. Compare `NKpsk0` and `XXpsk3` and document the chosen tradeoff.
2. Build the Go monobinary skeleton with `opentunnel relay`, `opentunnel create`, and `opentunnel exec` role entry points.
3. Build relay with `/cli` placeholder, in-memory host/client routing, and atomic one-client admission.
4. Build local host/client command execution and streaming without E2E only as a short validation step.
5. Add Noise-based E2E handshake and encrypted frame transport.
6. Move host metadata into encrypted `helloMetadata`; keep relay-visible state limited to routing.
7. Add `/cli` installer script, frictionless unsigned v1 artifact serving, same-origin checksum verification where practical, and temp/runtime binary caching.
8. Add generated agent prompt with explicit bearer-invite-code and no-approval warning.
9. Add command timeout, idle session timeout, process-group cleanup, output truncation, command IDs, stream sequence numbers, and remote exit-code propagation.
10. Add replay/session lifecycle tests: reusable bearer invite semantics, duplicate client rejection, handshake timeout, host disconnect, relay restart, duplicate `commandId`, idle timeout, and relay liveness cleanup.
11. Package the monobinary for relay deployment and temporary host/client use.

## First Major Version Acceptance Criteria

The first major version is acceptable when:

1. A user can run:

```bash
curl -fsSL https://relay.example/cli | sh -s -- create
```

2. The command prints an agent-ready prompt.
3. The prompt contains an `exec` command with the invite code.
4. A client can run the generated command and execute `hostname && uname -a && pwd` on the host.
5. The relay persists no runtime state.
6. The relay cannot read command payloads, command output, final status, host metadata, or client identity.
7. Command output streams to the client as it is produced, similar to a local tool call.
8. `Ctrl+C` on the host-side `create` process closes the tunnel and terminates any active command best-effort using process-group cleanup where supported.
9. A second client is rejected while one client is connected.
10. Repeated client commands reuse the temporary binary cache during the session.
11. Client and host establish `Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s`, `Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s`, or an explicitly chosen equivalent before any command payload is sent; the chosen pattern is documented after the spike.
12. The client verifies the host session public key against `hostPubKey` from the invite code.
13. Host metadata is visible only locally or inside encrypted frames, not as relay plaintext.
14. `/cli` serves the matching temporary monobinary with a frictionless UX and no additional host requirements beyond `curl` and `sh`; same-origin checksums may be used for corruption detection but are not described as a supply-chain security boundary.
15. The host-side runtime log is structured and readable: one event per line, stable `event=<name>` values, `camelCase` fields, and local-only command visibility.
16. If no authenticated command activity happens before the fixed idle timeout, the tunnel closes without persistent relay state.
17. The generated prompt warns that the reusable invite is bearer-secret material and mentions the safer encoded-command path for complex quoting.

## Non-Goals For the first major version

Do not build initially:

- accounts or login,
- OAuth,
- multiple clients,
- persistent relay state,
- relay-side command logs,
- dashboard,
- daemon/background mode,
- `opentunnel stop`, `opentunnel list`, `opentunnel logs`,
- MCP integration,
- raw SSH compatibility,
- approval workflows,
- policy profiles,
- file upload/download,
- background process management,
- named agents,
- first-client pinning or single-use continuation tokens.

## Future Features

Potential later additions after the first major version:

- explicit approval mode for sensitive use cases,
- multi-client sessions,
- MCP integration,
- daemon/background mode,
- persistent installed CLI option,
- teams/accounts,
- dashboard,
- local host-side audit log,
- Prometheus metrics as an operator-only relay feature, preferably on a separate localhost-bound listener,
- richer command safety warnings,
- file upload/download,
- background process support,
- concurrent command execution,
- explicit command cancellation through an encrypted `cancel` frame,
- PTY support for interactive command sessions,
- named agents,
- first-client pinning or single-use continuation tokens if reusable invite leakage becomes a practical concern,
- raw SSH compatibility as an alternative transport.

## First Major Version Product Decisions

These are intended product decisions, not prototype shortcuts:

- The relay persists no runtime state at all.
- The relay is application-layer blind. It only routes opaque encrypted packets and must not receive plaintext host metadata, client identity, commands, output, or command status. It still observes routing, timing, size, and network metadata.
- The host-side `create` process is the lifecycle boundary. When it exits, the tunnel is gone.
- The live host process plus a fixed idle session timeout is the validity window. Do not require `expiresAt` in the first major version invite payload. The idle timeout resets on authenticated command activity and must not create persistent relay state.
- The first major version supports one active client connection and one active command at a time per tunnel. Multi-client support, concurrent command execution, and explicit command cancellation are deliberately deferred.
- The client-side binary cache is automatic and hidden. Users should not see a `--cache-session` flag.
- Both host and client use temporary binaries served from `/cli`.
- `/cli` is served by the relay itself so hosted and self-hosted deployments are self-contained.
- `camelCase` is used for all JSON/API/payload attributes.
- Command output streams back to the client as it is produced, and the client exits with the remote command's exit code.
- A live foreground host tunnel may serve repeated sequential `exec` commands until the host-side `create` process exits or the idle timeout fires. The reusable bearer invite is an accepted v1 tradeoff; first-client pinning is deferred.
- Only one command may run at a time in the first major version; concurrent command attempts should fail with `CommandAlreadyRunningError`.
- V1 commands run through the same kind of non-interactive shell used by the host user who created the tunnel. Each command should run in its own process group/session where supported, with graceful then forced cleanup on timeout, disconnect, idle shutdown, or host Ctrl+C.
- V1 uses a fixed, non-configurable command timeout and a combined stdout/stderr output limit.
- V1 does not support PTY execution, but PTY support is desirable in a later version.
- Error types use semantic `PascalCase` names suffixed with `Error`.
- OpenTunnel v1 should be implemented as a single Go monobinary named `opentunnel`, capable of running as relay, host-side `create`, and client-side `exec`.
- The Go monobinary direction is chosen for reviewability, simple distribution, self-hosting, and shared protocol code.
- The Go Noise library ecosystem is a known risk; v1 uses `github.com/flynn/noise` as the intended implementation, validated by a focused first engineering milestone before building the full product.
- The recommended first protocol is `Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s`, pending the required Go Noise spike; `Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s` is acceptable if library support is cleaner and the tradeoff is documented.
- Invite-code possession via a 256-bit `clientSecret` is the client authentication primitive; do not invent client accounts or static client identity for v1. The host session key pair is generated fresh per `create` process and kept only in memory.
- `/cli` may serve unsigned artifacts in v1 to keep the UX frictionless and avoid additional host requirements. Same-origin checksums alone are not a security boundary; signed manifests are a later public-hardening direction, not a v1 host requirement.
- Relay cleanup stays simple: when the host closes the tunnel, disconnects, the relay restarts, or the relay is unreachable, in-memory connection state is removed and the tunnel is gone.
- The relay may continuously log compact aggregate status to stdout, such as active host sessions and active client connections, but v1 should not expose Prometheus metrics or monitoring endpoints.
- The first major version has one internal permission object stored in the invite-code payload: `permission.mode = "yolo"`. It is the default and only valid value. It means command execution without per-command approval while the foreground host session is alive. Approval workflows are deferred unless the product direction changes.

## Resolved Questions For The Next Agent

Resolved by research and product decisions:

- Use a named Noise protocol pattern for E2E transport. Prefer `Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s` if the Go spike is clean; use `Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s` only if library support and reviewability are better, and document the tradeoff.
- Keep `/cli` frictionless in v1 and avoid additional host-side signing requirements. Same-origin checksums can be used for corruption detection but are not a supply-chain security boundary; signed manifests are a later public-hardening direction.
- Keep host metadata out of relay-visible plaintext and move it into encrypted `helloMetadata`.
- Allow repeated sequential `exec` commands while the foreground host-side `create` process remains alive and before the idle timeout fires. The invite is a reusable bearer capability in v1; first-client pinning is deferred.
- Allow only one command at a time in v1, and mention this in the generated host prompt. Concurrent command execution is a future feature.
- Use a fixed, non-configurable v1 command timeout of 120 seconds.
- Use a combined stdout/stderr output limit of 10 MiB in v1.
- Run v1 commands through the same kind of non-interactive shell used by the host user who created the tunnel.
- Keep `pty: false` in v1, while treating PTY support as a likely later feature.
- Use semantic `PascalCase` error types suffixed with `Error`.
- Implement v1 as a single Go monobinary named `opentunnel` with relay, create, and exec roles.
- Use `github.com/flynn/noise` as the intended Go Noise implementation for v1, validated by the first milestone spike.
- Keep `/cli` frictionless for v1: no additional host-side signing or verification tools are required beyond `curl` and `sh`; same-origin checksums are corruption detection only.
- Keep relay cleanup simple: host close/disconnect, idle timeout, relay restart, or unreachable relay ends the tunnel and clears in-memory state.
- Use simple aggregate stdout status logging for relay operator visibility; defer Prometheus metrics to a later operator-only feature.

## Still Open Design Details

These are intentionally not fully settled yet:

- Exact encrypted binary frame format, size limits, and sequencing rules. The spec includes the recommended direction, but a short framing spike should validate it before implementation.
- Final CLI argument names for safe complex command input. Current candidates are `--command-base64` and `--command-file -`; prefer clear agent-readable names over clever abbreviations.


