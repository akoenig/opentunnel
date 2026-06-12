// @ts-check
import { defineConfig, fontProviders } from 'astro/config';
import starlight from '@astrojs/starlight';
import lucode from 'lucode-starlight';

// https://astro.build/config
export default defineConfig({
	site: 'https://opentunnel.sh',
	fonts: [
		{
			provider: fontProviders.fontsource(),
			name: 'Geist',
			cssVariable: '--font-geist',
			weights: ['100 900'],
			// 'optional' never swaps mid-view: Geist renders when ready within
			// ~100ms (virtually always, thanks to preload), otherwise the
			// metric-matched fallback is kept for the whole page view.
			display: 'optional',
			fallbacks: ['ui-sans-serif', 'system-ui', 'sans-serif'],
		},
		{
			provider: fontProviders.fontsource(),
			name: 'Geist Mono',
			cssVariable: '--font-geist-mono',
			weights: ['100 900'],
			display: 'optional',
			fallbacks: ['ui-monospace', 'monospace'],
		},
	],
	vite: {
		// Dev-server only: allow previewing `pnpm dev` through Cloudflare
		// quick tunnels, whose hostnames are random on every run.
		server: {
			allowedHosts: ['.trycloudflare.com'],
		},
	},
	integrations: [
		starlight({
			title: 'OpenTunnel',
			expressiveCode: {
				styleOverrides: {
					frames: {
						terminalTitlebarBackground: 'var(--ot-code-header)',
						editorTabBarBackground: 'var(--ot-code-header)',
					},
				},
			},
			description:
				"Your agent's tool calls, on any machine. OpenTunnel gives AI agents ephemeral, end-to-end encrypted command tunnels to remote machines — no SSH, no accounts, no standing access.",
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/akoenig/opentunnel' },
			],
			customCss: ['./src/styles/custom.css'],
			components: {
				Head: './src/components/Head.astro',
			},
			plugins: [
				lucode({
					navLinks: [
						{ label: 'Docs', link: '/getting-started/' },
						{ label: 'Security Model', link: '/concepts/security-model/' },
						{ label: 'Self-Hosting', link: '/guides/self-hosting/' },
					],
					docs: { includeAiUtilities: true },
					footerText:
						'Built by [André König](https://andrekoenig.com). Source on [GitHub](https://github.com/akoenig/opentunnel).',
				}),
			],
			sidebar: [
				{
					label: 'Start Here',
					items: ['getting-started'],
				},
				{
					label: 'Concepts',
					items: ['concepts/how-it-works', 'concepts/security-model'],
				},
				{
					label: 'Guides',
					items: ['guides/self-hosting', 'guides/relay-operations'],
				},
				{
					label: 'Reference',
					items: ['reference/cli', 'reference/scope-and-non-goals'],
				},
			],
		}),
	],
});
