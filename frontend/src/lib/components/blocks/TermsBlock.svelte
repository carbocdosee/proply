<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { createEventDispatcher } from 'svelte';
	import { tick } from 'svelte';

	export let data: { items: string[] };
	export let readonly = false;

	const dispatch = createEventDispatcher<{ update: { items: string[] } }>();

	let items: string[] = [...(data.items ?? [''])];

	function notify() {
		dispatch('update', { items: [...items] });
	}

	async function addItem() {
		items = [...items, ''];
		notify();
		await tick();
		const inputs = document.querySelectorAll<HTMLInputElement>('.terms-item-input');
		inputs[inputs.length - 1]?.focus();
	}

	function deleteItem(i: number) {
		if (items.length <= 1) {
			items[0] = '';
		} else {
			items = items.filter((_, idx) => idx !== i);
		}
		notify();
	}

	function onKeydown(e: KeyboardEvent, i: number) {
		if (e.key === 'Enter') {
			e.preventDefault();
			items = [...items.slice(0, i + 1), '', ...items.slice(i + 1)];
			notify();
			tick().then(() => {
				const inputs = document.querySelectorAll<HTMLInputElement>('.terms-item-input');
				inputs[i + 1]?.focus();
			});
		} else if (e.key === 'Backspace' && items[i] === '' && items.length > 1) {
			e.preventDefault();
			const focusIdx = Math.max(0, i - 1);
			deleteItem(i);
			tick().then(() => {
				const inputs = document.querySelectorAll<HTMLInputElement>('.terms-item-input');
				inputs[focusIdx]?.focus();
			});
		}
	}
</script>

{#if readonly}
	<ol class="list-decimal list-inside space-y-2">
		{#each items as item, i}
			{#if item.trim()}
				<li class="text-sm text-gray-700">{item}</li>
			{/if}
		{/each}
	</ol>
{:else}
	<div class="space-y-2">
		{#each items as item, i}
			<div class="flex items-center gap-2 group">
				<span class="text-xs text-gray-400 w-5 text-right flex-shrink-0 font-mono">{i + 1}.</span>
				<input
					type="text"
					bind:value={items[i]}
					on:input={notify}
					on:keydown={(e) => onKeydown(e, i)}
					placeholder={$_('terms.placeholder')}
					class="terms-item-input flex-1 px-3 py-1.5 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
				/>
				<button
					type="button"
					on:click={() => deleteItem(i)}
					class="p-1 rounded text-gray-300 hover:text-red-500 hover:bg-red-50 opacity-0 group-hover:opacity-100 transition-all flex-shrink-0"
					aria-label={$_('terms.remove')}
				>
					<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path d="M6 18L18 6M6 6l12 12"/>
					</svg>
				</button>
			</div>
		{/each}

		<button
			type="button"
			on:click={addItem}
			class="flex items-center gap-1.5 text-xs text-indigo-600 hover:text-indigo-700 font-medium mt-1"
		>
			<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
				<path d="M12 4v16m8-8H4"/>
			</svg>
			{$_('terms.add')}
		</button>
	</div>
{/if}
