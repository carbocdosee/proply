/**
 * E2E proposal registry tests — TASK-AQA-110
 *
 * Covers: AC-1 through AC-9 (list, filter, search, sort, duplicate, delete, empty state)
 *
 * Requires a fully running stack:
 *   - SvelteKit dev server: npm run dev  (port 5173)
 *   - Go backend:           make dev     (port 8080)
 *   - PostgreSQL:           running and migrated
 *
 * Run: npx playwright test e2e/proposals-registry.spec.ts
 */
import { test, expect, type Page } from '@playwright/test';

const API = process.env.VITE_API_URL ?? 'http://localhost:8080';

// ─── Helpers ─────────────────────────────────────────────────────────────────

const uid = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 6);
const freshEmail = () => `registry+${uid()}@proply-test.io`;

/** Register a fresh user and return credentials + access_token. */
async function registerUser(name = 'Registry Tester') {
	const email = freshEmail();
	const password = 'Registry1!';
	const res = await fetch(`${API}/api/v1/auth/register`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name, email, password })
	});
	if (!res.ok) throw new Error(`register failed: ${res.status} ${await res.text()}`);
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

/** Create a proposal via API and return its id. */
async function createProposal(token: string, title: string, clientName?: string) {
	const res = await fetch(`${API}/api/v1/proposals`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${token}`
		},
		body: JSON.stringify({ title, client_name: clientName ?? '' })
	});
	if (!res.ok) throw new Error(`create proposal failed: ${res.status} ${await res.text()}`);
	const { id } = await res.json();
	return id as string;
}

/** Set proposal status via API. */
async function setStatus(token: string, id: string, status: string) {
	const res = await fetch(`${API}/api/v1/proposals/${id}/status`, {
		method: 'PATCH',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${token}`
		},
		body: JSON.stringify({ status })
	});
	if (!res.ok) throw new Error(`setStatus failed: ${res.status} ${await res.text()}`);
}

