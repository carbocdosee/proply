export interface PriceRow {
	service: string;
	qty: number;
	price: number;
}

/** Computes the grand total for a price table: sum of (qty * price) per row. */
export function calcTotal(rows: PriceRow[]): number {
	return rows.reduce((sum, row) => sum + row.qty * row.price, 0);
}

/** Formats a number as a currency string with 2 decimal places (en-US locale). */
export function formatCurrency(value: number): string {
	return value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}
