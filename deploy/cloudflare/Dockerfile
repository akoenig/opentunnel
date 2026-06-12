# Cloudflare Containers wrapper for the hosted relay at relay.opentunnel.sh.
# The version is pinned by the Deploy Relay workflow before `wrangler deploy`.
ARG OPENTUNNEL_VERSION=latest
FROM ghcr.io/akoenig/opentunnel:${OPENTUNNEL_VERSION}

CMD ["relay", "--listen", ":8080", "--health-listen", ":8081", "--public-url", "https://relay.opentunnel.sh"]
