/**
 * TASK-AQA-104 — Block editor E2E tests
 *
 * AC coverage:
 *  AC1  — Text block: TipTap activates, formatting (bold) applied
 *  AC2  — Price table: 3 rows with prices → correct total
 *  AC3  — Case study: 4 MB image uploads; 6 MB image shows error
 *  AC4  — Team member: photo + fields → block saves successfully
 *  AC5  — Drag-and-drop: block order changes and persists after reload
 *  AC6  — Auto-save: change → 2 s debounce → "Saved ✓" status
 *  AC7  — Delete block: first click shows confirm, second click removes block
 *  AC8  — Duplicate block: clone inserted immediately after original
 */

import { test, expect, Page } from '@playwright/test';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import { registerAndLogin, createBlankProposal } from './helpers/auth';

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

/** Creates a temporary PNG file of the given size for upload testing. */
function makeTempPng(sizeBytes: number): string {
	// Minimal valid PNG header (8 bytes signature + IHDR chunk)
	const PNG_HEADER = Buffer.from([
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // signature
		0x00, 0x00, 0x00, 0x0d, // IHDR length
		0x49, 0x48, 0x44, 0x52, // IHDR
		0x00, 0x00, 0x00, 0x01, // width = 1
		0x00, 0x00, 0x00, 0x01, // height = 1
		0x08, 0x02, 0x00, 0x00, 0x00, // 8-bit RGB
		0x90, 0x77, 0x53, 0xde  // CRC
	]);

	// Pad to the requested size with zeros
	const buf = Buffer.alloc(sizeBytes, 0);
	PNG_HEADER.copy(buf);

	const filePath = path.join(os.tmpdir(), `proply-test-${sizeBytes}.png`);
	fs.writeFileSync(filePath, buf);
	return filePath;
}

/** Clicks "Add block", then selects the given block type from the menu. */
async function addBlock(page: Page, blockType: string) {
	await page.getByRole('button', { name: 'Add block' }).click();
	await page.getByTitle(blockType).click();
}

