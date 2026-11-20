import adapter from '@sveltejs/adapter-static';
import { fileURLToPath, URL } from 'node:url';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import wails from '@wailsio/runtime/plugins/vite';

// Wails webview = SPA: static build with an index.html fallback.
// (Kit 3: configuration lives on the vite plugin, not svelte.config.js.)
export default defineConfig({
	server: {
		host: '127.0.0.1',
		port: Number(process.env.WAILS_VITE_PORT) || 9245,
		strictPort: true,
	},
	resolve: {
		alias: {
			'#lib': fileURLToPath(new URL('./src/lib', import.meta.url)),
		},
	},
	plugins: [
		sveltekit({
			adapter: adapter({ pages: 'dist', assets: 'dist', fallback: 'index.html' }),
		}),
		wails('./bindings'),
	],
});
