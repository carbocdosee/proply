/**
 * E2E proposal template tests — TASK-AQA-103
 *
 * Requires a fully running stack:
 *   - SvelteKit dev server: npm run dev  (port 5173)
 *   - Go backend:           make dev     (port 8080)
 *   - PostgreSQL:           running and migrated
 *
 * Run: npx playwright test e2e/proposals-template.spec.ts
 */
import { test, expect, type Page } from '@playwright/test';

const API = process.env.VITE_API_URL ?? 'http://localhost:8080';

// ─── Helpers ─────────────────────────────────────────────────────────────────

const uid = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 6);
const freshEmail = () => `proposals+${uid()}@proply-test.io`;

/** Register a new user and return { email, password, access_token }. */
async function registerUser() {
	const email = freshEmail();
	const password = 'Proposals1!';
	const res = await fetch(`${API}/api/v1/auth/register`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name: 'Template Tester', email, password })
	});
	if (!res.ok) throw new Error(`register failed: ${res.status}`);
	const { access_token } = await res.json();
	return { email, password, access_token };
}

/** Log in via the UI and navigate to /dashboard. */
async function loginAndGoToDashboard(page: Page, email: string, password: string) {
	await page.goto('/auth/login');
	await page.fill('[data-testid="login-email"]', email);
	await page.fill('[data-testid="login-password"]', password);
	await page.click('[data-testid="login-submit"]');
	await expect(page).toHaveURL(/\/dashboard/, { timeout: 10_000 });
}

