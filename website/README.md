# opentunnel.sh

The OpenTunnel website and documentation, served at https://opentunnel.sh.

Built with [Astro Starlight](https://starlight.astro.build) and the
[lucode-starlight](https://github.com/lucas-labs/lucode-starlight-theme) theme,
deployed to Cloudflare Workers as static assets.

## Development

```bash
pnpm install
pnpm dev
```

## Build And Preview

```bash
pnpm build            # static build into dist/
pnpm preview:worker   # build + run the real Worker locally (wrangler dev)
```

`preview:worker` exercises the production setup, including the `/cli` route.

## Deployment

```bash
pnpm deploy           # astro build && wrangler deploy
```

Deploys the Worker `opentunnel-website` with `dist/` as static assets and a
custom domain route for `opentunnel.sh` (see `wrangler.jsonc`). Requires an
authenticated wrangler session (`pnpx wrangler login`) or `CLOUDFLARE_API_TOKEN`.

## The `/cli` Route

`https://opentunnel.sh/cli` responds with an HTTP 308 redirect to
`https://relay.opentunnel.sh/cli` (see `worker/index.js`). It must stay a
redirect (never a proxy) because the relay bakes its `--public-url` into the
served bootstrapper, and binary downloads plus checksum verification are
same-origin against the relay.

## Content Source

The pages under `src/content/docs/` were adapted from `../docs/public-v1/`.
Until those files are consolidated, changes to the public docs should be
mirrored here.
