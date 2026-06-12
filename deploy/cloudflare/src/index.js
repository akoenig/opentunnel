import { Container, getContainer } from '@cloudflare/containers';

export class RelayContainer extends Container {
	defaultPort = 8080;
	sleepAfter = '1h';

	// Only sleep when the relay is truly idle. Tunnel sessions live in relay
	// memory, so stopping the container would end them; the relay's private
	// health listener (port 8081, never exposed publicly because all ingress
	// flows through this Worker toward port 8080) reports the active tunnel
	// count, and we refuse to stop while it is non-zero. The probe itself
	// counts as activity and re-arms the sleep timer, so this check repeats
	// every sleepAfter interval.
	async onActivityExpired() {
		try {
			const response = await this.containerFetch('http://container/healthz', 8081);
			const body = await response.text();
			const count = Number((body.match(/\d+/) || ['0'])[0]);
			if (response.ok && count > 0) {
				return;
			}
		} catch {
			// Unreachable container: fall through and let it stop.
		}
		await this.stop();
	}
}

export default {
	async fetch(request, env) {
		// One global instance on purpose: hosts and clients of the same tunnel
		// must reach the same relay process because sessions live in memory.
		return getContainer(env.RELAY, 'relay').fetch(request);
	},
};
