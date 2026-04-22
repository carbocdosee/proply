<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { createEventDispatcher } from 'svelte';
	import { dndzone, SHADOW_ITEM_MARKER_PROPERTY_NAME } from 'svelte-dnd-action';
	import type { Block } from '$lib/api';
	import TextBlock from './blocks/TextBlock.svelte';
	import PriceTableBlock from './blocks/PriceTableBlock.svelte';
	import CaseStudyBlock from './blocks/CaseStudyBlock.svelte';
	import TeamMemberBlock from './blocks/TeamMemberBlock.svelte';
	import TermsBlock from './blocks/TermsBlock.svelte';

	export let blocks: Block[] = [];
	export let token = '';
	export let proposalId = '';
	export let readonly = false;

	const dispatch = createEventDispatcher<{ change: Block[] }>();

	// Working copy for DnD (items need an `id` field — our blocks already have one)
	let items: Block[] = reindex([...blocks]);

	// Re-index order values 0…n
	function reindex(arr: Block[]): Block[] {
		return arr.map((b, i) => ({ ...b, order: i }));
	}

	function notify(arr: Block[]) {
		items = arr;
		dispatch('change', arr);
	}

	// ── DnD ────────────────────────────────────────────────────────────
	const flipDurationMs = 150;

	function handleConsider(e: CustomEvent<{ items: Block[] }>) {
		items = e.detail.items;
	}

	function handleFinalize(e: CustomEvent<{ items: Block[] }>) {
		notify(reindex(e.detail.items));
	}

	// ── Block operations ───────────────────────────────────────────────
	let confirmDeleteId: string | null = null;

	function deleteBlock(id: string) {
		if (confirmDeleteId !== id) {
			confirmDeleteId = id;
			return;
		}
		confirmDeleteId = null;
		notify(reindex(items.filter((b) => b.id !== id)));
	}

	function cancelDelete() {
		confirmDeleteId = null;
	}

	function duplicateBlock(id: string) {
		const idx = items.findIndex((b) => b.id === id);
		if (idx === -1) return;
		const original = items[idx];
		const clone: Block = {
			...original,
			id: crypto.randomUUID(),
			data: JSON.parse(JSON.stringify(original.data))
		};
		const next = [...items.slice(0, idx + 1), clone, ...items.slice(idx + 1)];
		notify(reindex(next));
	}

	function updateBlockData(id: string, newData: Record<string, unknown>) {
		notify(items.map((b) => (b.id === id ? { ...b, data: newData } : b)));
	}

	// ── Add block ─────────────────────────────────────────────────────
	let addMenuOpen = false;

	const BLOCK_TYPES: { type: Block['type']; icon: string }[] = [
		{ type: 'text', icon: '📝' },
		{ type: 'price_table', icon: '💰' },
		{ type: 'case_study', icon: '🖼️' },
		{ type: 'team_member', icon: '👤' },
		{ type: 'terms', icon: '📋' }
	];

	const DEFAULT_DATA: Record<Block['type'], Record<string, unknown>> = {
		text: { html: '<p></p>' },
		price_table: { rows: [{ service: '', qty: 1, price: 0 }] },
		case_study: { title: '', description: '', image_url: null },
		team_member: { name: '', role: '', bio: '', photo_url: null },
		terms: { items: [''] }
	};

	function addBlock(type: Block['type']) {
		addMenuOpen = false;
		const newBlock: Block = {
			id: crypto.randomUUID(),
			type,
			order: items.length,
			data: { ...DEFAULT_DATA[type] }
		};
		notify(reindex([...items, newBlock]));
	}

	const TYPE_COLORS: Record<Block['type'], string> = {
		text: 'bg-blue-50 text-blue-600',
		price_table: 'bg-green-50 text-green-600',
		case_study: 'bg-orange-50 text-orange-600',
		team_member: 'bg-purple-50 text-purple-600',
		terms: 'bg-gray-100 text-gray-500'
	};

	// Close add menu on outside click
	function handleOutsideClick(e: MouseEvent) {
		if (addMenuOpen) {
			const target = e.target as HTMLElement;
			if (!target.closest('.add-block-container')) addMenuOpen = false;
		}
	}
</script>

<svelte:window on:click={handleOutsideClick} />

