<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { createEventDispatcher } from 'svelte';

	interface PriceRow { service: string; qty: number; price: number }

	export let data: { rows: PriceRow[] };
	export let readonly = false;

	const dispatch = createEventDispatcher<{ update: { rows: PriceRow[] } }>();

	let rows: PriceRow[] = data.rows?.map((r) => ({ ...r })) ?? [];

	$: total = rows.reduce((s, r) => s + r.qty * r.price, 0);

	function addRow() {
		rows = [...rows, { service: '', qty: 1, price: 0 }];
		notify();
	}

	function deleteRow(i: number) {
		rows = rows.filter((_, idx) => idx !== i);
		notify();
	}

	function notify() {
		dispatch('update', { rows: rows.map((r) => ({ ...r })) });
	}

	function fmt(n: number) {
		return n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
	}
</script>

{#if readonly}
	<table class="w-full text-sm">
		<thead>
			<tr class="border-b border-gray-200">
				<th class="text-left py-2 font-medium text-gray-600">{$_('price_table.service')}</th>
				<th class="text-right py-2 font-medium text-gray-600 w-16">{$_('price_table.qty')}</th>
				<th class="text-right py-2 font-medium text-gray-600 w-28">{$_('price_table.unit_price')}</th>
				<th class="text-right py-2 font-medium text-gray-600 w-28">{$_('price_table.line_total')}</th>
			</tr>
		</thead>
		<tbody>
			{#each rows as row}
				<tr class="border-b border-gray-100">
					<td class="py-2">{row.service}</td>
					<td class="py-2 text-right">{row.qty}</td>
					<td class="py-2 text-right">{fmt(row.price)}</td>
					<td class="py-2 text-right">{fmt(row.qty * row.price)}</td>
				</tr>
			{/each}
		</tbody>
		<tfoot>
			<tr>
				<td colspan="3" class="pt-3 text-right font-semibold text-gray-700">{$_('price_table.total')}</td>
				<td class="pt-3 text-right font-bold text-gray-900">{fmt(total)}</td>
			</tr>
		</tfoot>
	</table>
{:else}
	<div class="overflow-x-auto">
		<table class="w-full text-sm">
			<thead>
				<tr class="border-b border-gray-200">
					<th class="text-left py-2 pr-2 font-medium text-gray-600">{$_('price_table.service_header')}</th>
					<th class="py-2 px-2 font-medium text-gray-600 w-20 text-center">{$_('price_table.qty')}</th>
					<th class="py-2 px-2 font-medium text-gray-600 w-28 text-right">{$_('price_table.unit_price')}</th>
					<th class="py-2 pl-2 font-medium text-gray-600 w-28 text-right">{$_('price_table.line_total')}</th>
					<th class="w-8"></th>
				</tr>
			</thead>
			<tbody>
				{#each rows as row, i}
					<tr class="border-b border-gray-50 group">
						<td class="py-1 pr-2">
							<input
								type="text"
								bind:value={row.service}
								on:input={notify}
								placeholder={$_('price_table.service_placeholder')}
								class="w-full px-2 py-1 border border-transparent rounded focus:border-indigo-300 focus:outline-none text-sm bg-transparent hover:bg-gray-50 focus:bg-white transition-colors"
							/>
						</td>
						<td class="py-1 px-2">
							<input
								type="number"
								bind:value={row.qty}
								on:input={notify}
								min="0"
								step="1"
								class="w-full px-2 py-1 border border-transparent rounded focus:border-indigo-300 focus:outline-none text-sm text-center bg-transparent hover:bg-gray-50 focus:bg-white transition-colors"
							/>
						</td>
						<td class="py-1 px-2">
							<input
								type="number"
								bind:value={row.price}
								on:input={notify}
								min="0"
								step="0.01"
								class="w-full px-2 py-1 border border-transparent rounded focus:border-indigo-300 focus:outline-none text-sm text-right bg-transparent hover:bg-gray-50 focus:bg-white transition-colors"
							/>
						</td>
						<td class="py-1 pl-2 text-right text-gray-600 tabular-nums">{fmt(row.qty * row.price)}</td>
						<td class="py-1 pl-1">
							<button
								type="button"
								on:click={() => deleteRow(i)}
								class="opacity-0 group-hover:opacity-100 p-1 rounded text-gray-400 hover:text-red-500 hover:bg-red-50 transition-all"
								aria-label={$_('price_table.delete_row')}
							>
								<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path d="M6 18L18 6M6 6l12 12"/>
								</svg>
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
			<tfoot>
				<tr>
					<td colspan="3" class="pt-3 text-right font-semibold text-gray-700 pr-2">{$_('price_table.total')}</td>
					<td class="pt-3 text-right font-bold text-gray-900 tabular-nums">{fmt(total)}</td>
					<td></td>
				</tr>
			</tfoot>
		</table>

		<button
			type="button"
			on:click={addRow}
			class="mt-3 flex items-center gap-1.5 text-xs text-indigo-600 hover:text-indigo-700 font-medium"
		>
			<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
				<path d="M12 4v16m8-8H4"/>
			</svg>
			{$_('price_table.add_row')}
		</button>
	</div>
{/if}