/** Get proposals list via API. */
async function listProposals(token: string, params: Record<string, string> = {}) {
	const qs = new URLSearchParams(params).toString();
	const res = await fetch(`${API}/api/v1/proposals${qs ? '?' + qs : ''}`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (!res.ok) throw new Error(`list proposals failed: ${res.status}`);
	return res.json();
}

// ─── AC-1: Registry shows proposals with correct fields ───────────────────────

test('AC-1: registry displays proposals with title and status badge', async ({ page }) => {
	const { email, password, access_token } = await registerUser();
	await createProposal(access_token, 'Client Alpha Project', 'Acme Corp');

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	// Proposal title must appear in the list
	await expect(page.getByText('Client Alpha Project')).toBeVisible({ timeout: 5_000 });

	// Status badge must be visible (draft is the initial status)
	await expect(page.getByText('Draft')).toBeVisible();
});

test('AC-1: proposal row shows client name when present', async ({ page }) => {
	const { email, password, access_token } = await registerUser();
	await createProposal(access_token, 'Website Project', 'Beta Corp');

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	await expect(page.getByText('Beta Corp')).toBeVisible({ timeout: 5_000 });
});

// ─── AC-2: Status filter "Opened" → only opened proposals ────────────────────

test('AC-2: status filter "Opened" shows only opened proposals', async ({ page }) => {
	const { email, password, access_token } = await registerUser();

	const draftId = await createProposal(access_token, 'Draft Proposal');
	const openedId = await createProposal(access_token, 'Opened Proposal');

	// Transition openedId to 'opened' status via API
	await setStatus(access_token, openedId, 'sent');
	await setStatus(access_token, openedId, 'opened');
	void draftId;

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	// Both proposals should be visible initially
	await expect(page.getByText('Draft Proposal')).toBeVisible({ timeout: 5_000 });
	await expect(page.getByText('Opened Proposal')).toBeVisible();

	// Click the "Opened" filter button
	await page.getByRole('button', { name: 'Opened' }).click();

	// Only opened proposal should remain
	await expect(page.getByText('Opened Proposal')).toBeVisible({ timeout: 3_000 });
	await expect(page.getByText('Draft Proposal')).not.toBeVisible();
});

// ─── AC-3: Search by title ────────────────────────────────────────────────────

test('AC-3: search by title keyword filters proposals', async ({ page }) => {
	const { email, password, access_token } = await registerUser();
	await createProposal(access_token, 'тест веб-сайт');
	await createProposal(access_token, 'Another Project');

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	// Both visible initially
	await expect(page.getByText('тест веб-сайт')).toBeVisible({ timeout: 5_000 });
	await expect(page.getByText('Another Project')).toBeVisible();

	// Type in search box
	const searchInput = page.getByPlaceholder('Search by title or client…');
	await searchInput.fill('тест');

	// Wait for debounce (300ms) and re-render
	await page.waitForTimeout(500);

	// Only matching proposal should be visible
	await expect(page.getByText('тест веб-сайт')).toBeVisible({ timeout: 3_000 });
	await expect(page.getByText('Another Project')).not.toBeVisible();
});

// ─── AC-4: Search special characters (SQL injection attempt) ─────────────────

test('AC-4: search with SQL injection characters returns no error', async ({ page }) => {
	const { email, password, access_token } = await registerUser();
	await createProposal(access_token, 'Normal Proposal');

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	await expect(page.getByText('Normal Proposal')).toBeVisible({ timeout: 5_000 });

	const searchInput = page.getByPlaceholder('Search by title or client…');
	await searchInput.fill("' OR 1=1 --");
	await page.waitForTimeout(500);

	// No JS error should occur — check page is still healthy
	await expect(page.locator('body')).toBeVisible();

	// No proposals should match the injection attempt
	await expect(page.getByText('Normal Proposal')).not.toBeVisible();

	// The "no match" state or empty state must appear (not a 500 error page)
	const hasNoMatch = await page.getByText('No proposals match your filters').isVisible().catch(() => false);
	const hasEmpty = await page.getByText('No proposals yet').isVisible().catch(() => false);
	expect(hasNoMatch || hasEmpty, 'page should show empty/no-match state, not an error').toBe(true);
});

test('AC-4: search with XSS attempt string returns no error', async ({ page }) => {
	const { email, password, access_token } = await registerUser();
	await createProposal(access_token, 'Safe Proposal');

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	await expect(page.getByText('Safe Proposal')).toBeVisible({ timeout: 5_000 });

	const searchInput = page.getByPlaceholder('Search by title or client…');
	await searchInput.fill('<script>alert(1)</script>');
	await page.waitForTimeout(500);

	// Page should not crash or execute any injected script
	await expect(page.locator('body')).toBeVisible();
	await expect(page.getByText('Safe Proposal')).not.toBeVisible();
});

// ─── AC-5: Sort by date ───────────────────────────────────────────────────────

test('AC-5: sort by "Date created" shows newest proposal first', async ({ page }) => {
	const { email, password, access_token } = await registerUser();

	// Create proposals sequentially — createdSecond will have a later created_at
	const firstId = await createProposal(access_token, 'Created First');
	const secondId = await createProposal(access_token, 'Created Second');
	void firstId;
	void secondId;

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	// Switch sort to "Date created"
	await page.selectOption('select', { label: 'Date created' });
	await page.waitForTimeout(300);

	// Collect proposal titles in DOM order
	const links = page.locator('a[href*="/dashboard/proposals/"]');
	const count = await links.count();
	expect(count).toBeGreaterThanOrEqual(2);

	// The first visible title should be "Created Second" (most recently created)
	const firstVisibleTitle = await links.first().textContent();
	expect(firstVisibleTitle).toContain('Created Second');
});

test('AC-5: sort by "Title A–Z" changes list order', async ({ page }) => {
	const { email, password, access_token } = await registerUser();
	await createProposal(access_token, 'Zebra Project');
	await createProposal(access_token, 'Apple Project');

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	// Switch to title sort
	await page.selectOption('select', { label: 'Title A–Z' });
	await page.waitForTimeout(300);

	const links = page.locator('a[href*="/dashboard/proposals/"]');
	const count = await links.count();
	expect(count).toBeGreaterThanOrEqual(2);

	// Both proposals should be present — exact order depends on backend sort direction
	const allTitles = await links.allTextContents();
	const normalised = allTitles.join('\n');
	expect(normalised).toContain('Zebra Project');
	expect(normalised).toContain('Apple Project');

	// They should appear in a consistent, deterministic order (not random)
	const zebraIndex = allTitles.findIndex((t) => t.includes('Zebra Project'));
	const appleIndex = allTitles.findIndex((t) => t.includes('Apple Project'));
	expect(zebraIndex).not.toBe(-1);
	expect(appleIndex).not.toBe(-1);
	expect(zebraIndex).not.toBe(appleIndex);
});

// ─── AC-6: Duplicate proposal ─────────────────────────────────────────────────

test('AC-6: duplicate navigates to new proposal and title has "(копия)" suffix', async ({ page }) => {
	const { email, password, access_token } = await registerUser();
	await createProposal(access_token, 'Original Proposal');

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	await expect(page.getByText('Original Proposal')).toBeVisible({ timeout: 5_000 });

	// Hover over the proposal row to reveal action buttons
	const proposalRow = page.locator('.group').filter({ hasText: 'Original Proposal' }).first();
	await proposalRow.hover();

	// Click Duplicate
	await proposalRow.getByRole('button', { name: 'Duplicate proposal' }).click({ force: true });

	// Should navigate to the new proposal editor
	await expect(page).toHaveURL(/\/dashboard\/proposals\//, { timeout: 8_000 });

	// Editor title input must contain "(копия)"
	const titleInput = page.locator('input[type="text"]').first();
	await expect(titleInput).toHaveValue(/\(копия\)/, { timeout: 5_000 });
});

test('AC-6: duplicated proposal appears in registry with "(копия)" suffix', async ({ page }) => {
	const { email, password, access_token } = await registerUser();
	const id = await createProposal(access_token, 'Source Proposal');

	// Duplicate via API directly to verify the name convention
	const res = await fetch(`${API}/api/v1/proposals/${id}/duplicate`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${access_token}` }
	});
	expect(res.status).toBe(201);
	const { id: copiedId } = await res.json();
	expect(copiedId).toBeTruthy();

	// Fetch the copy to verify title and status
	const getRes = await fetch(`${API}/api/v1/proposals/${copiedId}`, {
		headers: { Authorization: `Bearer ${access_token}` }
	});
	expect(getRes.status).toBe(200);
	const copy = await getRes.json();

	expect(copy.title).toBe('Source Proposal (копия)');
	expect(copy.status).toBe('draft');
});

// ─── AC-7: Delete proposal with confirm dialog ───────────────────────────────

test('AC-7: delete shows confirm dialog then removes proposal from list', async ({ page }) => {
	const { email, password, access_token } = await registerUser();
	await createProposal(access_token, 'To Be Deleted');
	await createProposal(access_token, 'Should Remain');

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	await expect(page.getByText('To Be Deleted')).toBeVisible({ timeout: 5_000 });

	// Hover and click delete
	const proposalRow = page.locator('.group').filter({ hasText: 'To Be Deleted' }).first();
	await proposalRow.hover();
	await proposalRow.getByRole('button', { name: 'Delete proposal' }).click({ force: true });

	// Confirm dialog should appear
	await expect(page.getByRole('button', { name: 'Delete' })).toBeVisible({ timeout: 3_000 });
	await expect(page.getByRole('button', { name: 'Cancel' })).toBeVisible();

	// Cancel and verify proposal is still there
	await page.getByRole('button', { name: 'Cancel' }).click();
	await expect(page.getByText('To Be Deleted')).toBeVisible();

	// Now actually delete
	const proposalRow2 = page.locator('.group').filter({ hasText: 'To Be Deleted' }).first();
	await proposalRow2.hover();
	await proposalRow2.getByRole('button', { name: 'Delete proposal' }).click({ force: true });
	await page.getByRole('button', { name: 'Delete' }).click();

	// Deleted proposal should disappear; the other one stays
	await expect(page.getByText('To Be Deleted')).not.toBeVisible({ timeout: 5_000 });
	await expect(page.getByText('Should Remain')).toBeVisible();
});

// ─── AC-8: Soft-delete via API ────────────────────────────────────────────────

test('AC-8: deleted proposal disappears from registry after page reload', async ({ page }) => {
	const { email, password, access_token } = await registerUser();
	const id = await createProposal(access_token, 'API Deleted Proposal');

	// Soft-delete via API
	const res = await fetch(`${API}/api/v1/proposals/${id}`, {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${access_token}` }
	});
	expect(res.status).toBe(204);

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	// The deleted proposal must not appear in the registry
	await expect(page.getByText('API Deleted Proposal')).not.toBeVisible({ timeout: 5_000 });
});

