<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { proposals } from '$lib/api';
	import { authStore } from '$lib/stores/auth';
	import type { Proposal } from '$lib/api';

	let items: Proposal[] = [];
	let total = 0;
	let loading = true;
	let error = '';
	let planUsage = { used: 0, limit: 3 as number | null };

	const STATUS_COLORS: Record<string, string> = {
		draft: 'bg-gray-100 text-gray-600',
		sent: 'bg-blue-50 text-blue-700',
		opened: 'bg-yellow-50 text-yellow-700',
		approved: 'bg-green-50 text-green-700',
		rejected: 'bg-red-50 text-red-700'
	};

	onMount(async () => {
		await loadProposals();
	});

	async function loadProposals() {
		if (!$authStore.accessToken) return;
		loading = true;
		try {
			const result = await proposals.list($authStore.accessToken);
			items = result.items;
			total = result.total;
			planUsage = result.plan_usage;
		} catch (e) {
			error = 'Failed to load proposals';
		} finally {
			loading = false;
		}
	}

	async function createProposal() {
		if (!$authStore.accessToken) return;
		try {
			const { id } = await proposals.create($authStore.accessToken, {});
			goto(`/dashboard/proposals/${id}`);
		} catch (e: unknown) {
			if (e instanceof Error && e.message.includes('PLAN_LIMIT')) {
				alert('You have reached the free plan limit of 3 published proposals. Please upgrade.');
			}
		}
	}

	function formatDate(date: string) {
		return new Date(date).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
	}
</script>

<svelte:head>
	<title>Proposals — Proply</title>
</svelte:head>

<div class="max-w-5xl mx-auto">
	<div class="flex items-center justify-between mb-8">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">Proposals</h1>
			{#if planUsage.limit !== null}
				<p class="text-sm text-gray-500 mt-1">
					{planUsage.used} / {planUsage.limit} published on free plan
				</p>
			{/if}
		</div>
		<button
			on:click={createProposal}
			class="flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 transition-colors"
		>
			<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
				<path d="M12 4v16m8-8H4"/>
			</svg>
			New proposal
		</button>
	</div>

	{#if loading}
		<div class="space-y-3">
			{#each [1, 2, 3] as _}
				<div class="h-20 bg-gray-100 rounded-xl animate-pulse"></div>
			{/each}
		</div>
	{:else if error}
		<div class="text-center py-12 text-red-500">{error}</div>
	{:else if items.length === 0}
		<div class="text-center py-20">
			<div class="w-16 h-16 bg-indigo-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
				<svg class="w-8 h-8 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
					<path d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
				</svg>
			</div>
			<h3 class="text-lg font-medium text-gray-900 mb-2">No proposals yet</h3>
			<p class="text-gray-500 mb-6">Create your first commercial proposal</p>
			<button
				on:click={createProposal}
				class="px-4 py-2 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 transition-colors"
			>
				Create proposal
			</button>
		</div>
	{:else}
		<div class="space-y-3">
			{#each items as proposal}
				<a
					href="/dashboard/proposals/{proposal.id}"
					class="block bg-white rounded-xl border border-gray-100 p-4 hover:border-indigo-200 hover:shadow-sm transition-all"
				>
					<div class="flex items-center justify-between">
						<div class="flex-1 min-w-0">
							<div class="flex items-center gap-3">
								<h3 class="font-medium text-gray-900 truncate">
									{proposal.title || 'Untitled'}
								</h3>
								<span class="shrink-0 text-xs px-2 py-0.5 rounded-full font-medium {STATUS_COLORS[proposal.status] ?? 'bg-gray-100 text-gray-600'}">
									{proposal.status}
								</span>
							</div>
							{#if proposal.client_name}
								<p class="text-sm text-gray-500 mt-0.5">{proposal.client_name}</p>
							{/if}
						</div>
						<div class="text-right ml-4 shrink-0">
							{#if proposal.open_count > 0}
								<p class="text-sm font-medium text-gray-900">{proposal.open_count} opens</p>
							{/if}
							<p class="text-xs text-gray-400">{formatDate(proposal.updated_at)}</p>
						</div>
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>
