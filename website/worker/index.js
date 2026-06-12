// opentunnel.sh worker: static site plus the /cli convenience endpoint.
//
// /cli must redirect (never proxy) to the relay: the bootstrapper served by
// the relay bakes in the relay's --public-url as the origin for binary
// downloads, same-origin checksum verification, and the tunnel itself.
// `curl -fsSL` follows redirects, so the hop is invisible to users.

const RELAY_CLI_URL = 'https://relay.opentunnel.sh/cli';

export default {
	async fetch(request, env) {
		const url = new URL(request.url);
		if (url.pathname === '/cli') {
			return Response.redirect(RELAY_CLI_URL, 308);
		}
		return env.ASSETS.fetch(request);
	},
};
