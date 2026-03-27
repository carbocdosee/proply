import type { Config } from 'tailwindcss';

export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	theme: {
		extend: {
			colors: {
				// Brand colors (overridable via CSS variables for per-proposal branding)
				brand: {
					primary: 'var(--color-primary, #6366F1)',
					accent: 'var(--color-accent, #F59E0B)'
				}
			},
			fontFamily: {
				sans: ['Inter', 'system-ui', 'sans-serif']
			}
		}
	},
	plugins: []
} satisfies Config;