/** Returns the block container at the given 0-based index within the DnD zone. */
function blockAt(page: Page, index: number) {
	return page.locator('section[class*="space-y-3"] > div').nth(index);
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

test.describe('Block editor — TASK-AQA-104', () => {
	// Each test gets an isolated user + blank proposal.
	test.beforeEach(async ({ page }) => {
		await registerAndLogin(page);
		await createBlankProposal(page, 'AQA Block Editor Test');
	});

	// ── AC1: Text block — TipTap activates + bold formatting applied ──────────

	test('AC1: text block — TipTap activates and bold formatting applies', async ({ page }) => {
		await addBlock(page, 'Intro text');

		// The TipTap editor content area is a [contenteditable] div
		// Use keyboard.type() instead of fill() — TipTap requires real input events
		const editor = page.locator('[contenteditable="true"]').first();
		await editor.click();
		await page.keyboard.type('Hello world');

		// Select all and apply bold via toolbar
		await page.keyboard.press('Control+A');
		await page.getByTitle('Bold').click();

		// Bold text is wrapped in <strong>
		const html = await editor.innerHTML();
		expect(html).toMatch(/<strong>/);
	});

	// ── AC2: Price table — 3 rows → correct total ────────────────────────────

	test('AC2: price table — 3 rows with prices produce correct total', async ({ page }) => {
		await addBlock(page, 'Price table');

		// The block already has 1 default row. Add 2 more.
		const addRowBtn = page.getByRole('button', { name: 'Add row' });
		await addRowBtn.click();
		await addRowBtn.click();

		// Fill row data: [service, qty, price]
		const rows: Array<[string, number, number]> = [
			['Design', 1, 500],
			['Development', 2, 300],
			['SEO', 3, 100]
		];

		for (let i = 0; i < 3; i++) {
			const serviceInputs = page.getByPlaceholder('Service name');
			const qtyInputs = page.locator('input[type="number"][min="0"][step="1"]');
			const priceInputs = page.locator('input[type="number"][min="0"][step="0.01"]');

			await serviceInputs.nth(i).fill(rows[i][0]);
			await qtyInputs.nth(i).fill(String(rows[i][1]));
			await priceInputs.nth(i).fill(String(rows[i][2]));
		}

		// Expected total: 1*500 + 2*300 + 3*100 = 500 + 600 + 300 = 1400
		const totalCell = page.locator('tfoot td.font-bold').first();
		await expect(totalCell).toContainText('1,400.00');
	});

	// ── AC3: Case study — 4 MB OK, 6 MB shows error ───────────────────────────

	test('AC3: case study — image up to 5 MB allowed, 6 MB rejected with error', async ({
		page
	}) => {
		// Mock the presign endpoint so no real S3 is needed.
		// For the 4 MB (valid) case we return a fake presigned URL and intercept the PUT.
		await page.route('**/api/v1/upload/presign', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ url: 'http://localhost:9999/fake-s3/test-key', key: 'test-key' })
			});
		});

		// Intercept the fake S3 PUT — always return 200
		await page.route('http://localhost:9999/**', async (route) => {
			await route.fulfill({ status: 200 });
		});

		await addBlock(page, 'Case study');

		const fileInput = page.locator('input[type="file"]').first();

		// Sub-test A: 4 MB PNG → should upload (no client-side size error)
		const file4mb = makeTempPng(4 * 1024 * 1024);
		await fileInput.setInputFiles(file4mb);

		// Should NOT show the "too large" error
		await expect(page.getByText('Max file size is 5 MB.')).not.toBeVisible();
		// The block should transition to "Uploading…" / then show image or no error
		// Wait briefly to confirm the upload pipeline started without client error
		await page.waitForTimeout(500);
		await expect(page.getByText('Max file size is 5 MB.')).not.toBeVisible();

		// Sub-test B: 6 MB PNG → should show client-side error
		const file6mb = makeTempPng(6 * 1024 * 1024);
		await fileInput.setInputFiles(file6mb);
		await expect(page.getByText('Max file size is 5 MB.')).toBeVisible();

		// Cleanup temp files
		fs.unlinkSync(file4mb);
		fs.unlinkSync(file6mb);
	});

	// ── AC4: Team member — photo + fields → block renders correctly ───────────

	test('AC4: team member — photo upload and field input save correctly', async ({ page }) => {
		await page.route('**/api/v1/upload/presign', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ url: 'http://localhost:9999/fake-s3/team-key', key: 'team-key' })
			});
		});
		await page.route('http://localhost:9999/**', async (route) => {
			await route.fulfill({ status: 200 });
		});

		await addBlock(page, 'Team member');

		// Fill name, role, bio
		await page.getByPlaceholder('Full name').fill('Alice Smith');
		await page.getByPlaceholder('Role / title').fill('Lead Designer');
		await page.getByPlaceholder('Short bio…').fill('10 years of experience in UI/UX.');

		// Upload a small team photo (under 2 MB)
		const photoPath = makeTempPng(512 * 1024); // 512 KB
		const photoInput = page.locator('input[type="file"]').first();
		await photoInput.setInputFiles(photoPath);

		// Should not show size error
		await expect(page.getByText('Max file size is 2 MB.')).not.toBeVisible();

		// Wait for autosave
		await expect(page.getByText('Saved ✓')).toBeVisible({ timeout: 5000 });

		fs.unlinkSync(photoPath);
	});

	// ── AC5: Drag-and-drop — order changes and persists after reload ─────────

	test('AC5: drag-and-drop reorders blocks and order persists after reload', async ({
		page,
		baseURL
	}) => {
		// Add two identifiable text blocks
		await addBlock(page, 'Intro text');
		const firstEditor = page.locator('[contenteditable="true"]').first();
		await firstEditor.click();
		await page.keyboard.type('Block ONE');

		await addBlock(page, 'Intro text');
		const secondEditor = page.locator('[contenteditable="true"]').last();
		await secondEditor.click();
		await page.keyboard.type('Block TWO');

		// Wait for autosave to complete before drag
		await expect(page.getByText('Saved ✓')).toBeVisible({ timeout: 5000 });

		// Drag block at index 1 ("Block TWO") to position 0 ("Block ONE")
		const block0 = blockAt(page, 0);
		const block1 = blockAt(page, 1);

		const dragHandle0 = block0.locator('[aria-label="Drag to reorder"]');
		const dragHandle1 = block1.locator('[aria-label="Drag to reorder"]');

		// Perform drag from block1's handle to block0's bounding box
		const targetBox = await block0.boundingBox();
		if (targetBox) {
			await dragHandle1.dragTo(block0, {
				targetPosition: { x: targetBox.width / 2, y: 4 }
			});
		}

		// Wait for finalize + autosave
		await expect(page.getByText('Saved ✓')).toBeVisible({ timeout: 5000 });

		// After drag, the first block should now contain "Block TWO"
		const newFirst = page
			.locator('section[class*="space-y-3"] > div')
			.first()
			.locator('[contenteditable="true"]');
		await expect(newFirst).toContainText('Block TWO');

		// Reload and verify the order was persisted
		await page.reload();
		await page.waitForLoadState('networkidle');

		const reloadedFirst = page
			.locator('section[class*="space-y-3"] > div')
			.first()
			.locator('[contenteditable="true"]');
		await expect(reloadedFirst).toContainText('Block TWO');
	});

	// ── AC6: Auto-save — status shows "Saved ✓" after 2 s ──────────────────

	test('AC6: autosave triggers within 2s after block change', async ({ page }) => {
		await addBlock(page, 'Intro text');

		const editor = page.locator('[contenteditable="true"]').first();
		await editor.click();
		await page.keyboard.type('Auto-save test content');

		// "Saving…" should appear immediately after the debounce starts
		await expect(page.getByText('Saving…')).toBeVisible({ timeout: 3000 });

		// "Saved ✓" should appear once the API call completes
		await expect(page.getByText('Saved ✓')).toBeVisible({ timeout: 5000 });
	});

	// ── AC7: Delete block — confirm dialog, then block disappears ────────────

	test('AC7: delete block requires confirm dialog and removes block on confirm', async ({
		page
	}) => {
		await addBlock(page, 'Intro text');

		const block = blockAt(page, 0);

		// First delete click → confirm prompt appears, block still present
		await block.getByTitle('Delete block').click();
		await expect(page.getByText('Delete?')).toBeVisible();
		await expect(block).toBeVisible();

		// Click "No" — block stays, prompt disappears
		await page.getByRole('button', { name: 'No' }).click();
		await expect(page.getByText('Delete?')).not.toBeVisible();
		await expect(block).toBeVisible();

		// Open confirm again, click "Yes" — block disappears
		await block.getByTitle('Delete block').click();
		await expect(page.getByText('Delete?')).toBeVisible();
		await page.getByRole('button', { name: 'Yes' }).click();

		await expect(page.getByText('No blocks yet')).toBeVisible();
	});

	// ── AC8: Duplicate block — clone inserted after original ─────────────────

	test('AC8: duplicate block inserts clone immediately after original', async ({ page }) => {
		await addBlock(page, 'Intro text');

		const editor = page.locator('[contenteditable="true"]').first();
		await editor.click();
		await page.keyboard.type('Original block');

		const block = blockAt(page, 0);
		await block.getByTitle('Duplicate block').click();

		// Two blocks should now exist
		const blocks = page.locator('section[class*="space-y-3"] > div');
		await expect(blocks).toHaveCount(2);

		// The second block (clone) should also contain the same text
		const cloneEditor = blocks.nth(1).locator('[contenteditable="true"]');
		await expect(cloneEditor).toContainText('Original block');
	});
});
