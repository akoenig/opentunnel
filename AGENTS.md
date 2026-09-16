## Project Context

- This repository contains OpenTunnel v2: two POSIX-invocable bash scripts and a pinned build of [tailcat](https://github.com/tailscale/tailcat). There is no Go module; the 1.x Go relay and CLI live on the `1.x` branch.
- `scripts/host.sh` is served at `https://beta.opentunnel.sh`, `scripts/agent.sh` at `https://beta.opentunnel.sh/agent`, and `scripts/ot-exec.sh` is the ForceCommand wrapper embedded into the host script at build time.
- `build/` builds the tailcat binaries from `build/TAILCAT_COMMIT` and embeds version, base URL, checksums, and the wrapper into `dist/`.
- `site/` is the Cloudflare Worker that serves `dist/` at beta.opentunnel.sh. `website/` is the public site at https://opentunnel.sh; see "Website And Documentation".
- `PLAN.md` records the v2 design and the decisions behind it.
- The project is licensed under MIT (`LICENSE`). Redistributed tailcat binaries ship with `LICENSE.tailcat` (BSD-3-Clause).

## Shell Style

- Target bash 3.2 (macOS): no associative arrays, no `mapfile`, no `${var,,}`, no `declare -n`.
- Use only tools that exist on both Linux and macOS: no GNU-only `timeout`, `stat -c`, or `sed -i` without a suffix.
- `set -euo pipefail` in every script; tabs for indentation; functions prefixed `ot_` in the served scripts.
- The served scripts are wrapped in a POSIX shim by `build/embed.sh`, so `curl ... | sh` and `curl ... | bash` both work. Keep the marker lines (`# @@VERSION@@`, `# @@BASE_URL@@`, `# @@CHECKSUMS@@`, `# @@OT_EXEC@@`) intact; the build fails if one goes missing.
- Status output goes to stderr with the `[opentunnel]` prefix. Only the prompt (host) and the `remote helper: <path>` line (agent) go to stdout, because an agent parses them.
- All state lives in one `mktemp -d` directory per process, `chmod 700`, removed by the cleanup trap.

## Testing And Verification

- `shellcheck -s bash scripts/*.sh build/*.sh test/e2e.sh`
- `bats test/unit`
- `OPENTUNNEL_SKIP_CHECKSUMS=1 build/embed.sh && bash -n dist/host.sh && bash -n dist/agent.sh`
- `build/build-tailcat.sh && build/embed.sh && test/e2e.sh` when network access is available. The end-to-end test runs a real host session and a real agent install on one machine over the public DERP relays.
- Unit tests source `scripts/host.sh` and `scripts/agent.sh` with `OPENTUNNEL_SOURCE_ONLY=1`; keep that guard around `main "$@"`.

## OpenTunnel Design Constraints

- One host process, one claim, one pinned client. Phase 1 accepts exactly one connection to collect the agent's node key; phase 2 restarts with `--allow=<that key>`. A malformed or foreign claim ends the session; the host never reopens.
- Keys are ephemeral and live only in the temp directory. The tunnel address is a bearer secret until the claim, and useless afterwards.
- Lifetime is enforced outside tailcat: claim window, idle timeout, optional hard limit, Ctrl+C. tailcat itself has no timeouts.
- The agent runs every command through one shared SSH connection (`ControlMaster`) over a single tailcat client. A second tailcat client process using the same node key fights the first for the connection, so never spawn one per command.
- Commands run through the ForceCommand wrapper: no interactive shells, session markers for the supervisor, one audit line per command (command lines only, never payloads).
- Concurrent commands and file transfer are in scope. Still excluded without explicit approval: accounts, dashboards, package-manager distribution, install-to-system flows, daemon mode, PTY, multiple clients for one tunnel, approval workflows, MCP, persistent state of any kind.

## Dependency And Artifact Hygiene

- `dist/` is a build output; never commit it.
- Upgrading tailcat means changing `build/TAILCAT_COMMIT` and re-running the end-to-end test, not vendoring code.
- Keep the scripts dependency free: curl, ssh, and the shell are the only runtime requirements.

## Website And Documentation

- `website/` is an Astro Starlight site with the `lucode-starlight` theme, served at https://opentunnel.sh as Cloudflare Workers static assets (`wrangler.jsonc`, `worker/index.js`).
- Use `pnpm` and `pnpx` for all JavaScript tooling; never `npm` or `npx`.
- During the v2 beta the website still describes the 1.x flow at the apex domain. The v2 copy is prepared on `main` and goes live when v2 is generally available; at that point `website/worker/index.js` switches `/` and `/cli` to the v2 scripts and adds `/agent`.
- Fonts (Geist, Geist Mono) are embedded as base64 data URIs in `website/src/styles/fonts.css` so text can never paint in a fallback font and swap. Do not reintroduce asynchronous font loading, preloads, or external font files.
- Keep font ligatures disabled in code contexts; Geist Mono otherwise fuses `--` into a single long dash.
- Verify website changes with `pnpm build`; judge visuals with `pnpm preview:worker`, not the dev server (Vite injects styles late in dev and misrepresents fonts and layout).
- After Starlight or Expressive Code config changes, stale page HTML can reference outdated hashed assets; clear with `rm -rf node_modules/.astro .astro` and rebuild.
- Pushes to `main` touching `website/**` deploy automatically via `.github/workflows/deploy-website.yml` (requires the `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` repository secrets).
- The website is the canonical home of public docs; keep `README.md` consistent with it when messaging or command shapes change.

## Writing Style

- The quality bar for all public-facing copy is premium; write short, confident, declarative sentences without marketing fluff.
- Never use em dashes; rephrase with commas, colons, periods, or parentheses.
- Say "remote machine", never "target machine".
- Avoid morbid wording such as "dies", "dead", or "killed"; prefer "expires" or "ends" (for example, "the session ends with the last command").
- Core positioning: an agent makes tool calls on remote machines as if it executed them locally; ephemeral access and end-to-end encryption are the supporting safety story.

## Commit Conventions

- Write commit subjects in the conventional style used by the history (`feat:`, `fix:`, `chore:`, `docs:`, `ci:`, with optional scopes like `feat(website):`).
- Do not add AI attribution to commits: no "Generated with" lines and no `Co-Authored-By` trailers. All commits are authored by the repository owner. I don't like cheap advertising and also, code is also partially written by humans, so the AI contribution feels off.
