/**
 * E2E branding settings tests — TASK-AQA-102
 *
 * Requires a fully running stack:
 *   - SvelteKit dev server: npm run dev  (port 5173)
 *   - Go backend:           make dev     (port 8080)
 *   - PostgreSQL:           running and migrated
 *
 * Run: npx playwright test e2e/settings-branding.spec.ts
 */
import { test, expect, type Page } from '@playwright/test';

const API = process.env.VITE_API_URL ?? 'http://localhost:8080';

// ─── Helpers ─────────────────────────────────────────────────────────────────

const uid = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 6);
const freshEmail = () => `branding+${uid()}@proply-test.io`;

/** Register a new user and land on /dashboard/settings. */
async function loginAndGoToSettings(page: Page, plan: 'free' | 'pro' = 'free') {
	const email = freshEmail();
	const password = 'Branding1!';

	// Register via API
	const res = await fetch(`${API}/api/v1/auth/register`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name: 'Branding Test', email, password })
	});
	if (!res.ok) throw new Error(`register failed: ${res.status}`);

	// If pro plan is needed for a test, upgrade via API (requires test endpoint or DB seed).
	// For most tests the Free plan is sufficient.

	// Log in via the UI to get a session cookie
	await page.goto('/auth/login');
	await page.fill('[data-testid="login-email"]', email);
	await page.fill('[data-testid="login-password"]', password);
	await page.click('[data-testid="login-submit"]');
	await expect(page).toHaveURL(/\/dashboard/, { timeout: 10_000 });

	await page.goto('/dashboard/settings');
	await expect(page.locator('[data-testid="save-branding-btn"]')).toBeVisible({ timeout: 8_000 });
}

/** Create an in-memory File-like buffer for Playwright's setInputFiles. */
function makeFile(name: string, sizeBytes: number, mimeType: string) {
	return {
		name,
		mimeType,
		buffer: Buffer.alloc(sizeBytes, 0x89), // fill with dummy bytes
	};
}

// ─── AC-1: Valid PNG selected → local preview appears ────────────────────────
//
// The preview is rendered immediately from a local object URL — no S3 call yet.
// Full upload + persistence (logo_url saved in DB) requires S3 and is covered
// by the integration test in account_handler_test.go.

test('AC-1: selecting valid PNG shows local preview', async ({ page }) => {
	await loginAndGoToSettings(page);

	// Before upload: no preview image
	await expect(page.locator('[data-testid="logo-preview-img"]')).not.toBeVisible();

	// Upload a 500 KB valid PNG
	await page.locator('[data-testid="logo-file-input"]').setInputFiles(
		makeFile('logo.png', 500 * 1024, 'image/png')
	);

	// Preview should appear immediately (local object URL)
	await expect(page.locator('[data-testid="logo-preview-img"]')).toBeVisible();
	// Remove button should appear alongside the preview
	await expect(page.locator('[data-testid="logo-remove-btn"]')).toBeVisible();
	// No error
	await expect(page.locator('[data-testid="logo-error"]')).not.toBeVisible();
});

// ─── AC-2: File > 2 MB → client-side "too large" error ──────────────────────
//
// The frontend validates size before any API call.
// 3 MB file → logoError = $_('settings.logo_too_large')

test('AC-2: file larger than 2 MB shows error without API call', async ({ page }) => {
	await loginAndGoToSettings(page);

	// Monitor: no API call should be made for presign
	const presignCalled = { value: false };
	page.on('request', (req) => {
		if (req.url().includes('/upload/presign')) presignCalled.value = true;
	});

	await page.locator('[data-testid="logo-file-input"]').setInputFiles(
		makeFile('large.png', 3 * 1024 * 1024, 'image/png') // 3 MB
	);

	// Error should appear
	const err = page.locator('[data-testid="logo-error"]');
	await expect(err).toBeVisible();
	await expect(err).not.toBeEmpty();

	// No preview should be shown
	await expect(page.locator('[data-testid="logo-preview-img"]')).not.toBeVisible();

	// Confirm no presign call was made
	expect(presignCalled.value).toBe(false);
});

// ─── AC-3: Wrong file type → "invalid format" error ─────────────────────────
//
// The frontend checks file.type against allowedTypes before any API call.

test('AC-3: non-image file type shows invalid format error', async ({ page }) => {
	await loginAndGoToSettings(page);

	// Simulate an .exe file with an unexpected MIME type
	await page.locator('[data-testid="logo-file-input"]').setInputFiles(
		makeFile('malware.exe', 1024, 'application/x-msdownload')
	);

	const err = page.locator('[data-testid="logo-error"]');
	await expect(err).toBeVisible();
	await expect(err).not.toBeEmpty();
	await expect(page.locator('[data-testid="logo-preview-img"]')).not.toBeVisible();
});

test('AC-3: GIF file type shows invalid format error', async ({ page }) => {
	await loginAndGoToSettings(page);

	await page.locator('[data-testid="logo-file-input"]').setInputFiles(
		makeFile('image.gif', 10 * 1024, 'image/gif')
	);

	const err = page.locator('[data-testid="logo-error"]');
	await expect(err).toBeVisible();
	await expect(page.locator('[data-testid="logo-preview-img"]')).not.toBeVisible();
});

// ─── AC-4: Invalid HEX → field reverts on blur ───────────────────────────────
//
// The onPrimaryColorBlur handler reverts primaryColorText to the last valid
// color if the regex ^#[0-9A-Fa-f]{6}$ does not match.

