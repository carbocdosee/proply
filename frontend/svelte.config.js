import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),

	kit: {
		// Node adapter for Railway deployment
		adapter: adapter({
			out: 'build',
			precompress: true
		}),

		alias: {
			$lib: './src/lib',
			$components: './src/lib/components',
			$stores: './src/lib/stores'
		}
	}
};

export default config;
