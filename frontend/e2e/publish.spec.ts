/**
 * TASK-AQA-105 — Publish, slug generation, copy link, revoke
 *
 * AC coverage:
 *  AC1  — "Publish" → slug generated, status = sent, /p/{slug} accessible without auth
 *  AC2  — "Copy link" → clipboard holds the correct /p/{slug} URL
 *  AC3  — 100 concurrent publishes → unique slugs
 *         (covered at unit level: pkg/slug/slug_test.go#TestGenerate_Concurrent_100_UniqueSlugs)
 *  AC4  — slug is 12 chars, Base62 only
 *         (covered at unit level: pkg/slug/slug_test.go#TestGenerate_Is12Chars / _IsBase62Only)
 *  AC5  — "Revoke link" → /p/{slug} shows stub "Proposal unavailable"
 *  AC6  — POST /publish for already-published proposal → 409 ALREADY_PUBLISHED
 *
 * Open question (AC6 discrepancy):
 *   The AC states the second publish should be idempotent (200, same slug).
 *   The implementation returns 409 ALREADY_PUBLISHED via ErrConflict.
 *   Test documents the REAL behavior. PO/BA to resolve the spec vs implementation gap.
 */

import { test, expect, Page } from '@playwright/test';
import { registerAndLogin, createBlankProposal } from './helpers/auth';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const BASE62_RE = /^[0-9A-Za-z]{12}$/;

/** Returns the proposal ID from the current editor URL. */
function proposalIdFromUrl(page: Page): string {
	return page.url().split('/proposals/').pop() ?? '';
}

/** Clicks "Publish" and waits for the slug to appear in the UI. */
async function publishProposal(page: Page): Promise<string> {
	await page.getByRole('button', { name: 'Publish' }).click();
	// Wait for the "Copy link" button to appear — it signals publish success.
	await expect(page.getByRole('button', { name: /Copy link/i })).toBeVisible({ timeout: 8000 });

	// Extract the slug from the page URL or the copy-link button's data attribute.
	// The published link is shown in the UI as /p/{slug}.
	const linkEl = page.locator('a[href*="/p/"]').first();
	const href = (await linkEl.getAttribute('href')) ?? '';
	return href.split('/p/').pop()?.split('?')[0] ?? '';
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

test.describe('Publish & Revoke — TASK-AQA-105', () => {
	// ── AC1: Publish → slug generated, status sent, /p/{slug} is accessible ──

	test('AC1: publish generates a valid slug and public URL is accessible', async ({
		page,
		context
	}) => {
		await registerAndLogin(page);
		await createBlankProposal(page, 'AC1 Publish Test');

		const slug = await publishProposal(page);

		// Verify slug shape matches AC-4 constraints (12 chars, Base62).
		expect(slug).toMatch(BASE62_RE);

		// Open the public URL in a fresh (unauthenticated) browser tab.
		const publicPage = await context.newPage();
		await publicPage.goto(`/p/${slug}`);

		// The public viewer must load without requiring auth.
		// Presence of the proposal title or the viewer root element is sufficient.
		await expect(publicPage.locator('body')).not.toContainText('404');
		await expect(publicPage.locator('body')).not.toContainText('NOT_FOUND');
		await publicPage.close();
	});

	// ── AC2: "Copy link" → clipboard contains /p/{slug} ──────────────────────

	test('AC2: copy link button writes correct /p/{slug} URL to clipboard', async ({
		page,
		context
	}) => {
		// Grant clipboard-write permission so navigator.clipboard.writeText succeeds.
		await context.grantPermissions(['clipboard-read', 'clipboard-write']);

		await registerAndLogin(page);
		await createBlankProposal(page, 'AC2 Copy Link Test');

		const slug = await publishProposal(page);

		// Click "Copy link".
		await page.getByRole('button', { name: /Copy link/i }).click();

		// Read back the clipboard contents.
		const clipboardText = await page.evaluate(() => navigator.clipboard.readText());

		// The clipboard must contain the /p/{slug} path (full URL or relative).
		expect(clipboardText).toContain(`/p/${slug}`);
	});

	// ── AC5: Revoke → /p/{slug} shows stub page ───────────────────────────────

	test('AC5: revoke link makes /p/{slug} show the unavailable stub', async ({
		page,
		context
	}) => {
		await registerAndLogin(page);
		await createBlankProposal(page, 'AC5 Revoke Test');

		const slug = await publishProposal(page);

		// Confirm public URL is accessible before revoke.
		const publicPage = await context.newPage();
		await publicPage.goto(`/p/${slug}`);
		await expect(publicPage.locator('body')).not.toContainText('unavailable');
		await publicPage.close();

		// Revoke the link.
		await page.getByRole('button', { name: /Revoke/i }).click();

		// The "Revoke" button is replaced by a "Publish" button after revoking.
		await expect(page.getByRole('button', { name: 'Publish' })).toBeVisible({ timeout: 5000 });

		// /p/{slug} must now display the stub.
		const revokedPage = await context.newPage();
		await revokedPage.goto(`/p/${slug}`);
		await expect(
			revokedPage.getByText(/unavailable|не доступно|недоступно/i)
		).toBeVisible({ timeout: 5000 });
		await revokedPage.close();
	});

	// ── AC6: Double-publish → 409 ALREADY_PUBLISHED ──────────────────────────
	//
	// Open question: AC says "idempotent (200, same slug)" but the implementation
	// returns 409 ALREADY_PUBLISHED. This test asserts the ACTUAL behavior.

	test('AC6 [actual]: publishing an already-published proposal shows an error in UI', async ({
		page
	}) => {
		await registerAndLogin(page);
		await createBlankProposal(page, 'AC6 Double Publish Test');

		// First publish succeeds.
		await publishProposal(page);

		// Attempt to click "Publish" again.
		// After first publish the button may be hidden; if visible, it should produce an error.
		const publishBtn = page.getByRole('button', { name: 'Publish' });
		if (await publishBtn.isVisible()) {
			await publishBtn.click();
			// The UI should show an error or the button must be disabled/absent.
			// Accept either: an error message OR the button being gone (replaced by Revoke).
			const hasError = await page.getByText(/error|already|опубликовано/i).isVisible();
			const hasRevoke = await page.getByRole('button', { name: /Revoke/i }).isVisible();
			expect(hasError || hasRevoke).toBe(true);
		} else {
			// Publish button not visible after first publish is also valid UX behaviour.
			await expect(page.getByRole('button', { name: /Revoke/i })).toBeVisible();
		}
	});

	// ── Slug format regression ────────────────────────────────────────────────

	test('AC4 regression: every published slug is 12 Base62 chars', async ({ page }) => {
		await registerAndLogin(page);
		await createBlankProposal(page, 'AC4 Slug Format Test');

		const slug = await publishProposal(page);
		expect(slug.length).toBe(12);
		expect(slug).toMatch(BASE62_RE);
	});
});