test('AC-4: invalid HEX in primary color input reverts on blur', async ({ page }) => {
	await loginAndGoToSettings(page);

	const input = page.locator('[data-testid="primary-color-text"]');

	// Record the current valid value
	const validColor = await input.inputValue();

	// Type an invalid hex value
	await input.fill('#ZZZZZZ');
	expect(await input.inputValue()).toBe('#ZZZZZZ');

	// Trigger blur — the value should revert
	await input.blur();
	await page.waitForTimeout(100); // allow Svelte reactivity to settle

	const reverted = await input.inputValue();
	if (reverted === '#ZZZZZZ') {
		// The field should NOT keep the invalid value
		throw new Error(`Expected input to revert from '#ZZZZZZ', but it stayed: ${reverted}`);
	}
	// The reverted value must be a valid 7-char hex
	if (!/^#[0-9A-Fa-f]{6}$/.test(reverted)) {
		throw new Error(`Reverted value is not a valid hex color: ${reverted}`);
	}
	// Confirm it reverted to the original valid value
	expect(reverted).toBe(validColor);
});

test('AC-4: partial HEX (5 chars) reverts on blur', async ({ page }) => {
	await loginAndGoToSettings(page);

	const input = page.locator('[data-testid="primary-color-text"]');
	const validColor = await input.inputValue();

	await input.fill('#12345'); // 5 chars — invalid
	await input.blur();
	await page.waitForTimeout(100);

	expect(await input.inputValue()).toBe(validColor);
});

test('AC-4: HEX without # reverts on blur', async ({ page }) => {
	await loginAndGoToSettings(page);

	const input = page.locator('[data-testid="accent-color-text"]');
	const validColor = await input.inputValue();

	await input.fill('FF0000'); // missing # — invalid
	await input.blur();
	await page.waitForTimeout(100);

	expect(await input.inputValue()).toBe(validColor);
});

// ─── AC-5: Free plan → toggle "hide footer" → paywall modal ──────────────────
//
// New users start on the Free plan. The UI intercepts the checkbox change event
// before sending to the API and shows a paywall modal instead.

test('AC-5: Free plan user clicking hide-footer toggle shows paywall modal', async ({ page }) => {
	await loginAndGoToSettings(page);

	// Paywall modal should not be visible initially
	await expect(page.locator('[data-testid="paywall-modal"]')).not.toBeVisible();

	// Click the checkbox — it's visually hidden (sr-only) so we click directly
	await page.locator('[data-testid="hide-footer-checkbox"]').click({ force: true });

	// Paywall modal must appear
	await expect(page.locator('[data-testid="paywall-modal"]')).toBeVisible({ timeout: 3_000 });

	// The checkbox itself should remain unchecked (reverted by the handler)
	const checked = await page.locator('[data-testid="hide-footer-checkbox"]').isChecked();
	expect(checked).toBe(false);
});

test('AC-5: paywall modal can be dismissed', async ({ page }) => {
	await loginAndGoToSettings(page);

	await page.locator('[data-testid="hide-footer-checkbox"]').click({ force: true });
	await expect(page.locator('[data-testid="paywall-modal"]')).toBeVisible({ timeout: 3_000 });

	// Click "Later" button (second button in the modal)
	await page.locator('[data-testid="paywall-modal"] button').nth(0).click();

	await expect(page.locator('[data-testid="paywall-modal"]')).not.toBeVisible();
});

// ─── AC-6: Integration — PATCH /api/v1/account/branding with valid data ──────
//
// Tested at the handler level in account_handler_test.go (unit) and here at
// API level (requires running backend).

test('AC-6: PATCH /api/v1/account/branding with valid colors returns 200', async () => {
	// Register and get an access token
	const email = freshEmail();
	const regRes = await fetch(`${API}/api/v1/auth/register`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name: 'Branding AC6', email, password: 'BrandAC6!' })
	});
	const { access_token } = await regRes.json();

	const patchRes = await fetch(`${API}/api/v1/account/branding`, {
		method: 'PATCH',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${access_token}`
		},
		body: JSON.stringify({
			primary_color: '#FF5733',
			accent_color: '#33C4FF'
		})
	});

	expect(patchRes.status).toBe(200);
});

test('AC-6: PATCH /api/v1/account/branding with invalid hex returns 422', async () => {
	const email = freshEmail();
	const regRes = await fetch(`${API}/api/v1/auth/register`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name: 'Color Invalid', email, password: 'ColorInv1!' })
	});
	const { access_token } = await regRes.json();

	const patchRes = await fetch(`${API}/api/v1/account/branding`, {
		method: 'PATCH',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${access_token}`
		},
		body: JSON.stringify({ primary_color: '#ZZZZZZ' })
	});

	expect(patchRes.status).toBe(422);
	const body = await patchRes.json();
	expect(body.code).toBe('INVALID_COLOR');
});

test('AC-6: PATCH /api/v1/account/branding hide_footer on Free plan returns 402', async () => {
	const email = freshEmail();
	const regRes = await fetch(`${API}/api/v1/auth/register`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name: 'Footer Free', email, password: 'FooterFr1!' })
	});
	const { access_token } = await regRes.json();

	const patchRes = await fetch(`${API}/api/v1/account/branding`, {
		method: 'PATCH',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${access_token}`
		},
		body: JSON.stringify({ hide_proply_footer: true })
	});

	expect(patchRes.status).toBe(402);
	const body = await patchRes.json();
	expect(body.code).toBe('PLAN_REQUIRED');
	expect(body.feature).toBe('hide_proply_footer');
	expect(body.min_plan).toBe('pro');
});
