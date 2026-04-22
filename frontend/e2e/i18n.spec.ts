/**
 * TASK-AQA-113 — i18n localisation testing (EN / RU)
 *
 * AC coverage:
 *  AC1 — Settings → Language = RU → all UI strings shown in Russian
 *  AC2 — Proposal owner language = RU → /p/{slug} rendered in Russian
 *  AC3 — Proposal owner language = EN → /p/{slug} rendered in English
 *  AC4 — Static analysis: no hard-coded UI strings in Svelte components
 *  AC5 — Language toggle EN → RU → EN without page reload → strings stay correct
 *
 * Requires a fully running stack:
 *   - SvelteKit dev server: npm run dev  (port 5173)
 *   - Go backend:           make dev     (port 8080)
 *   - PostgreSQL:           running and migrated
 *
 * Run: npx playwright test e2e/i18n.spec.ts
 */

import { test, expect, type Page } from '@playwright/test';
import { execSync } from 'child_process';
import path from 'path';
import { fileURLToPath } from 'url';
import { registerAndLogin, createBlankProposal } from './helpers/auth';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const API = process.env.VITE_API_URL ?? 'http://localhost:8080';

// ─── Helpers ──────────────────────────────────────────────────────────────────

const uid = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 6);
const freshEmail = () => `i18n+${uid()}@proply-test.io`;

interface AuthResult {
	email: string;
	password: string;
	token: string;
}

/** Register a user and return their email, password, and access token. */
async function registerUser(name = 'i18n Tester'): Promise<AuthResult> {
	const email = freshEmail();
	const password = 'I18nTest1!';

	const res = await fetch(`${API}/api/v1/auth/register`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name, email, password }),
	});
	if (!res.ok) throw new Error(`register failed: ${res.status}`);

	const data = await res.json();
	return { email, password, token: data.access_token as string };
}

