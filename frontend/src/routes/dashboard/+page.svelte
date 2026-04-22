<script lang="ts">
	import { goto } from '$app/navigation';
	import { _ } from 'svelte-i18n';
	import { proposals, HttpError } from '$lib/api';
	import { authStore } from '$lib/stores/auth';
	import type { Proposal } from '$lib/api';
	import TemplatePickerModal from '$lib/components/TemplatePickerModal.svelte';

	let items: Proposal[] = [];
	let total = 0;
	let loading = true;
	let error = '';
	let planUsage = { used: 0, limit: 3 as number | null };

	let search = '';
	let statusFilter = 'all';
	let sortBy = 'updated_at';
	let debounceTimer: ReturnType<typeof setTimeout>;

	const STATUSES = ['all', 'draft', 'sent', 'opened', 'approved', 'rejected'];

	const STATUS_COLORS: Record<string, string> = {
		draft:    'bg-gray-100 text-gray-600',
		sent:     'bg-blue-50 text-blue-700',
		opened:   'bg-yellow-50 text-yellow-700',
		approved: 'bg-green-50 text-green-700',
		rejected: 'bg-red-50 text-red-700'
	};

	let deleteConfirmId: string | null = null;
	let actionInProgress: string | null = null;

	let showPicker = false;
	let showPaywallModal = false;
	let creating = false;
	let createError = '';

	async function loadProposals() {
		if (!$authStore.accessToken) return;
		loading = true;
		error = '';
		try {
			const params: Record<string, string> = { sort: sortBy };
			if (statusFilter !== 'all') params.status = statusFilter;
			if (search.trim()) params.search = search.trim();
			const result = await proposals.list($authStore.accessToken, params);
			items = result.items;
			total = result.total;
			planUsage = result.plan_usage;
		} catch {
			error = $_('proposals.load_failed');
		} finally {
			loading = false;
		}
	}

	function onSearchInput() {
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(loadProposals, 300);
	}

	// Track when auth becomes ready (token goes from null → value).
	// This prevents a double-load when auth.refresh() rotates the token after login.
	let _authReady = false;
	$: if ($authStore.accessToken && !_authReady) _authReady = true;

	// Reload proposals when filter/sort changes, or on initial auth ready.
	$: if (_authReady) {
		statusFilter;
		sortBy;
		loadProposals();
	}

	function handleNewProposal() {
		if (planUsage.limit !== null && planUsage.used >= planUsage.limit) {
			showPaywallModal = true;
			return;
		}
		showPicker = true;
	}

	async function handleCreate(e: CustomEvent<{ templateId: string | null; title: string; clientName: string }>) {
		if (!$authStore.accessToken) return;
		const { templateId, title, clientName } = e.detail;
		showPicker = false;
		creating = true;
		createError = '';
		try {
			const { id } = await proposals.create($authStore.accessToken, {
				title: title || undefined,
				client_name: clientName || undefined,
				template_id: templateId ?? undefined
			});
			goto(`/dashboard/proposals/${id}`);
		} catch (err) {
			if (err instanceof HttpError && err.status === 402) {
				showPaywallModal = true;
			} else {
				createError = $_('proposals.create_failed');
			}
		} finally {
			creating = false;
		}
	}

	async function handleDuplicate(e: MouseEvent, id: string) {
		e.preventDefault();
		if (!$authStore.accessToken || actionInProgress) return;
		actionInProgress = id;
		try {
			const { id: newId } = await proposals.duplicate($authStore.accessToken, id);
			goto(`/dashboard/proposals/${newId}`);
		} catch {
			actionInProgress = null;
		}
	}

	function requestDelete(e: MouseEvent, id: string) {
		e.preventDefault();
		deleteConfirmId = id;
	}

	async function confirmDelete(id: string) {
		if (!$authStore.accessToken || actionInProgress) return;
		actionInProgress = id;
		deleteConfirmId = null;
		try {
			await proposals.delete($authStore.accessToken, id);
			items = items.filter((p) => p.id !== id);
			total -= 1;
		} catch { /* ignore */ } finally {
			actionInProgress = null;
		}
	}

	function cancelDelete() {
		deleteConfirmId = null;
	}

	function formatDate(date: string) {
		return new Date(date).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
	}

	$: atLimit = planUsage.limit !== null && planUsage.used >= planUsage.limit;

	// Status filter label lookup via i18n
	function statusLabel(s: string): string {
		if (s === 'all') return $_('proposals.filter.all');
		return $_(`proposals.filter.${s}`);
	}
