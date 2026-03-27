<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { proposals } from '$lib/api';
	import { authStore, isEmailVerified } from '$lib/stores/auth';
	import type { Proposal } from '$lib/api';

	let proposal: Proposal | null = null;
	let loading = true;
	let saveStatus: 'idle' | 'saving' | 'saved' = 'idle';
	let saveTimer: ReturnType<typeof setTimeout>;

	$: id = $page.params.id;

	onMount(async () => {
		if (!$authStore.accessToken) return;
		try {
			proposal = await proposals.get($authStore.accessToken, id);
		} catch {
			// TODO: handle 404/403
		} finally {
			loading = false;
		}
	});

	onDestroy(() => clearTimeout(saveTimer));

	// Debounced auto-save (2 seconds after last change)
	function scheduleAutoSave() {
		clearTimeout(saveTimer);
		saveStatus = 'saving';
		saveTimer = setTimeout(async () => {
			if (!proposal || !$authStore.accessToken) return;
			try {
				await proposals.update($authStore.accessToken, proposal.id, {
					title: proposal.title,
					client_name: proposal.client_name,
					blocks: proposal.blocks
				});
				saveStatus = 'saved';
				setTimeout(() => (saveStatus = 'idle'), 2000);
			} catch {
				saveStatus = 'idle';
			}
		}, 2000);
	}

	async function handlePublish() {
		if (!proposal || !$authStore.accessToken) return;
		try {
			const { slug } = await proposals.publish($authStore.accessToken, proposal.id);
			proposal = { ...proposal, slug, status: 'sent' };
		} catch (e: unknown) {
			if (e instanceof Error && e.message.includes('PLAN_LIMIT')) {
				alert('Free plan limit reached. Please upgrade to publish more proposals.');
			} else if (e instanceof Error && e.message.includes('ALREADY_PUBLISHED')) {
				alert('This proposal is already published.');
			} else if (e instanceof Error && e.message.includes('EMAIL_NOT_VERIFIED')) {
				alert('Please verify your email before publishing a proposal.');
			}
		}
	}

	function copyLink() {
		if (!proposal?.slug) return;
		const url = `${window.location.origin}/p/${proposal.slug}`;
		navigator.clipboard.writeText(url);
	}
</script>

<svelte:head>
	<title>{proposal?.title || 'Proposal'} — Proply</title>
</svelte:head>

{#if loading}
	<div class="flex items-center justify-center h-64">
		<div class="w-8 h-8 border-4 border-indigo-200 border-t-indigo-600 rounded-full animate-spin"></div>
	</div>
{:else if proposal}
	<!-- Editor toolbar -->
	<div class="flex items-center justify-between mb-6">
		<div class="flex items-center gap-3">
			<a href="/dashboard" class="text-gray-400 hover:text-gray-600 transition-colors">
				<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path d="M10 19l-7-7m0 0l7-7m-7 7h18"/>
				</svg>
			</a>
			<input
				type="text"
				bind:value={proposal.title}
				on:input={scheduleAutoSave}
				class="text-xl font-semibold text-gray-900 bg-transparent border-none outline-none focus:ring-0 p-0 w-64"
				placeholder="Proposal title"
			/>
			{#if saveStatus === 'saving'}
				<span class="text-xs text-gray-400">Saving...</span>
			{:else if saveStatus === 'saved'}
				<span class="text-xs text-green-500">Saved</span>
			{/if}
		</div>

		<div class="flex items-center gap-3">
			{#if proposal.slug}
				<button
					on:click={copyLink}
					class="flex items-center gap-2 px-3 py-1.5 text-sm border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors"
				>
					<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/>
					</svg>
					Copy link
				</button>
				<a
					href="/p/{proposal.slug}"
					target="_blank"
					rel="noopener noreferrer"
					class="text-sm text-gray-500 hover:text-indigo-600 transition-colors"
				>
					Preview ↗
				</a>
			{:else}
				<button
					on:click={handlePublish}
					class="px-4 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors"
				>
					Publish
				</button>
			{/if}
		</div>
	</div>

	<!-- Proposal metadata -->
	<div class="bg-white rounded-xl border border-gray-100 p-4 mb-6 flex items-center gap-6">
		<div>
			<label class="block text-xs text-gray-500 mb-1">Client</label>
			<input
				type="text"
				bind:value={proposal.client_name}
				on:input={scheduleAutoSave}
				placeholder="Client name"
				class="text-sm text-gray-900 bg-transparent border-none outline-none focus:ring-0 p-0"
			/>
		</div>
		<div class="w-px h-8 bg-gray-100"></div>
		<div>
			<label class="block text-xs text-gray-500 mb-1">Status</label>
			<span class="text-sm font-medium text-gray-900 capitalize">{proposal.status}</span>
		</div>
		{#if proposal.open_count > 0}
			<div class="w-px h-8 bg-gray-100"></div>
			<div>
				<label class="block text-xs text-gray-500 mb-1">Opens</label>
				<span class="text-sm font-medium text-gray-900">{proposal.open_count}</span>
			</div>
		{/if}
	</div>

	<!-- Blocks editor placeholder -->
	<div class="bg-white rounded-xl border border-gray-100 p-8 text-center">
		<p class="text-gray-400 text-sm">
			Block editor (TipTap + svelte-dnd-action) — to be implemented
		</p>
		<p class="text-xs text-gray-300 mt-1">Blocks: {proposal.blocks.length}</p>
	</div>
{/if}
