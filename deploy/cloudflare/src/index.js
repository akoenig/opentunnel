import { Container, getContainer } from '@cloudflare/containers';

export class RelayContainer extends Container {
	defaultPort = 8080;
	// The relay keeps tunnel state in memory only; sleeping ends sessions,
	// which the product tolerates (hosts reconnect with a fresh session).
	sleepAfter = '1h';
}

export default {
	async fetch(request, env) {
		// One global instance on purpose: hosts and clients of the same tunnel
		// must reach the same relay process because sessions live in memory.
		return getContainer(env.RELAY, 'relay').fetch(request);
	},
};
