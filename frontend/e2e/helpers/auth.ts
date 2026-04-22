import type { Page } from '@playwright/test';

/** Registers a fresh user, logs in, returns access credentials. */
export async function registerAndLogin(
	page: Page,
	name = 'Test User'
): Promise<{ email: string; password: string }> {
	const uid = crypto.randomUUID().slice(0, 8);
	const email = `test-${uid}@example.com`;
	const password = 'Password123!';

	await page.goto('/auth/register');
	await page.getByPlaceholder('Jane Smith').fill(name);
	await page.getByPlaceholder('you@agency.com').fill(email);
	await page.getByLabel('Password').fill(password);
	await page.getByRole('button', { name: 'Create account' }).click();

	// Wait for redirect to dashboard
	await page.waitForURL('**/dashboard');

	return { email, password };
}

/**
 * Creates a blank proposal from the template picker.
 * Returns the proposal ID extracted from the URL after navigation.
 */
export async function createBlankProposal(page: Page, title = 'Test Proposal'): Promise<string> {
	await page.goto('/dashboard');
	await page.getByRole('button', { name: 'New proposal' }).click();

	// Template picker — step 1: select blank
	await page.getByText('Blank proposal').click();

	// Step 2: fill title and create
	const titleInput = page.getByPlaceholder('e.g. Website redesign for Acme Corp');
	await titleInput.fill(title);
	await page.getByRole('button', { name: 'Create proposal' }).click();

	// Should redirect to /dashboard/proposals/{id}
	await page.waitForURL('**/dashboard/proposals/**');

	const url = page.url();
	const id = url.split('/proposals/').pop() ?? '';
	return id;
}

/**
 * Creates a proposal from the "Web Development" template.
 * Returns the proposal ID.
 */
export async function createWebProposal(page: Page, title = 'Web Test Proposal'): Promise<string> {
	await page.goto('/dashboard');
	await page.getByRole('button', { name: 'New proposal' }).click();

	// Pick the Web Development template
	await page.getByText('Web Development').click();
	const titleInput = page.getByPlaceholder('e.g. Website redesign for Acme Corp');
	await titleInput.fill(title);
	await page.getByRole('button', { name: 'Create proposal' }).click();

	await page.waitForURL('**/dashboard/proposals/**');
	const url = page.url();
	return url.split('/proposals/').pop() ?? '';
}
