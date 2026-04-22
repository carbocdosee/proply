<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { proposals, HttpError } from '$lib/api';
	import type { Analytics } from '$lib/api';
	import { authStore } from '$lib/stores/auth';
	import type { Proposal, Block } from '$lib/api';
	import BlockEditor from '$lib/components/BlockEditor.svelte';

	let proposal: Proposal | null = null;
	let loading = true;
	let notFound = false;
	let saveStatus: 'idle' | 'saving' | 'saved' | 'error' = 'idle';
	let saveTimer: ReturnType<typeof setTimeout>;
	let publishError = '';
	let linkCopied = false;

	$: id = $page.params.id;

	let proposalLoaded = false;
	$: if ($authStore.accessToken && id && !proposalLoaded) loadProposal();

	async function loadProposal() {
		if (!$authStore.accessToken) return;
		proposalLoaded = true;
		try {
			proposal = await proposals.get($authStore.accessToken, id);
		} catch {
			notFound = true;
		} finally {
			loading = false;
		}
	}

	onDestroy(() => clearTimeout(saveTimer));

	// Debounced auto-save (2 seconds after last change)
	function scheduleAutoSave() {
		clearTimeout(saveTimer);
		saveStatus = 'saving';
		saveTimer = setTimeout(doSave, 2000);
	}

	async function doSave() {
		if (!proposal || !$authStore.accessToken) return;
		try {
			await proposals.update($authStore.accessToken, proposal.id, {
				title: proposal.title,
				client_name: proposal.client_name,
				blocks: proposal.blocks
			});
			saveStatus = 'saved';
			setTimeout(() => (saveStatus = 'idle'), 2500);
		} catch {
			saveStatus = 'error';
			setTimeout(() => (saveStatus = 'idle'), 3000);
		}
	}

	function handleBlocksChange(e: CustomEvent<Block[]>) {
		if (!proposal) return;
		proposal = { ...proposal, blocks: e.detail };
		scheduleAutoSave();
	}

	async function handlePublish() {
		if (!proposal || !$authStore.accessToken) return;
		publishError = '';
		try {
			const { slug } = await proposals.publish($authStore.accessToken, proposal.id);
			proposal = { ...proposal, slug, slug_active: true, status: 'sent' };
		} catch (err) {
			if (err instanceof HttpError) {
				switch (err.status) {
					case 402: publishError = $_('editor.limit_reached'); break;
					case 409: publishError = $_('editor.already_published'); break;
					case 403: publishError = $_('editor.verify_email'); break;
					default: publishError = $_('editor.publish_failed');
				}
			}
		}
	}

	async function handleRevoke() {
		if (!proposal || !$authStore.accessToken) return;
		try {
			await proposals.revoke($authStore.accessToken, proposal.id);
			proposal = { ...proposal, slug_active: false, status: 'draft' };
		} catch { /* ignore */ }
	}

	async function copyLink() {
		if (!proposal?.slug) return;
		const url = `${window.location.origin}/p/${proposal.slug}`;
		await navigator.clipboard.writeText(url);
		linkCopied = true;
		setTimeout(() => (linkCopied = false), 2000);
	}

	$: isApproved = proposal?.status === 'approved';
	$: isPublished = !!(proposal?.slug && proposal.slug_active);

	// Analytics
	let analytics: Analytics | null = null;
	let analyticsLoading = false;
	let showAnalytics = false;
	let analyticsPaywallClicked = false;

	async function loadAnalytics() {
		if (!proposal || !$authStore.accessToken) return;
		analyticsLoading = true;
		try {
			analytics = await proposals.getAnalytics($authStore.accessToken, proposal.id);
		} catch { /* ignore */ } finally {
			analyticsLoading = false;
		}
	}

	function toggleAnalytics() {
		showAnalytics = !showAnalytics;
		if (showAnalytics && !analytics) loadAnalytics();
	}

	function fmtDate(iso?: string): string {
		if (!iso) return '—';
		return new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });
	}

	function fmtSec(sec: number): string {
		if (sec < 60) return `${sec}s`;
		return `${Math.floor(sec / 60)}m ${sec % 60}s`;
	}

	$: maxBlockSec = analytics?.block_stats
		? Math.max(1, ...analytics.block_stats.map((b) => b.duration_sec))
		: 1;
</script>

<svelte:head>
	<title>{$_('editor.page_title', { values: { title: proposal?.title || 'Proposal' } })}</title>
</svelte:head>

