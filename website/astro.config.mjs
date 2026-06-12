// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import lucode from 'lucode-starlight';

const AVATAR = 'https://avatars.githubusercontent.com/u/224910?v=4';
const GITHUB_ICON =
	'<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"/></svg>';
const X_ICON =
	'<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M18.901 1.153h3.68l-8.04 9.19L24 22.846h-7.406l-5.8-7.584-6.638 7.584H.474l8.6-9.83L0 1.154h7.594l5.243 6.932ZM17.61 20.644h2.039L6.486 3.24H4.298Z"/></svg>';
const GLOBE_ICON =
	'<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/></svg>';
const MAIL_ICON =
	'<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect width="20" height="16" x="2" y="4" rx="2"/><path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/></svg>';
const FOOTER_TEXT =
	`Built by <span class="ot-hovercard"><a class="ot-hc-trigger" href="https://andrekoenig.com"><img src="${AVATAR}" alt="" /> André König</a><span class="ot-hc-pop">` +
	`<span class="ot-hc-head"><img src="${AVATAR}" alt="" /><span class="ot-hc-id"><span class="ot-hc-name">André König</span></span></span>` +
	`<span class="ot-hc-links"><a href="https://andrekoenig.com">${GLOBE_ICON}andrekoenig.com</a><a href="mailto:hi@andrekoenig.com">${MAIL_ICON}hi@andrekoenig.com</a><a href="https://github.com/akoenig">${GITHUB_ICON}akoenig</a><a href="https://x.com/ItsAndreKoenig">${X_ICON}ItsAndreKoenig</a></span>` +
	`</span></span><span class="ot-ft-sep">·</span>Source on [GitHub](https://github.com/akoenig/opentunnel)`;

// https://astro.build/config
export default defineConfig({
	site: 'https://opentunnel.sh',
	vite: {
		server: {
			allowedHosts: ['.trycloudflare.com'],
		},
		build: {
			// Inline the woff2 fonts into the stylesheet (see src/styles/fonts.css).
			assetsInlineLimit: 40960,
		},
	},
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
			customCss: ['./src/styles/fonts.css', './src/styles/custom.css'],
			plugins: [
				lucode({
					navLinks: [
						{ label: 'Docs', link: '/getting-started/' },
						{ label: 'Security Model', link: '/concepts/security-model/' },
						{ label: 'Self-Hosting', link: '/guides/self-hosting/' },
					],
					docs: { includeAiUtilities: true },
					footerText: FOOTER_TEXT,
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