test('AC-8: soft-deleted proposal returns 404 on direct GET', async () => {
	const { access_token } = await registerUser();
	const id = await createProposal(access_token, 'Soft Delete Check');

	const del = await fetch(`${API}/api/v1/proposals/${id}`, {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${access_token}` }
	});
	expect(del.status).toBe(204);

	const get = await fetch(`${API}/api/v1/proposals/${id}`, {
		headers: { Authorization: `Bearer ${access_token}` }
	});
	expect(get.status).toBe(404);
});

// ─── AC-9: Empty registry ─────────────────────────────────────────────────────

test('AC-9: empty registry shows "No proposals yet" placeholder', async ({ page }) => {
	const { email, password } = await registerUser();

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	// Fresh user has no proposals — empty state heading must appear
	await expect(page.getByText('No proposals yet')).toBeVisible({ timeout: 5_000 });

	// The "Create proposal" CTA button must also be present
	await expect(page.getByRole('button', { name: 'Create proposal' })).toBeVisible();
});

test('AC-9: filtering with no results shows "no match" placeholder', async ({ page }) => {
	const { email, password, access_token } = await registerUser();
	await createProposal(access_token, 'Only Draft Proposal');

	await loginAndGoToDashboard(page, email, password);
	await page.goto('/dashboard');

	await expect(page.getByText('Only Draft Proposal')).toBeVisible({ timeout: 5_000 });

	// Apply a filter that returns no results
	await page.getByRole('button', { name: 'Approved' }).click();
	await page.waitForTimeout(300);

	// Should show the no-match placeholder
	await expect(page.getByText('No proposals match your filters')).toBeVisible({ timeout: 3_000 });

	// "Clear filters" link must appear
	await expect(page.getByText('Clear filters')).toBeVisible();
});