{#if loading}
	<div class="flex items-center justify-center h-64">
		<div class="w-8 h-8 border-4 border-indigo-200 border-t-indigo-600 rounded-full animate-spin"></div>
	</div>
{:else if notFound}
	<div class="text-center py-20">
		<p class="text-gray-500">{$_('editor.not_found')}</p>
		<a href="/dashboard" class="mt-4 inline-block text-indigo-600 hover:underline text-sm">{$_('editor.back')}</a>
	</div>
{:else if proposal}
	<!-- Toolbar -->
	<div class="flex items-center justify-between mb-5 gap-4">
		<div class="flex items-center gap-3 min-w-0">
			<a href="/dashboard" class="text-gray-400 hover:text-gray-600 transition-colors flex-shrink-0" aria-label="Back">
				<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path d="M10 19l-7-7m0 0l7-7m-7 7h18"/>
				</svg>
			</a>
			<input
				type="text"
				bind:value={proposal.title}
				on:input={scheduleAutoSave}
				disabled={isApproved}
				class="text-xl font-semibold text-gray-900 bg-transparent border-none outline-none focus:ring-0 p-0 min-w-0 truncate disabled:cursor-default"
				placeholder={$_('editor.title_placeholder')}
			/>
			<span class="text-xs flex-shrink-0 {saveStatus === 'saving' ? 'text-gray-400' : saveStatus === 'saved' ? 'text-green-500' : saveStatus === 'error' ? 'text-red-500' : 'invisible'}">
				{saveStatus === 'saving' ? $_('editor.saving') : saveStatus === 'saved' ? $_('editor.saved') : $_('editor.save_failed')}
			</span>
		</div>

		<div class="flex items-center gap-2 flex-shrink-0">
			{#if isPublished}
				<button
					on:click={copyLink}
					class="flex items-center gap-1.5 px-3 py-1.5 text-sm border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors"
				>
					{#if linkCopied}
						<svg class="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path d="M5 13l4 4L19 7"/>
						</svg>
						{$_('editor.copied')}
					{:else}
						<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/>
						</svg>
						{$_('editor.copy_link')}
					{/if}
				</button>
				<a
					href="/p/{proposal.slug}"
					target="_blank"
					rel="noopener noreferrer"
					class="flex items-center gap-1.5 px-3 py-1.5 text-sm text-gray-500 hover:text-indigo-600 border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors"
				>
					<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"/>
					</svg>
					{$_('editor.preview')}
				</a>
				{#if !isApproved}
					<button
						on:click={handleRevoke}
						class="px-3 py-1.5 text-sm text-gray-500 border border-gray-200 rounded-lg hover:bg-gray-50 hover:text-red-600 transition-colors"
						title="Deactivate link"
					>
						{$_('editor.revoke')}
					</button>
				{/if}
			{:else if !isApproved}
				<button
					on:click={handlePublish}
					class="px-4 py-1.5 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors"
				>
					{$_('editor.publish')}
				</button>
			{/if}
		</div>
	</div>

	{#if publishError}
		<div class="mb-4 px-4 py-3 bg-red-50 border border-red-200 rounded-xl text-sm text-red-700">
			{publishError}
		</div>
	{/if}

	{#if isApproved}
		<div class="mb-4 px-4 py-3 bg-green-50 border border-green-200 rounded-xl text-sm text-green-800 flex items-center gap-2">
			<svg class="w-4 h-4 text-green-500 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
				<path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
			</svg>
			{$_('editor.read_only')}
		</div>
	{/if}

	<!-- Metadata bar -->
	<div class="bg-white rounded-xl border border-gray-100 p-4 mb-5 flex items-center gap-5 flex-wrap">
		<div>
			<p class="text-xs text-gray-400 mb-0.5">{$_('editor.client_label')}</p>
			<input
				type="text"
				bind:value={proposal.client_name}
				on:input={scheduleAutoSave}
				disabled={isApproved}
				placeholder={$_('editor.client_placeholder')}
				class="text-sm text-gray-900 bg-transparent border-none outline-none focus:ring-0 p-0 disabled:cursor-default"
			/>
		</div>
		<div class="w-px h-7 bg-gray-100"></div>
		<div>
			<p class="text-xs text-gray-400 mb-0.5">{$_('editor.status_label')}</p>
			<span class="text-sm font-medium text-gray-900">{$_('proposals.filter.' + proposal.status)}</span>
		</div>
		{#if proposal.open_count > 0}
			<div class="w-px h-7 bg-gray-100"></div>
			<div>
				<p class="text-xs text-gray-400 mb-0.5">{$_('editor.opens_label')}</p>
				<span class="text-sm font-medium text-gray-900">{proposal.open_count}</span>
			</div>
		{/if}
		{#if proposal.approved_at}
			<div class="w-px h-7 bg-gray-100"></div>
			<div>
				<p class="text-xs text-gray-400 mb-0.5">{$_('editor.approved_label')}</p>
				<span class="text-sm font-medium text-gray-900">
					{new Date(proposal.approved_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}
				</span>
			</div>
		{/if}
	</div>

	<!-- Block editor -->
	<BlockEditor
		blocks={proposal.blocks}
		token={$authStore.accessToken ?? ''}
		proposalId={proposal.id}
		readonly={isApproved}
		on:change={handleBlocksChange}
	/>

	<!-- Analytics panel -->
	<div class="mt-6 bg-white rounded-xl border border-gray-100">
		<button
			on:click={toggleAnalytics}
			class="w-full flex items-center justify-between px-5 py-4 text-left hover:bg-gray-50 transition-colors rounded-xl"
		>
			<span class="flex items-center gap-2 font-medium text-gray-800">
				<svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/>
				</svg>
				{$_('analytics.heading')}
			</span>
			<svg class="w-4 h-4 text-gray-400 transition-transform {showAnalytics ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
				<path d="M19 9l-7 7-7-7"/>
			</svg>
		</button>

		{#if showAnalytics}
			<div class="px-5 pb-5 border-t border-gray-100">
				{#if analyticsLoading}
					<div class="flex items-center justify-center py-8">
						<div class="w-6 h-6 border-4 border-indigo-200 border-t-indigo-600 rounded-full animate-spin"></div>
					</div>
				{:else if analytics}
					<!-- Summary row (visible on all plans) -->
					<div class="grid grid-cols-2 sm:grid-cols-4 gap-4 py-4">
						<div>
							<p class="text-xs text-gray-400 mb-0.5">{$_('analytics.opens')}</p>
							<p class="text-2xl font-bold text-gray-900">{analytics.open_count}</p>
						</div>
						<div>
							<p class="text-xs text-gray-400 mb-0.5">{$_('analytics.total_time')}</p>
							<p class="text-2xl font-bold text-gray-900">{fmtSec(analytics.total_duration_sec)}</p>
						</div>
						<div>
							<p class="text-xs text-gray-400 mb-0.5">{$_('analytics.first_opened')}</p>
							<p class="text-sm font-medium text-gray-900">{fmtDate(analytics.first_opened_at)}</p>
						</div>
						<div>
							<p class="text-xs text-gray-400 mb-0.5">{$_('analytics.last_opened')}</p>
							<p class="text-sm font-medium text-gray-900">{fmtDate(analytics.last_opened_at)}</p>
						</div>
					</div>

					<!-- Block stats (Pro) or paywall (Free) -->
					{#if analytics.plan_gate}
						<!-- Free plan — paywall -->
						<div
							class="relative mt-2 rounded-xl overflow-hidden cursor-pointer"
							role="button"
							tabindex="0"
							on:click={() => analyticsPaywallClicked = true}
							on:keydown={(e) => e.key === 'Enter' && (analyticsPaywallClicked = true)}
						>
							<!-- Blurred preview -->
							<div class="blur-sm pointer-events-none select-none space-y-2 py-2">
								{#each [70, 45, 30, 20] as pct}
									<div class="flex items-center gap-3">
										<span class="text-xs text-gray-400 w-24 truncate">Block {pct}</span>
										<div class="flex-1 h-3 bg-gray-100 rounded-full">
											<div class="h-3 bg-indigo-300 rounded-full" style="width: {pct}%"></div>
										</div>
										<span class="text-xs text-gray-400 w-10 text-right">{pct}s</span>
									</div>
								{/each}
							</div>
							<!-- Overlay -->
							<div class="absolute inset-0 flex flex-col items-center justify-center bg-white/70 backdrop-blur-[2px]">
								<svg class="w-6 h-6 text-indigo-500 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
								</svg>
								<p class="text-sm font-semibold text-gray-900">{$_('analytics.pro_only')}</p>
								<a href="/dashboard/billing" class="mt-2 text-xs text-indigo-600 hover:underline">{$_('analytics.upgrade_cta')}</a>
							</div>
						</div>
					{:else if analytics.block_stats.length > 0}
						<!-- Pro — horizontal bar chart -->
						<div class="mt-2 space-y-2">
							<p class="text-xs font-medium text-gray-500 uppercase tracking-wide mb-3">{$_('analytics.block_title')}</p>
							{#each analytics.block_stats as block}
								<div class="flex items-center gap-3">
									<span class="text-xs text-gray-500 w-28 truncate capitalize">{block.block_type}</span>
									<div class="flex-1 h-3 bg-gray-100 rounded-full overflow-hidden">
										<div
											class="h-3 bg-indigo-500 rounded-full transition-all"
											style="width: {Math.round((block.duration_sec / maxBlockSec) * 100)}%"
										></div>
									</div>
									<span class="text-xs text-gray-500 w-12 text-right">{fmtSec(block.duration_sec)}</span>
								</div>
							{/each}
						</div>
					{:else}
						<p class="text-sm text-gray-400 py-3">{$_('analytics.no_data')}</p>
					{/if}
				{/if}
			</div>
		{/if}
	</div>
{/if}
