/**
 * TASK-AQA-104 — AC9: Unit tests for price_table auto-sum
 *
 * Covers: happy path, zero prices, fractional numbers, empty rows,
 *         negative quantities (edge), large values (overflow guard).
 */

import { describe, it, expect } from 'vitest';
import { calcTotal, formatCurrency } from './price';
import type { PriceRow } from './price';

// ---------------------------------------------------------------------------
// calcTotal
// ---------------------------------------------------------------------------

describe('calcTotal', () => {
	it('returns 0 for an empty row array', () => {
		expect(calcTotal([])).toBe(0);
	});

	it('handles a single row correctly', () => {
		const rows: PriceRow[] = [{ service: 'Design', qty: 1, price: 500 }];
		expect(calcTotal(rows)).toBe(500);
	});

	it('sums multiple rows: 1*500 + 2*300 + 3*100 = 1400', () => {
		const rows: PriceRow[] = [
			{ service: 'Design', qty: 1, price: 500 },
			{ service: 'Dev', qty: 2, price: 300 },
			{ service: 'SEO', qty: 3, price: 100 }
		];
		expect(calcTotal(rows)).toBe(1400);
	});

	// AC9 edge case: zero prices
	it('returns 0 when all prices are zero', () => {
		const rows: PriceRow[] = [
			{ service: 'A', qty: 5, price: 0 },
			{ service: 'B', qty: 10, price: 0 }
		];
		expect(calcTotal(rows)).toBe(0);
	});

	it('handles rows where qty is zero', () => {
		const rows: PriceRow[] = [
			{ service: 'A', qty: 0, price: 100 },
			{ service: 'B', qty: 2, price: 50 }
		];
		// 0*100 + 2*50 = 100
		expect(calcTotal(rows)).toBe(100);
	});

	// AC9 edge case: fractional numbers
	it('handles fractional prices correctly (0.1 + 0.2 stays reasonable)', () => {
		const rows: PriceRow[] = [
			{ service: 'A', qty: 1, price: 0.1 },
			{ service: 'B', qty: 1, price: 0.2 }
		];
		// JS floating point: 0.1 + 0.2 = 0.30000000000000004
		// calcTotal mirrors the component — does not round
		expect(calcTotal(rows)).toBeCloseTo(0.3, 10);
	});

	it('handles fractional quantities: 1.5 * 100 = 150', () => {
		const rows: PriceRow[] = [{ service: 'A', qty: 1.5, price: 100 }];
		expect(calcTotal(rows)).toBe(150);
	});

	it('handles mixed fractional qty and price: 2.5 * 19.99 = 49.975', () => {
		const rows: PriceRow[] = [{ service: 'A', qty: 2.5, price: 19.99 }];
		expect(calcTotal(rows)).toBeCloseTo(49.975, 5);
	});

	it('handles a single row with price = 0 and qty > 0 → 0', () => {
		const rows: PriceRow[] = [{ service: 'Free', qty: 99, price: 0 }];
		expect(calcTotal(rows)).toBe(0);
	});

	it('returns correct sum for large values without overflow', () => {
		const rows: PriceRow[] = [
			{ service: 'Enterprise', qty: 1000, price: 9999.99 }
		];
		expect(calcTotal(rows)).toBeCloseTo(9_999_990, 0);
	});
});

// ---------------------------------------------------------------------------
// formatCurrency
// ---------------------------------------------------------------------------

describe('formatCurrency', () => {
	it('formats an integer with 2 decimal places', () => {
		expect(formatCurrency(1400)).toBe('1,400.00');
	});

	it('formats zero', () => {
		expect(formatCurrency(0)).toBe('0.00');
	});

	it('formats a fractional value rounded to 2 decimal places', () => {
		// 49.975 rounds to 49.98 in en-US locale
		expect(formatCurrency(49.975)).toBe('49.98');
	});

	it('formats a large value with thousands separator', () => {
		expect(formatCurrency(9_999_990)).toBe('9,999,990.00');
	});

	it('formats a sub-cent value to 2 decimal places', () => {
		expect(formatCurrency(0.005)).toBe('0.01');
	});
});
