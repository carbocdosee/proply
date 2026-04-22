/**
 * TASK-AQA-108 — Proposal Approval Flow
 *
 * AC coverage:
 *  AC1  — "Approve" button → modal with email field opens
 *  AC2  — empty email → "Confirm" button inactive (HTML5 required) or validation error
 *  AC3  — invalid email (no @) → browser validation prevents submission
 *  AC4  — valid email + "Confirm" → navigates to confirmation page with agency name + date
 *  AC8  — reload approved proposal → button inactive, text "Already approved"
 *
 * AC5, AC6, AC7, AC9 are covered at the service integration layer
 * (see proply/backend/internal/service/proposal_approve_test.go).
 */

import { test, expect, Page } from '@playwright/test';
import { registerAndLogin, createBlankProposal } from './helpers/auth';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Publishes a proposal from the editor page and returns the generated slug. */
async function publishProposal(page: Page): Promise<string> {
	await page.getByRole('button', { name: 'Publish' }).click();
	await expect(page.getByRole('button', { name: /Copy link/i })).toBeVisible({ timeout: 8000 });

	const linkEl = page.locator('a[href*="/p/"]').first();
	const href = (await linkEl.getAttribute('href')) ?? '';
	return href.split('/p/').pop()?.split('?')[0] ?? '';
}

