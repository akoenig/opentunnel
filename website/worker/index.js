// opentunnel.sh worker: static site plus the tunnel-creation conveniences.
//
// Three doors, one script:
//   /cli      308 redirect to the relay bootstrapper (full command surface).
//   /create   serves CREATE_SCRIPT, which fetches the relay bootstrapper and
//             runs `create`. The documented way to start a session.
//   /         serves CREATE_SCRIPT to curl/wget user agents, the website to
//             everyone else. `curl -fsSL opentunnel.sh | sh` just works.
//
// All of them must stay redirects or thin wrappers, never proxies: the
// bootstrapper served by the relay bakes in the relay's --public-url as the
// origin for binary downloads, same-origin checksum verification, and the
// tunnel itself.

const RELAY_CLI_URL = 'https://relay.opentunnel.sh/cli';

const CREATE_SCRIPT = `#!/bin/sh
#
#   OpenTunnel
#   Your agent's tool calls, on any machine.
#   https://opentunnel.sh
#
#   This script starts an OpenTunnel host session on this machine:
#
#     1. It fetches the OpenTunnel bootstrapper from the relay
#        (${RELAY_CLI_URL}).
#     2. The bootstrapper downloads a temporary CLI for your platform,
#        verifies its checksum against the same origin, and runs
#        \`create\`. Nothing is installed.
#     3. The session prints a ready-made prompt for your agent and stays
#        open until you press Ctrl+C. Ctrl+C revokes all access.
#
#   Want to inspect before running? You already are: this file is the
#   whole script. Security model:
#   https://opentunnel.sh/concepts/security-model/
#

set -eu

curl -fsSL ${RELAY_CLI_URL} | sh -s -- create "$@"
`;

const createResponse = () =>
	new Response(CREATE_SCRIPT, {
		headers: {
			'content-type': 'text/x-shellscript; charset=utf-8',
			'cache-control': 'public, max-age=300',
		},
	});

const isCommandLineClient = (request) => {
	const agent = (request.headers.get('user-agent') || '').toLowerCase();
	return agent.startsWith('curl/') || agent.startsWith('wget/');
};

export default {
	async fetch(request, env) {
		const url = new URL(request.url);
		if (url.pathname === '/cli') {
			return Response.redirect(RELAY_CLI_URL, 308);
		}
		if (url.pathname === '/create') {
			return createResponse();
		}
		if (url.pathname === '/' && isCommandLineClient(request)) {
			return createResponse();
		}
		return env.ASSETS.fetch(request);
	},
};
