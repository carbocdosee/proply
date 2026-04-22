import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		port: 5173
	},
	test: {
		// Run unit tests in a Node environment (no DOM needed for pure utils)
		environment: 'node',
		include: ['src/**/*.test.ts'],
		exclude: ['node_modules', 'e2e']
	}
});