</script>

<svelte:head>
	<title>{$_('proposals.page_title')}</title>
</svelte:head>

{#if showPicker}
	<TemplatePickerModal
		planUsed={planUsage.used}
		planLimit={planUsage.limit}
		on:create={handleCreate}
		on:close={() => (showPicker = false)}
	/>
{/if}

{#if showPaywallModal}
	<div
		data-testid="plan-limit-modal"
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
		role="dialog"
		aria-modal="true"
	>
		<div class="bg-white rounded-2xl shadow-xl p-8 max-w-sm w-full">
			<div class="text-3xl mb-3">✨</div>
			<h2 class="text-lg font-bold text-gray-900 mb-2">{$_('proposals.limit.title')}</h2>
			<p class="text-sm text-gray-500 mb-6">{$_('proposals.limit.description')}</p>
			<div class="flex gap-3">
				<a
					href="/dashboard/billing"
					class="flex-1 text-center px-4 py-2 bg-indigo-600 text-white text-sm font-semibold rounded-lg hover:bg-indigo-700 transition-colors"
				>{$_('proposals.limit.cta')}</a>
				<button
					type="button"
					class="flex-1 px-4 py-2 border border-gray-200 text-sm font-medium rounded-lg hover:bg-gray-50 transition-colors"
					on:click={() => (showPaywallModal = false)}
				>{$_('proposals.limit.maybe_later')}</button>
			</div>
		</div>
	</div>
{/if}

<div class="max-w-5xl mx-auto" aria-hidden={showPicker ? 'true' : null}>
	<div class="flex items-center justify-between mb-6">
		<div>
			<h1 class="text-2xl font-bold text-gray-900">{$_('proposals.heading')}</h1>
			{#if planUsage.limit !== null}
				<p data-testid="plan-usage" class="text-sm mt-1 {atLimit ? 'text-amber-600 font-medium' : 'text-gray-500'}">
					{planUsage.used} / {planUsage.limit}{$_('proposals.free_suffix')}
					{#if atLimit}· <a href="/dashboard/billing" class="underline hover:no-underline">{$_('settings.plan.upgrade')}</a>{/if}
				</p>
			{/if}
		</div>
		<button
			data-testid="new-proposal-btn"
			on:click={handleNewProposal}
			disabled={creating}
			class="flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
		>
			{#if creating}
				<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"/>
				</svg>
				{$_('proposals.creating')}
			{:else}
				<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
					<path d="M12 4v16m8-8H4"/>
				</svg>
				{$_('proposals.new')}
			{/if}
		</button>
	</div>

	{#if createError}
		<div class="mb-4 px-4 py-3 bg-red-50 border border-red-200 rounded-xl text-sm text-red-700">
			{createError}
		</div>
	{/if}

	<div class="flex flex-col sm:flex-row gap-3 mb-5">
		<div class="relative flex-1">
			<svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
				<path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-4.35-4.35M17 11A6 6 0 1 1 5 11a6 6 0 0 1 12 0z"/>
			</svg>
			<input
				type="text"
				bind:value={search}
				on:input={onSearchInput}
				placeholder={$_('proposals.search_placeholder')}
				class="w-full pl-9 pr-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
			/>
		</div>

		<div class="flex gap-1 flex-wrap">
			{#each STATUSES as s}
				<button
					aria-label={statusLabel(s)}
					data-label={statusLabel(s)}
					on:click={() => { statusFilter = s; }}
					class="filter-chip px-3 py-1.5 text-xs font-medium rounded-lg transition-colors {statusFilter === s
						? 'bg-indigo-600 text-white'
						: 'bg-gray-100 text-gray-600 hover:bg-gray-200'}"
				></button>
			{/each}
		</div>

		<select
			bind:value={sortBy}
			class="text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500 bg-white"
		>
			<option value="updated_at">{$_('proposals.sort.updated_at')}</option>
			<option value="created_at">{$_('proposals.sort.created_at')}</option>
			<option value="last_opened_at">{$_('proposals.sort.last_opened')}</option>
			<option value="title">{$_('proposals.sort.title')}</option>
		</select>
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
			{#if search || statusFilter !== 'all'}
				<h3 class="text-lg font-medium text-gray-900 mb-2">{$_('proposals.no_match')}</h3>
				<button
					on:click={() => { search = ''; statusFilter = 'all'; loadProposals(); }}
					class="text-sm text-indigo-600 hover:underline"
				>{$_('proposals.clear_filters')}</button>
			{:else}
				<h3 class="text-lg font-medium text-gray-900 mb-2">{$_('proposals.empty.heading')}</h3>
				<p class="text-gray-500 mb-6">{$_('proposals.empty.description')}</p>
				<button
					on:click={handleNewProposal}
					class="px-4 py-2 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 transition-colors"
				>{$_('proposals.empty.cta')}</button>
			{/if}
		</div>
	{:else}
		<div class="space-y-2">
			{#each items as proposal (proposal.id)}
				{#if deleteConfirmId === proposal.id}
					<div class="bg-red-50 border border-red-200 rounded-xl px-4 py-3 flex items-center justify-between gap-4">
						<p class="text-sm text-red-700">
							{$_('proposals.delete_confirm', { title: proposal.title || $_('proposals.untitled') })}
						</p>
						<div class="flex gap-2 shrink-0">
							<button
								on:click={() => confirmDelete(proposal.id)}
								class="px-3 py-1.5 text-sm font-medium bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
							>{$_('proposals.delete')}</button>
							<button
								on:click={cancelDelete}
								class="px-3 py-1.5 text-sm font-medium border border-gray-200 rounded-lg hover:bg-white transition-colors"
							>{$_('proposals.cancel')}</button>
						</div>
					</div>
				{:else}
					<div class="group relative bg-white rounded-xl border border-gray-100 hover:border-indigo-200 hover:shadow-sm transition-all">
						<a
							href="/dashboard/proposals/{proposal.id}"
							class="block p-4"
							aria-label="Open {proposal.title || $_('proposals.untitled')}"
						>
							<div class="flex items-center justify-between gap-4">
								<div class="flex-1 min-w-0">
									<div class="flex items-center gap-3">
										<h3 class="font-medium text-gray-900 truncate">
											{proposal.title || $_('proposals.untitled')}
										</h3>
										<span class="shrink-0 text-xs px-2 py-0.5 rounded-full font-medium {STATUS_COLORS[proposal.status] ?? 'bg-gray-100 text-gray-600'}">
											{$_(`proposals.status.${proposal.status}`) || proposal.status}
										</span>
									</div>
									{#if proposal.client_name}
										<p class="text-sm text-gray-500 mt-0.5 truncate">{proposal.client_name}</p>
									{/if}
								</div>

								<div class="text-right shrink-0">
									{#if proposal.open_count > 0}
										<p class="text-sm font-medium text-gray-700">
											{$_('proposals.opens', { count: proposal.open_count })}
										</p>
									{/if}
									<p class="text-xs text-gray-400">{formatDate(proposal.updated_at)}</p>
								</div>
							</div>
						</a>

						<div class="absolute right-3 top-1/2 -translate-y-1/2 hidden group-hover:flex items-center gap-1 bg-white pl-2">
							<button
								on:click={(e) => handleDuplicate(e, proposal.id)}
								disabled={actionInProgress === proposal.id}
								class="p-1.5 rounded-lg text-gray-400 hover:text-indigo-600 hover:bg-indigo-50 transition-colors disabled:opacity-40"
								aria-label="Duplicate proposal"
							>
								<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/>
								</svg>
							</button>

							<button
								on:click={(e) => requestDelete(e, proposal.id)}
								disabled={actionInProgress === proposal.id}
								class="p-1.5 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 transition-colors disabled:opacity-40"
								aria-label="Delete proposal"
							>
								<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
								</svg>
							</button>
						</div>
					</div>
				{/if}
			{/each}
		</div>

		{#if total > items.length}
			<p class="text-center text-sm text-gray-400 mt-6">
				{$_('proposals.showing', { count: items.length, total })}
			</p>
		{/if}
	{/if}
</div>


<style>
	/* Filter chip labels rendered via CSS to keep text nodes empty,
	   preventing getByText('Draft') from matching both filter button and status badge. */
	.filter-chip::before {
		content: attr(data-label);
	}
</style>