/** Create a proposal via API (returns proposal id). */
async function createProposalViaAPI(token: string, title?: string) {
	const res = await fetch(`${API}/api/v1/proposals`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${token}`
		},
		body: JSON.stringify({ title: title ?? `Proposal ${uid()}` })
	});
	if (!res.ok) throw new Error(`create proposal failed: ${res.status}`);
	const { id } = await res.json();
	return id as string;
}

// ─── AC-1: New proposal button → template picker shows 5 templates ────────────
//
// Clicking "New proposal" opens TemplatePickerModal.
// The picker must show: 1 blank card + 5 template cards = 6 total.

test('AC-1: new proposal button opens template picker with 5 template cards', async ({ page }) => {
	const { email, password } = await registerUser();
	await loginAndGoToDashboard(page, email, password);

	// Template picker should not be visible initially
	await expect(page.locator('[data-testid="template-picker-modal"]')).not.toBeVisible();

	// Click "New proposal"
	await page.locator('[data-testid="new-proposal-btn"]').click();

	// Modal must appear
	await expect(page.locator('[data-testid="template-picker-modal"]')).toBeVisible({ timeout: 3_000 });

	// Blank card must be present
	await expect(page.locator('[data-testid="template-card-blank"]')).toBeVisible();

	// All 5 named templates must be present
	const expectedTemplateIDs = ['web', 'seo', 'smm', 'design', 'consulting'];
	for (const id of expectedTemplateIDs) {
		await expect(
			page.locator(`[data-testid="template-card-${id}"]`),
			`template card "${id}" must be visible`
		).toBeVisible();
	}
});

test('AC-1: template picker can be dismissed with Escape key', async ({ page }) => {
	const { email, password } = await registerUser();
	await loginAndGoToDashboard(page, email, password);

	await page.locator('[data-testid="new-proposal-btn"]').click();
	await expect(page.locator('[data-testid="template-picker-modal"]')).toBeVisible({ timeout: 3_000 });

	await page.keyboard.press('Escape');
	await expect(page.locator('[data-testid="template-picker-modal"]')).not.toBeVisible();
});

// ─── AC-2: Select template → proposal editor opens with pre-filled blocks ────
//
// Selecting a template advances to the "details" step.
// Submitting creates the proposal and redirects to /dashboard/proposals/{id}.

test('AC-2: selecting "web" template and creating navigates to editor', async ({ page }) => {
	const { email, password } = await registerUser();
	await loginAndGoToDashboard(page, email, password);

	await page.locator('[data-testid="new-proposal-btn"]').click();
	await expect(page.locator('[data-testid="template-picker-modal"]')).toBeVisible({ timeout: 3_000 });

	// Select the "web" template — advances to step 2 (details)
	await page.locator('[data-testid="template-card-web"]').click();

	// Details step: title and client inputs must be visible
	await expect(page.locator('[data-testid="picker-title"]')).toBeVisible();
	await expect(page.locator('[data-testid="picker-client"]')).toBeVisible();

	// Fill optional title
	await page.fill('[data-testid="picker-title"]', 'My Web Proposal');

	// Submit
	await page.locator('[data-testid="picker-create-btn"]').click();

	// Must redirect to proposal editor
	await expect(page).toHaveURL(/\/dashboard\/proposals\/[a-z0-9-]+/, { timeout: 10_000 });
});

test('AC-2: blank template creates an empty proposal', async ({ page }) => {
	const { email, password } = await registerUser();
	await loginAndGoToDashboard(page, email, password);

	await page.locator('[data-testid="new-proposal-btn"]').click();
	await expect(page.locator('[data-testid="template-picker-modal"]')).toBeVisible({ timeout: 3_000 });

	// Select blank card
	await page.locator('[data-testid="template-card-blank"]').click();

	// Details step must appear
	await expect(page.locator('[data-testid="picker-create-btn"]')).toBeVisible();

	// Create without filling any fields (title is optional)
	await page.locator('[data-testid="picker-create-btn"]').click();

	// Must navigate to proposal editor
	await expect(page).toHaveURL(/\/dashboard\/proposals\/[a-z0-9-]+/, { timeout: 10_000 });
});

test('AC-2: detail step shows back button that returns to template picker', async ({ page }) => {
	const { email, password } = await registerUser();
	await loginAndGoToDashboard(page, email, password);

	await page.locator('[data-testid="new-proposal-btn"]').click();
	await page.locator('[data-testid="template-card-seo"]').click();

	// Now on detail step — both back button and create button visible
	await expect(page.locator('[data-testid="picker-create-btn"]')).toBeVisible();

	// Click Back (first button in form footer)
	await page.locator('form button[type="button"]').click();

	// Should return to template picker step — template cards visible again
	await expect(page.locator('[data-testid="template-card-web"]')).toBeVisible();
});

// ─── AC-3: Free plan limit → paywall shown when creating 4th proposal ─────────
//
// Free plan allows 3 non-deleted proposals total.
// After 3 are created via API, the 4th attempt triggers a 402 from the backend
// and the frontend shows [data-testid="plan-limit-modal"].

test('AC-3: Free plan user hitting proposal limit sees paywall modal', async ({ page }) => {
	const { email, password, access_token } = await registerUser();

	// Create 3 proposals via API (puts the user at the plan limit)
	await Promise.all([
		createProposalViaAPI(access_token, 'Proposal 1'),
		createProposalViaAPI(access_token, 'Proposal 2'),
		createProposalViaAPI(access_token, 'Proposal 3')
	]);

	await loginAndGoToDashboard(page, email, password);

	// Paywall must not be visible initially
	await expect(page.locator('[data-testid="plan-limit-modal"]')).not.toBeVisible();

	// Open template picker (frontend doesn't know about total-count limit yet)
	await page.locator('[data-testid="new-proposal-btn"]').click();
	await expect(page.locator('[data-testid="template-picker-modal"]')).toBeVisible({ timeout: 3_000 });

	// Select a template and submit to trigger the backend 402
	await page.locator('[data-testid="template-card-web"]').click();
	await page.locator('[data-testid="picker-create-btn"]').click();

	// Backend returns 402 → frontend shows plan-limit-modal
	await expect(page.locator('[data-testid="plan-limit-modal"]')).toBeVisible({ timeout: 5_000 });

	// User should remain on /dashboard (not navigated to editor)
	await expect(page).toHaveURL(/\/dashboard$/, { timeout: 3_000 });
});

// ─── AC-6 (API level): GET /api/v1/templates — public endpoint ────────────────

test('AC-6: GET /api/v1/templates returns 200 with 5 templates without auth', async () => {
	const res = await fetch(`${API}/api/v1/templates`);

	expect(res.status).toBe(200);

	const body = await res.json();
	expect(Array.isArray(body)).toBe(true);
	expect(body).toHaveLength(5);

	const expectedIDs = ['web', 'seo', 'smm', 'design', 'consulting'];
	const gotIDs = body.map((t: { id: string }) => t.id);
	for (const id of expectedIDs) {
		expect(gotIDs).toContain(id);
	}
});

test('AC-6: GET /api/v1/templates — each template has required fields', async () => {
	const res = await fetch(`${API}/api/v1/templates`);
	const body = await res.json();

	for (const tpl of body) {
		expect(typeof tpl.id).toBe('string');
		expect(tpl.id.length).toBeGreaterThan(0);
		expect(typeof tpl.name).toBe('string');
		expect(tpl.name.length).toBeGreaterThan(0);
		expect(typeof tpl.description).toBe('string');
		expect(Array.isArray(tpl.block_types)).toBe(true);
		expect(tpl.block_types.length).toBeGreaterThan(0);
	}
});

test('AC-6: POST /api/v1/proposals without auth returns 401', async () => {
	const res = await fetch(`${API}/api/v1/proposals`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ title: 'Unauthorized' })
	});

	expect(res.status).toBe(401);
});

test('AC-6: POST /api/v1/proposals with valid auth and template_id creates proposal', async () => {
	const { access_token } = await registerUser();

	const res = await fetch(`${API}/api/v1/proposals`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${access_token}`
		},
		body: JSON.stringify({
			title: 'Template Integration Test',
			template_id: 'web'
		})
	});

	expect(res.status).toBe(201);
	const body = await res.json();
	expect(typeof body.id).toBe('string');
	expect(body.id.length).toBeGreaterThan(0);
});

test('AC-6: POST /api/v1/proposals on Free plan at limit returns 402 PLAN_LIMIT', async () => {
	const { access_token } = await registerUser();

	// Exhaust the Free plan limit (3 proposals)
	for (let i = 0; i < 3; i++) {
		const r = await fetch(`${API}/api/v1/proposals`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				Authorization: `Bearer ${access_token}`
			},
			body: JSON.stringify({ title: `Seed proposal ${i + 1}` })
		});
		if (!r.ok) throw new Error(`seed proposal ${i + 1} failed: ${r.status}`);
	}

	// 4th attempt must fail with 402
	const res = await fetch(`${API}/api/v1/proposals`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${access_token}`
		},
		body: JSON.stringify({ title: 'Over limit' })
	});

	expect(res.status).toBe(402);
	const body = await res.json();
	expect(body.code).toBe('PLAN_LIMIT');
	expect(body.limit).toBe(3);
});
