import { defineConfig, devices } from '@playwright/test';

/**
 * E2E test configuration.
 * Tests live in frontend/e2e/ and run against the full running stack
 * (SvelteKit frontend + Go backend + PostgreSQL).
 *
 * Run: npx playwright test
 * UI:  npx playwright test --ui
 */
export default defineConfig({
	// E2E tests are in a dedicated directory, separate from unit/integration tests
	testDir: './e2e',

	// Fail fast in CI
	forbidOnly: !!process.env.CI,

	// Retry once on failure in CI
	retries: process.env.CI ? 1 : 0,

	// Parallelism — limit to 1 worker to avoid auth/DB race conditions
	workers: 1,

	reporter: process.env.CI ? 'github' : 'list',

	use: {
		// Base URL — override with PLAYWRIGHT_BASE_URL env var
		baseURL: process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:5173',

		// Collect trace on first retry only
		trace: 'on-first-retry',

		// Take screenshot on failure
		screenshot: 'only-on-failure',
	},

	projects: [
		{
			name: 'chromium',
			use: { ...devices['Desktop Chrome'] },
		},
	],

	// Auto-start the dev server if PLAYWRIGHT_BASE_URL is not set externally.
	// Comment out webServer when running against a deployed staging environment.
	webServer: process.env.PLAYWRIGHT_BASE_URL
		? undefined
		: {
				command: 'npm run dev',
				url: 'http://localhost:5173',
				reuseExistingServer: !process.env.CI,
				timeout: 30_000,
			},
});
