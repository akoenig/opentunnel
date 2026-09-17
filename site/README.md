# beta.opentunnel.sh

The Cloudflare Worker that serves the v2 scripts and the pinned tailcat binaries.

| Route | Serves |
|---|---|
| `/` | `host.sh` |
| `/agent` | `agent.sh` |
| `/bin/<version>/…` | `tailcat_<os>_<arch>`, `SHA256SUMS`, `LICENSE.tailcat` |

Scripts are served as `text/plain` with `Cache-Control: no-cache`. Released binaries are immutable, addressed by version. Development binaries under `/bin/dev/` are served uncached, because every push to `main` replaces them in place. Everything else is a 404; the worker never proxies and never redirects to another host.

## Deploying

`.github/workflows/deploy-beta.yml` deploys every push to `main` as version `dev`, and `.github/workflows/release.yml` deploys tagged releases. By hand:

```bash
build/build-tailcat.sh        # from the repository root, needs Go
build/embed.sh
rm -rf site/dist && mkdir -p site/dist
cp dist/host.sh dist/agent.sh site/dist/
cp -R dist/bin site/dist/bin
cd site && pnpm install && pnpm deploy
```

A deploy replaces the whole asset set, so `site/dist/bin` has to contain the binary directories of any earlier release whose prompts might still be in use.

## Locally

```bash
cd site && pnpm install && pnpm dev
```

Point the scripts at it with `OPENTUNNEL_BASE_URL=http://127.0.0.1:8787 OPENTUNNEL_ALLOW_HTTP=1`.
