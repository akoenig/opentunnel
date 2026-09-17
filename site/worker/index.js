// beta.opentunnel.sh worker.
//
// Two doors and an asset store, no proxying and no redirects to other hosts:
//   /             host.sh    (curl -fsSL https://beta.opentunnel.sh | sh)
//   /agent        agent.sh   (curl -fsSL https://beta.opentunnel.sh/agent | sh -s -- <addr>)
//   /bin/<ver>/*  the pinned tailcat binaries, SHA256SUMS, LICENSE.tailcat
//
// The scripts must never be cached: a release replaces them in place. The
// binaries are immutable, addressed by version.

const SCRIPT_ROUTES = new Map([
	['/', '/host.sh'],
	['/host.sh', '/host.sh'],
	['/agent', '/agent.sh'],
	['/agent.sh', '/agent.sh'],
]);

const withHeaders = (response, headers) => {
	const next = new Response(response.body, response);
	for (const [name, value] of Object.entries(headers)) {
		next.headers.set(name, value);
	}
	return next;
};

export default {
	async fetch(request, env) {
		const url = new URL(request.url);
		const script = SCRIPT_ROUTES.get(url.pathname);

		if (script) {
			const asset = await env.ASSETS.fetch(new URL(script, url));
			if (!asset.ok) {
				return new Response('not found\n', { status: 404 });
			}
			return withHeaders(asset, {
				'content-type': 'text/plain; charset=utf-8',
				'cache-control': 'no-cache',
			});
		}

		if (url.pathname.startsWith('/bin/')) {
			const asset = await env.ASSETS.fetch(request);
			if (!asset.ok) {
				return new Response('not found\n', { status: 404 });
			}
			// A released version is addressed by its version and never changes.
			// Development builds reuse /bin/dev/ on every push to main, so that
			// path must never be cached.
			const immutable = !url.pathname.startsWith('/bin/dev/');
			return withHeaders(asset, {
				'cache-control': immutable
					? 'public, max-age=31536000, immutable'
					: 'no-cache',
			});
		}

		return new Response('not found\n', {
			status: 404,
			headers: { 'content-type': 'text/plain; charset=utf-8' },
		});
	},
};