<div class="space-y-3">
	{#if items.length === 0}
		<div class="flex flex-col items-center justify-center py-16 border-2 border-dashed border-gray-200 rounded-xl text-center">
			<p class="text-gray-400 text-sm mb-3">{$_('editor.no_blocks')}</p>
			{#if !readonly}
				<p class="text-xs text-gray-300 mb-4">{$_('editor.no_blocks_hint')}</p>
			{/if}
		</div>
	{:else}
		<section
			use:dndzone={{ items, flipDurationMs, type: 'blocks', dropTargetStyle: {} }}
			on:consider={handleConsider}
			on:finalize={handleFinalize}
			class="space-y-3"
		>
			{#each items as block (block.id)}
				{@const isShadow = block[SHADOW_ITEM_MARKER_PROPERTY_NAME]}
				<div
					class="group relative bg-white rounded-xl border {isShadow
						? 'border-indigo-300 opacity-50'
						: 'border-gray-100 hover:border-gray-200'} transition-colors"
				>
					{#if !readonly}
						<!-- Block header: drag handle + type + actions -->
						<div class="flex items-center gap-2 px-3 pt-2.5 pb-2 border-b border-gray-50">
							<!-- Drag handle -->
							<div
								class="cursor-grab active:cursor-grabbing p-1 rounded text-gray-300 hover:text-gray-500 hover:bg-gray-100 transition-colors flex-shrink-0"
								aria-label={$_('block.drag')}
							>
								<svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
									<path d="M7 2a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 2zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 8zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 7 14zm6-8a2 2 0 1 0-.001-4.001A2 2 0 0 0 13 6zm0 2a2 2 0 1 0 .001 4.001A2 2 0 0 0 13 8zm0 6a2 2 0 1 0 .001 4.001A2 2 0 0 0 13 14z"/>
								</svg>
							</div>

							<!-- Type badge -->
							<span class="text-xs font-medium px-2 py-0.5 rounded-full {TYPE_COLORS[block.type] ?? 'bg-gray-100 text-gray-500'}">
								{$_('block_type.' + block.type)}
							</span>

							<div class="flex-1"></div>

							<!-- Duplicate -->
							<button
								type="button"
								on:click={() => duplicateBlock(block.id)}
								class="p-1.5 rounded text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors text-xs"
								title={$_('block.duplicate')}
							>
								<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/>
								</svg>
							</button>

							<!-- Delete / Confirm delete -->
							{#if confirmDeleteId === block.id}
								<div class="flex items-center gap-1">
									<span class="text-xs text-gray-500">{$_('block.delete_confirm')}</span>
									<button type="button" on:click={() => deleteBlock(block.id)}
										class="text-xs px-2 py-0.5 bg-red-600 text-white rounded hover:bg-red-700 transition-colors">
										{$_('block.yes')}
									</button>
									<button type="button" on:click={cancelDelete}
										class="text-xs px-2 py-0.5 border border-gray-200 rounded hover:bg-gray-50 transition-colors">
										{$_('block.no')}
									</button>
								</div>
							{:else}
								<button
									type="button"
									on:click={() => deleteBlock(block.id)}
									class="p-1.5 rounded text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors"
									title={$_('block.delete')}
								>
									<svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
									</svg>
								</button>
							{/if}
						</div>
					{/if}

					<!-- Block content -->
					<div class="p-4">
						{#if block.type === 'text'}
							<TextBlock
								data={block.data}
								{readonly}
								on:update={(e) => updateBlockData(block.id, e.detail)}
							/>
						{:else if block.type === 'price_table'}
							<PriceTableBlock
								data={block.data}
								{readonly}
								on:update={(e) => updateBlockData(block.id, e.detail)}
							/>
						{:else if block.type === 'case_study'}
							<CaseStudyBlock
								data={block.data}
								{readonly}
								{token}
								{proposalId}
								blockId={block.id}
								on:update={(e) => updateBlockData(block.id, e.detail)}
							/>
						{:else if block.type === 'team_member'}
							<TeamMemberBlock
								data={block.data}
								{readonly}
								{token}
								{proposalId}
								blockId={block.id}
								on:update={(e) => updateBlockData(block.id, e.detail)}
							/>
						{:else if block.type === 'terms'}
							<TermsBlock
								data={block.data}
								{readonly}
								on:update={(e) => updateBlockData(block.id, e.detail)}
							/>
						{/if}
					</div>
				</div>
			{/each}
		</section>
	{/if}

	<!-- Add block button -->
	{#if !readonly}
		<div class="add-block-container relative">
			<button
				type="button"
				on:click={() => (addMenuOpen = !addMenuOpen)}
				class="w-full flex items-center justify-center gap-2 py-2.5 border-2 border-dashed border-gray-200 rounded-xl text-sm text-gray-400 hover:border-indigo-300 hover:text-indigo-500 hover:bg-indigo-50/30 transition-all"
			>
				<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
					<path d="M12 4v16m8-8H4"/>
				</svg>
				{$_('editor.add_block')}
			</button>

			{#if addMenuOpen}
				<div class="absolute bottom-full mb-1 left-1/2 -translate-x-1/2 bg-white rounded-xl shadow-lg border border-gray-100 p-1.5 flex gap-1 z-10">
					{#each BLOCK_TYPES as bt}
						<button
							type="button"
							on:click={() => addBlock(bt.type)}
							class="flex flex-col items-center gap-1 px-3 py-2 rounded-lg hover:bg-indigo-50 text-gray-600 hover:text-indigo-700 transition-colors min-w-[64px]"
							title={$_('block_type.' + bt.type)}
						>
							<span class="text-xl">{bt.icon}</span>
							<span class="text-[10px] font-medium leading-none">{$_('block_type.' + bt.type)}</span>
						</button>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>
