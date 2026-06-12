# Hosted Relay on Cloudflare Containers

This directory deploys the official relay to `relay.opentunnel.sh` as a
Cloudflare Container behind a Worker.

## How It Works

- `Dockerfile` wraps the released `ghcr.io/akoenig/opentunnel` image and bakes
  in the relay command with `--public-url https://relay.opentunnel.sh`.
- `src/index.js` defines the `RelayContainer` class and routes every request
  to one named instance. This is deliberate: tunnel sessions live in the
  relay's memory, so hosts and clients of the same tunnel must reach the same
  process. `max_instances` is pinned to 1 in `wrangler.jsonc`.
- The custom domain `relay.opentunnel.sh` is provisioned from the route in
  `wrangler.jsonc` on first deploy.

## Deploying

Run the manual **Deploy Relay** workflow on GitHub Actions and choose the
image tag (prefer immutable release versions over `latest`). The workflow
logs in to GHCR with the repository token, pins the chosen version into the
Dockerfile, builds the wrapper image, and pushes it through
`wrangler deploy`.

Local deployment works the same way with an authenticated wrangler session:

```bash
docker login ghcr.io
pnpm install
pnpm deploy
```

## Operational Notes

- Requires the Workers Paid plan (Cloudflare Containers) and an API token
  that can edit Workers, containers, and the `opentunnel.sh` zone.
- The container only sleeps when the relay is truly idle: when the
  `sleepAfter` timer expires, the Worker probes the relay's private health
  listener (port 8081, reachable only from the Worker, never from the
  public internet) and refuses to stop the container while tunnels are
  active. An idle relay scales to zero, and the next request wakes it
  within seconds.
- The relay logs `relay: active tunnels: N` every five minutes; view it with
  `pnpx wrangler tail opentunnel-relay` or in the Cloudflare dashboard.
