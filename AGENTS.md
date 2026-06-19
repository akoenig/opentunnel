## Project Context

- This repository contains OpenTunnel, a Go 1.23 command-line and relay implementation.
- The module name is `opentunnel`.
- The entrypoint is under `cmd/opentunnel`; reusable implementation packages live under `internal/`.
- Deployment artifacts live under `deploy/`, and operational documentation lives under `docs/`.
- The public website and docs (https://opentunnel.sh) live under `website/`; see "Website And Documentation".
- The project is licensed under MIT (`LICENSE`).

## Go Style

- Prefer small, direct, idiomatic Go over clever abstractions.
- Keep package names short, lowercase, and focused on what the package provides.
- Accept interfaces at package boundaries when they make testing or substitution simpler; return concrete types by default.
- Pass `context.Context` as the first parameter for operations that can block, perform I/O, or need cancellation.
- Wrap errors with useful operation context using `%w`; do not discard errors silently.
- Avoid package-level mutable state unless it is immutable configuration or a deliberate process-wide singleton.
- Use `gofmt` formatting for all Go changes.

## Testing And Verification

- Keep tests close to the implementation using `*_test.go` files.
- Prefer table-driven tests when they make cases clearer.
- Keep tests deterministic; avoid sleeps, timing assumptions, and external network dependencies unless explicitly required.
- Run the standard verification commands after implementation changes:
  - `go test ./... -count=1`
  - `go vet ./...`
  - `go mod tidy -diff`
  - `go test -race ./... -count=1`
  - `go build ./cmd/opentunnel`
- Remove the local `./opentunnel` binary after build verification when it is produced in the repository root.

## OpenTunnel Design Constraints

- Preserve the v1 shape: one temporary host process, one client, one command at a time, and a relay-served temporary CLI.
- Treat invites as bearer-secret material.
- Keep command traffic end-to-end encrypted; the relay should route opaque packets and avoid command awareness.
- Keep relay state temporary and in-memory unless a task explicitly changes the product scope.
- Do not add accounts, dashboards, package-manager distribution, install-to-system flows, MCP, raw SSH, PTY, interactive stdin, file transfer, approval workflows, multiple clients for one tunnel, concurrent commands, persistent relay state, or persistent audit logs without explicit approval.

## Dependency And Artifact Hygiene

- Keep dependencies minimal; prefer the standard library when it is sufficient.
- Do not edit generated build artifacts or committed release outputs unless the task is specifically about those artifacts.
- Use `go mod tidy -diff` to verify module files remain tidy instead of changing `go.mod` or `go.sum` speculatively.
- Do not vendor external repositories into this project unless explicitly requested.

## Website And Documentation

- `website/` is an Astro Starlight site with the `lucode-starlight` theme, served at https://opentunnel.sh as Cloudflare Workers static assets (`wrangler.jsonc`, `worker/index.js`).
- Use `pnpm` and `pnpx` for all JavaScript tooling; never `npm` or `npx`.
- The Worker (`worker/index.js`) provides tunnel-creation routes: `/cli` is an HTTP 308 redirect to the relay, `/create` serves a thin wrapper script that fetches the relay bootstrapper and runs `create`, and `/` serves that same script to curl/wget user agents. All must stay redirects or thin wrappers, never proxies: the relay bakes `--public-url` into the served bootstrapper, and checksum verification is same-origin against the relay.
- Fonts (Geist, Geist Mono) are embedded as base64 data URIs in `website/src/styles/fonts.css` so text can never paint in a fallback font and swap. Do not reintroduce asynchronous font loading, preloads, or external font files.
- Keep font ligatures disabled in code contexts; Geist Mono otherwise fuses `--` into a single long dash.
- Verify website changes with `pnpm build`; judge visuals with `pnpm preview:worker`, not the dev server (Vite injects styles late in dev and misrepresents fonts and layout).
- After Starlight or Expressive Code config changes, stale page HTML can reference outdated hashed assets; clear with `rm -rf node_modules/.astro .astro` and rebuild.
- Pushes to `main` touching `website/**` deploy automatically via `.github/workflows/deploy-website.yml` (requires the `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` repository secrets).
- The website is the canonical home of public docs; keep `README.md` and `docs/public-v1/` consistent with it when messaging or command shapes change.

## Writing Style

- The quality bar for all public-facing copy is premium; write short, confident, declarative sentences without marketing fluff.
- Never use em dashes; rephrase with commas, colons, periods, or parentheses.
- Say "remote machine", never "target machine".
- Avoid morbid wording such as "dies", "dead", or "killed"; prefer "expires" or "ends" (for example, "the invite expires with the session").
- Core positioning: an agent makes tool calls on remote machines as if it executed them locally; ephemeral access and end-to-end encryption are the supporting safety story.

## Commit Conventions

- Write commit subjects in the conventional style used by the history (`feat:`, `fix:`, `chore:`, `docs:`, `ci:`, with optional scopes like `feat(website):`).
- Do not add AI attribution to commits: no "Generated with" lines and no `Co-Authored-By` trailers. All commits are authored by the repository owner. I don't like cheap advertising and also, code is also partially written by humans, so the AI contribution feels off.
