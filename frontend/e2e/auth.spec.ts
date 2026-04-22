/**
 * E2E auth tests — TASK-AQA-101
 *
 * Requires a fully running stack:
 *   - SvelteKit dev server: npm run dev  (port 5173)
 *   - Go backend:           make dev     (port 8080)
 *   - PostgreSQL:           running and migrated
 *
 * Run: npx playwright test
 * UI:  npx playwright test --ui
 */
import { test, expect, type Page } from '@playwright/test';

const API = process.env.VITE_API_URL ?? 'http://localhost:8080';

// Unique email per test run — avoids DB conflicts between runs.
const uid = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 6);
const freshEmail = () => `e2e+${uid()}@proply-test.io`;

// ─── Helpers ─────────────────────────────────────────────────────────────────

async function fillRegistrationForm(
	page: Page,
	opts: { email: string; password: string; name: string }
) {
	await page.fill('[data-testid="register-name"]', opts.name);
	await page.fill('[data-testid="register-email"]', opts.email);
	await page.fill('[data-testid="register-password"]', opts.password);
	await page.click('[data-testid="register-submit"]');
}

async function fillLoginForm(page: Page, opts: { email: string; password: string }) {
	await page.fill('[data-testid="login-email"]', opts.email);
	await page.fill('[data-testid="login-password"]', opts.password);
	await page.click('[data-testid="login-submit"]');
}

/** Create a user directly via API (faster than going through the UI a second time). */
async function createUser(email: string, password: string, name = 'E2E User') {
	const res = await fetch(`${API}/api/v1/auth/register`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name, email, password })
	});
	if (!res.ok && res.status !== 409) {
		throw new Error(`createUser failed: ${res.status} ${await res.text()}`);
	}
}

// ─── AC-1: New user registration → dashboard accessible ──────────────────────

test('AC-1: new user registers and reaches dashboard', async ({ page }) => {
	const email = freshEmail();

	await page.goto('/auth/register');
	await fillRegistrationForm(page, { name: 'Test User', email, password: 'Password123!' });

	await expect(page).toHaveURL(/\/dashboard/, { timeout: 10_000 });
	await expect(page.locator('[data-testid="dashboard-root"]')).toBeVisible();
});

// ─── AC-2: Duplicate email → "Email already taken" error ─────────────────────

test('AC-2: duplicate email shows error on register page', async ({ page }) => {
	const email = freshEmail();

	// Create the account first via API
	await createUser(email, 'Password123!');

	await page.goto('/auth/register');
	await fillRegistrationForm(page, { name: 'Second', email, password: 'Different456!' });

	// Should stay on register page
	await expect(page).toHaveURL(/\/auth\/register/);
	const errorEl = page.locator('[data-testid="register-error"]');
	await expect(errorEl).toBeVisible();
	await expect(errorEl).not.toBeEmpty();
});

// ─── AC-3: Email + password login success ────────────────────────────────────

test('AC-3: valid credentials log in and reach dashboard', async ({ page }) => {
	const email = freshEmail();
	const password = 'Str0ngPass!';
	await createUser(email, password);

	await page.goto('/auth/login');
	await fillLoginForm(page, { email, password });

	await expect(page).toHaveURL(/\/dashboard/, { timeout: 10_000 });
	await expect(page.locator('[data-testid="dashboard-root"]')).toBeVisible();
});

// ─── AC-4: Wrong password → error message ────────────────────────────────────

test('AC-4: wrong password shows error on login page', async ({ page }) => {
	const email = freshEmail();
	await createUser(email, 'CorrectPw1!');

	await page.goto('/auth/login');
	await fillLoginForm(page, { email, password: 'WrongPassword!' });

	// Should stay on login page and show an error
	await expect(page).toHaveURL(/\/auth\/login/);
	const errorEl = page.locator('[data-testid="login-error"]');
	await expect(errorEl).toBeVisible();
	await expect(errorEl).not.toBeEmpty();
});

// ─── AC-5: Invalid / expired magic link → redirect with error ────────────────
//
// The backend returns the same redirect for both "not found" and "expired"
// tokens: /auth/magic-link?error=invalid_or_expired
//
// Using a random token is sufficient to validate the redirect path.
// The 15-minute expiry logic is verified at unit-test level
// (backend/internal/service/auth_test.go).

test('AC-5: invalid magic link token redirects with error', async ({ page }) => {
	const fakeToken = 'totally-invalid-token-' + uid();

	// The backend /api/v1/auth/magic-link/verify redirects to the SvelteKit app.
	// Navigate directly to the SvelteKit magic-link verify route which calls the backend.
	await page.goto(`/auth/magic-link/verify?token=${fakeToken}`);

	// Should end up on the error variant of the magic-link page
	await expect(page).toHaveURL(/error=invalid_or_expired/, { timeout: 8_000 });
});

// ─── AC-6: Google OAuth callback with invalid code → redirect with error ─────

test('AC-6: Google OAuth invalid code redirects to login with error', async ({ page }) => {
	// Set oauth_state cookie to pass CSRF validation in the backend
	const state = 'playwright-test-state';
	await page.context().addCookies([
		{
			name: 'oauth_state',
			value: state,
			domain: new URL(API).hostname,
			path: '/',
			httpOnly: true,
			secure: false,
			sameSite: 'Lax'
		}
	]);

	// Hit the backend callback directly — it will redirect back to the frontend
	await page.goto(
		`${API}/api/v1/auth/google/callback?state=${state}&code=invalid-code-playwright`
	);

	// Expect redirect to login page with an OAuth error param
	await expect(page).toHaveURL(/oauth_exchange_failed|oauth_not_configured/, { timeout: 8_000 });
});

// ─── AC-7: Unverified user sees email verification banner ─────────────────────

test('AC-7: newly registered user sees verify-email banner on dashboard', async ({ page }) => {
	const email = freshEmail();

	// Register — new users always have email_verified_at = NULL
	await page.goto('/auth/register');
	await fillRegistrationForm(page, { name: 'Unverified', email, password: 'Unverified1!' });

	await expect(page).toHaveURL(/\/dashboard/, { timeout: 10_000 });

	// The verification banner must be visible
	const banner = page.locator('[data-testid="verify-email-banner"]');
	await expect(banner).toBeVisible();
	await expect(banner).not.toBeEmpty();
});

// ─── Anti-enumeration: magic link request always returns 200 ─────────────────

test('magic link: unknown email returns 200 (anti-enumeration)', async () => {
	const res = await fetch(`${API}/api/v1/auth/magic-link`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email: 'nobody-' + uid() + '@proply-test.io' })
	});
	expect(res.status).toBe(200);
});