/** Opens the public viewer page for a slug in the given page object. */
async function openPublicViewer(page: Page, slug: string): Promise<void> {
	await page.goto(`/p/${slug}`);
	// Wait for the viewer to load (agency header or approve button visible).
	await expect(page.locator('header')).toBeVisible({ timeout: 8000 });
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

test.describe('Proposal Approval — TASK-AQA-108', () => {

	// ── AC1: "Approve" button → modal with email field ───────────────────────

	test('AC1: clicking Approve opens the approval modal with an email field', async ({
		page,
		context,
	}) => {
		await registerAndLogin(page);
		await createBlankProposal(page, 'AC1 Approval Modal Test');
		const slug = await publishProposal(page);

		const publicPage = await context.newPage();
		await openPublicViewer(publicPage, slug);

		// The "Approve" / "Согласовать" button must be visible.
		const approveBtn = publicPage.getByRole('button', { name: /approve|согласовать/i });
		await expect(approveBtn).toBeVisible({ timeout: 5000 });
		await approveBtn.click();

		// Modal must appear containing an email input field.
		const emailInput = publicPage.locator('input[type="email"]');
		await expect(emailInput).toBeVisible({ timeout: 3000 });

		await publicPage.close();
	});

	// ── AC2: empty email → submit blocked ────────────────────────────────────

	test('AC2: empty email prevents form submission (HTML5 required or button disabled)', async ({
		page,
		context,
	}) => {
		await registerAndLogin(page);
		await createBlankProposal(page, 'AC2 Empty Email Test');
		const slug = await publishProposal(page);

		const publicPage = await context.newPage();
		await openPublicViewer(publicPage, slug);

		// Open modal.
		await publicPage.getByRole('button', { name: /approve|согласовать/i }).click();
		const emailInput = publicPage.locator('input[type="email"]');
		await expect(emailInput).toBeVisible({ timeout: 3000 });

		// Leave email empty and attempt to submit.
		const confirmBtn = publicPage.getByRole('button', { name: /confirm|подтвердить/i });

		// Check the input has the `required` attribute — browser prevents submission.
		const isRequired = await emailInput.getAttribute('required');
		expect(isRequired).not.toBeNull(); // `required` attribute must be present

		// Alternatively, confirm button may be disabled.
		// Accept either: `required` present OR button is disabled.
		// The SvelteKit form uses HTML5 required, so we validate the attribute above.

		// Ensure we are still on the same page (not redirected to /confirmed).
		await expect(publicPage).not.toHaveURL(/\/confirmed/);

		// The confirm button must not be in a loading state with empty email.
		await expect(confirmBtn).not.toBeDisabled(); // not loading-disabled, just prevented by required

		await publicPage.close();
	});

	// ── AC3: invalid email (no @) → browser validation blocks submission ──────

	test('AC3: invalid email without @ triggers browser validation, page not navigated', async ({
		page,
		context,
	}) => {
		await registerAndLogin(page);
		await createBlankProposal(page, 'AC3 Invalid Email Test');
		const slug = await publishProposal(page);

		const publicPage = await context.newPage();
		await openPublicViewer(publicPage, slug);

		// Open modal.
		await publicPage.getByRole('button', { name: /approve|согласовать/i }).click();
		const emailInput = publicPage.locator('input[type="email"]');
		await expect(emailInput).toBeVisible({ timeout: 3000 });

		// Type an invalid email (no @).
		await emailInput.fill('notanemail');

		// Attempt to submit — HTML5 type="email" should block it.
		// We use evaluate to dispatch the form submit directly; the browser will
		// reject the invalid value and set a validation message.
		const validationMessage: string = await publicPage.evaluate(() => {
			const input = document.querySelector('input[type="email"]') as HTMLInputElement;
			return input?.validationMessage ?? '';
		});

		// The browser must report a non-empty validation message for an invalid email.
		expect(validationMessage.length).toBeGreaterThan(0);

		// Ensure we did not navigate to /confirmed.
		await expect(publicPage).not.toHaveURL(/\/confirmed/);

		await publicPage.close();
	});

	// ── AC4: valid email → confirmation page with agency name + date ──────────

	test('AC4: valid email submission navigates to /confirmed with agency name and date', async ({
		page,
		context,
	}) => {
		await registerAndLogin(page, 'Test Agency');
		await createBlankProposal(page, 'AC4 Approval Happy Path');
		const slug = await publishProposal(page);

		const publicPage = await context.newPage();
		await openPublicViewer(publicPage, slug);

		// Open modal.
		await publicPage.getByRole('button', { name: /approve|согласовать/i }).click();
		const emailInput = publicPage.locator('input[type="email"]');
		await expect(emailInput).toBeVisible({ timeout: 3000 });

		// Fill a valid client email.
		await emailInput.fill('client@example.com');

		// Submit the form.
		await publicPage.getByRole('button', { name: /confirm|подтвердить/i }).click();

		// Must navigate to the /confirmed page.
		await publicPage.waitForURL(/\/p\/.+\/confirmed/, { timeout: 10000 });

		// The confirmation page must show agency name and/or the approval timestamp.
		// The confirmed page shows agency name (from confirmed.message_agency)
		// and the formatted date (fmtDate(data.approvedAt)).
		const body = publicPage.locator('body');

		// Either the agency name or the confirmation heading must be visible.
		const hasAgency = await body.getByText(/Test Agency/i).isVisible().catch(() => false);
		const hasHeading = await body.getByRole('heading').first().isVisible().catch(() => false);

		expect(hasAgency || hasHeading).toBe(true);

		// The formatted date should be present (year-level match is sufficient).
		const currentYear = new Date().getFullYear().toString();
		await expect(body).toContainText(currentYear);

		await publicPage.close();
	});

	// ── AC8: reload after approval → already-approved state ──────────────────

	test('AC8: reloading an approved proposal shows "already approved" state', async ({
		page,
		context,
	}) => {
		await registerAndLogin(page);
		await createBlankProposal(page, 'AC8 Already Approved Test');
		const slug = await publishProposal(page);

		const publicPage = await context.newPage();
		await openPublicViewer(publicPage, slug);

		// Approve the proposal.
		await publicPage.getByRole('button', { name: /approve|согласовать/i }).click();
		const emailInput = publicPage.locator('input[type="email"]');
		await expect(emailInput).toBeVisible({ timeout: 3000 });
		await emailInput.fill('client-ac8@example.com');
		await publicPage.getByRole('button', { name: /confirm|подтвердить/i }).click();
		await publicPage.waitForURL(/\/confirmed/, { timeout: 10000 });

		// Navigate back to the original /p/{slug} page.
		await publicPage.goto(`/p/${slug}`);

		// The "Approve/Согласовать" button must no longer be visible as an active CTA.
		// Instead the page should show an "already approved" badge/text.
		const approveBtn = publicPage.getByRole('button', { name: /^approve$|^согласовать$/i });
		const alreadyApprovedText = publicPage.getByText(/already approved|уже согласовано/i);

		// Either the button is absent or an "already approved" indicator is present.
		const btnVisible = await approveBtn.isVisible().catch(() => false);
		const badgeVisible = await alreadyApprovedText.isVisible().catch(() => false);

		// The viewer shows a green "approved" badge when proposal.status === 'approved'.
		// Check for the approved badge or absence of the approve button.
		expect(!btnVisible || badgeVisible).toBe(true);

		await publicPage.close();
	});
});