/** Set the user's language via PATCH /api/v1/account. */
async function setUserLanguage(token: string, language: 'en' | 'ru'): Promise<void> {
	const res = await fetch(`${API}/api/v1/account`, {
		method: 'PATCH',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${token}`,
		},
		body: JSON.stringify({ language }),
	});
	if (!res.ok) throw new Error(`setUserLanguage failed: ${res.status}`);
}

/** Create a blank proposal and publish it, returning the slug. */
async function createAndPublish(token: string, title = 'i18n Test Proposal'): Promise<string> {
	// Create
	const createRes = await fetch(`${API}/api/v1/proposals`, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${token}`,
		},
		body: JSON.stringify({ title }),
	});
	if (!createRes.ok) throw new Error(`create proposal failed: ${createRes.status}`);
	const proposal = await createRes.json();

	// Publish
	const publishRes = await fetch(`${API}/api/v1/proposals/${proposal.id}/publish`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${token}` },
	});
	if (!publishRes.ok) throw new Error(`publish failed: ${publishRes.status}`);
	const published = await publishRes.json();
	return published.slug as string;
}

/** Log in through the UI and navigate to /dashboard/settings. */
async function loginAndGoToSettings(page: Page, email: string, password: string): Promise<void> {
	await page.goto('/auth/login');
	await page.fill('[data-testid="login-email"]', email);
	await page.fill('[data-testid="login-password"]', password);
	await page.click('[data-testid="login-submit"]');
	await expect(page).toHaveURL(/\/dashboard/, { timeout: 10_000 });
	await page.goto('/dashboard/settings');
	await expect(page.locator('#settings-language')).toBeVisible({ timeout: 8_000 });
}

// ─── AC1: Settings → Language = RU → UI strings switch to Russian ─────────────

test('AC1: selecting RU in Settings switches the dashboard UI to Russian', async ({ page }) => {
	const { email, password } = await registerUser();
	await loginAndGoToSettings(page, email, password);

	// Verify the page is currently in English
	await expect(page.locator('h1')).toHaveText('Settings');

	// Change language selector to RU and save
	await page.selectOption('#settings-language', 'ru');
	await page.getByRole('button', { name: /Save profile/i }).click();

	// Wait for the save confirmation (EN "Saved ✓" before the locale flips, or RU after).
	await expect(
		page.locator(':text("Saved ✓"), :text("Сохранено ✓")')
	).toBeVisible({ timeout: 5_000 });

	// After save the UI should reflect Russian strings immediately (svelte-i18n reactive)
	// Heading of the Settings page in Russian is "Настройки"
	await expect(page.locator('h1')).toHaveText('Настройки', { timeout: 5_000 });

	// Section headings should also be in Russian
	await expect(page.locator('h2').first()).toHaveText('Профиль');
	await expect(page.getByRole('button', { name: 'Сохранить профиль' })).toBeVisible();
});

// ─── AC2: Owner language = RU → /p/{slug} shows Russian strings ───────────────

test('AC2: public proposal of RU-language owner is displayed in Russian', async ({ page }) => {
	const { token } = await registerUser('RU Owner');
	await setUserLanguage(token, 'ru');
	const slug = await createAndPublish(token, 'AC2 RU Proposal');

	await page.goto(`/p/${slug}`);

	// The viewer page sets locale from proposal.language on mount.
	// Key strings that differ between EN/RU:
	//   EN: "Approve proposal"  RU: "Согласовать предложение"
	//   EN: "Powered by Proply" RU: "Сделано с Proply"
	await expect(page.getByRole('button', { name: 'Согласовать предложение' })).toBeVisible({
		timeout: 8_000,
	});
	await expect(page.locator('text=Сделано с Proply')).toBeVisible();
});

// ─── AC3: Owner language = EN → /p/{slug} shows English strings ───────────────

test('AC3: public proposal of EN-language owner is displayed in English', async ({ page }) => {
	const { token } = await registerUser('EN Owner');
	await setUserLanguage(token, 'en');
	const slug = await createAndPublish(token, 'AC3 EN Proposal');

	await page.goto(`/p/${slug}`);

	// EN viewer strings
	await expect(page.getByRole('button', { name: 'Approve proposal' })).toBeVisible({
		timeout: 8_000,
	});
	await expect(page.locator('text=Powered by Proply')).toBeVisible();
});

// ─── AC4: Static analysis — no hard-coded UI strings in Svelte components ─────

test('AC4: no hard-coded user-visible strings in Svelte component files', () => {
	/**
	 * Strategy: grep for patterns that indicate hard-coded strings.
	 *
	 * What we flag as hard-coded:
	 *  - Quoted English words (2+ consecutive alphabetical words) outside of:
	 *    - import/export statements
	 *    - TypeScript type annotations / interfaces
	 *    - HTML attributes (class, type, placeholder, data-*, aria-*, id, href)
	 *    - Comments
	 *    - Translation key lookups: $_('...')  or t('...')
	 *
	 * Rather than an exhaustive regex we take a targeted approach:
	 * check that common hard-coded patterns are ABSENT from template sections.
	 *
	 * We look for English text nodes that appear directly in Svelte templates,
	 * i.e. bare text not wrapped in a $_(…) call and not inside an attribute.
	 */

	// Resolve the frontend src directory relative to this spec file.
	const srcDir = path.resolve(__dirname, '../src');

	// Patterns that indicate hard-coded visible text in Svelte templates:
	//   >  SomeEnglishText   — direct text content
	//   {  SomeEnglishText } — interpolated text without $_()
	//
	// We use ripgrep (rg) if available, falling back to grep.
	// The pattern captures: ">" or "{" followed by 2+ English words not inside
	// a translation function call.  We exclude:
	//   - Lines that contain $_ or t(  (already translated)
	//   - Lines that are comments (<!-- or //)
	//   - Lines that look like attribute values (class= / type= / placeholder= / data-)
	//   - import/export/interface lines
	//   - Empty or whitespace-only content

	// This is a targeted spot-check for the most common hard-coding patterns.
	// A dedicated i18n linter (e.g. eslint-plugin-i18n-checker) should be
	// added as a pre-commit hook for continuous enforcement.

	const PATTERNS_THAT_MUST_NOT_EXIST = [
		// Text node with at least two consecutive English words directly in template
		// Pattern: >   Word Word  (bare visible text, not in an attribute)
		String.raw`>\s{0,10}[A-Z][a-z]+ [A-Z][a-z]`,
	];

	// Files to scan: all Svelte files in src/routes (template-heavy)
	let grepCmd: string;
	try {
		execSync('rg --version', { stdio: 'ignore' });
		grepCmd = 'rg';
	} catch {
		grepCmd = 'grep -r';
	}

	const violations: string[] = [];

	for (const pattern of PATTERNS_THAT_MUST_NOT_EXIST) {
		let output = '';
		try {
			if (grepCmd === 'rg') {
				output = execSync(
					`rg --glob "*.svelte" -n "${pattern}" "${srcDir}/routes"`,
					{ encoding: 'utf-8', stdio: ['pipe', 'pipe', 'ignore'] }
				);
			} else {
				output = execSync(
					`grep -rn --include="*.svelte" "${pattern}" "${srcDir}/routes"`,
					{ encoding: 'utf-8', stdio: ['pipe', 'pipe', 'ignore'] }
				);
			}
		} catch {
			// exit code 1 means no matches — that's the desired outcome
			output = '';
		}

		if (!output.trim()) continue;

		// Filter out lines that are already correctly using $_ or are comments/attributes
		const lines = output
			.split('\n')
			.map((l) => l.trim())
			.filter(Boolean)
			.filter((l) => !l.includes('$_('))     // already translated
			.filter((l) => !l.includes("t('"))      // already translated (t helper)
			.filter((l) => !l.startsWith('<!--'))   // HTML comment
			.filter((l) => !l.includes('//'))        // JS comment
			.filter((l) => !/class=|type=|placeholder=|data-|aria-|href=|id=/.test(l)) // attribute values
			.filter((l) => !/^import |^export /.test(l));  // import/export lines

		if (lines.length > 0) {
			violations.push(...lines.map((l) => `  ${l}`));
		}
	}

	if (violations.length > 0) {
		throw new Error(
			`Hard-coded UI strings found in Svelte components (${violations.length} violation(s)):\n` +
				violations.slice(0, 20).join('\n') +
				(violations.length > 20 ? `\n  … and ${violations.length - 20} more` : '')
		);
	}
});

// ─── AC5: EN → RU → EN without page reload → strings stay correct ─────────────

test('AC5: toggling language EN→RU→EN on settings page updates strings without reload', async ({
	page,
}) => {
	const { email, password } = await registerUser();
	await loginAndGoToSettings(page, email, password);

	// --- Step 1: confirm English baseline ---
	await expect(page.locator('h1')).toHaveText('Settings');
	await expect(page.getByRole('button', { name: /Save profile/i })).toBeVisible();

	// --- Step 2: switch to RU and save ---
	await page.selectOption('#settings-language', 'ru');
	await page.getByRole('button', { name: /Save profile/i }).click();

	// Wait for reactive locale update — heading should flip to Russian
	await expect(page.locator('h1')).toHaveText('Настройки', { timeout: 5_000 });
	// Button text should also be Russian now
	await expect(page.getByRole('button', { name: 'Сохранить профиль' })).toBeVisible();

	// --- Step 3: switch back to EN and save (no reload in between) ---
	await page.selectOption('#settings-language', 'en');
	await page.getByRole('button', { name: 'Сохранить профиль' }).click();

	// Heading should flip back to English
	await expect(page.locator('h1')).toHaveText('Settings', { timeout: 5_000 });
	await expect(page.getByRole('button', { name: /Save profile/i })).toBeVisible();

	// Branding section heading should also be English
	await expect(page.locator('h2').nth(1)).toHaveText('Branding');
});
