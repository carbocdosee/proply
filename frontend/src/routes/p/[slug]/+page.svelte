<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { PageData } from './$types';
	import { browser } from '$app/environment';
	import { goto } from '$app/navigation';
	import { _ } from 'svelte-i18n';
	import { setLocale } from '$lib/i18n';
	import { publicApi, trackingApi } from '$lib/api';
	import type { PublicProposal } from '$lib/api';

	export let data: PageData;

	let proposal: PublicProposal | null = data.proposal;
	let passwordRequired = data.passwordRequired;
	let revoked = data.revoked;
	let password = '';
	let passwordError = '';
	let passwordLoading = false;

	// Approval state
	let showApproveModal = false;
	let clientEmail = '';
	let approveLoading = false;
	let approveError = '';
	let approved = proposal?.status === 'approved';

	interface PriceRow { service: string; qty: number; price: number }
	function priceRows(d: Record<string, unknown>): PriceRow[] {
		return (d.rows as PriceRow[]) ?? [];
	}
	function priceTotal(d: Record<string, unknown>): number {
		return priceRows(d).reduce((sum, r) => sum + r.qty * r.price, 0);
	}
	function termsItems(d: Record<string, unknown>): string[] {
		return (d.items as string[]) ?? [];
	}
	function fmtNumber(n: number): string {
		return new Intl.NumberFormat().format(n);
	}

	// Apply branding CSS variables — browser-only to avoid SSR crash
	$: if (browser && proposal) {
		document.documentElement.style.setProperty('--color-primary', proposal.primary_color);
		document.documentElement.style.setProperty('--color-accent', proposal.accent_color);
	}

	async function submitPassword() {
		passwordError = '';
		passwordLoading = true;
		try {
			proposal = await publicApi.getProposal(data.slug, password);
			passwordRequired = false;
		} catch {
			passwordError = $_('viewer.password.error');
		} finally {
			passwordLoading = false;
		}
	}

	// Block-time tracking via Intersection Observer
	// Accumulates ms per block_id, flushes to API every 10 seconds.
	const blockTimers = new Map<string, number>(); // blockId → entry timestamp (Date.now())
	const blockAccum = new Map<string, number>();  // blockId → accumulated ms
	let flushInterval: ReturnType<typeof setInterval>;
	let observer: IntersectionObserver;

	async function flushBlockTime() {
		if (!data.slug || blockAccum.size === 0) return;
		const events: Array<{ block_id: string; duration_ms: number }> = [];
		blockAccum.forEach((ms, id) => {
			if (ms > 0) events.push({ block_id: id, duration_ms: ms });
		});
		blockAccum.clear();
		if (events.length > 0) {
			trackingApi.trackBlockTime(data.slug, events).catch(() => { /* best-effort */ });
		}
	}

	onMount(() => {
		// Set locale to proposal owner's language
		if (proposal?.language) setLocale(proposal.language);

		if (!browser || !proposal) return;

		observer = new IntersectionObserver(
			(entries) => {
				entries.forEach((entry) => {
					const blockId = (entry.target as HTMLElement).dataset.blockId;
					if (!blockId) return;
					if (entry.isIntersecting) {
						blockTimers.set(blockId, Date.now());
					} else {
						const start = blockTimers.get(blockId);
						if (start != null) {
							blockAccum.set(blockId, (blockAccum.get(blockId) ?? 0) + (Date.now() - start));
							blockTimers.delete(blockId);
						}
					}
				});
			},
			{ threshold: 0.5 }
		);

		document.querySelectorAll('[data-block-id]').forEach((el) => observer.observe(el));

		// Flush accumulated block time every 10 seconds
		flushInterval = setInterval(flushBlockTime, 10_000);
	});

	onDestroy(() => {
		if (!browser) return;
		// Accumulate time for any still-visible blocks before flush
		blockTimers.forEach((start, id) => {
			blockAccum.set(id, (blockAccum.get(id) ?? 0) + (Date.now() - start));
		});
		flushBlockTime();
		clearInterval(flushInterval);
		observer?.disconnect();
	});

	async function handleApprove() {
		approveError = '';
		approveLoading = true;
		try {
			await publicApi.approve(data.slug, clientEmail);
			// Navigate to the dedicated confirmation page
			await goto(`/p/${data.slug}/confirmed`);
		} catch (e: unknown) {
			if (e instanceof Error && e.message.includes('ALREADY_APPROVED')) {
				// Proposal was already approved — treat as success
				await goto(`/p/${data.slug}/confirmed`);
			} else if (e instanceof Error && e.message.includes('INVALID_EMAIL')) {
				approveError = $_('viewer.approve.email_invalid');
			} else {
				approveError = $_('viewer.approve.failed');
			}
		} finally {
			approveLoading = false;
		}
	}
</script>

<svelte:head>
	{#if proposal}
		<title>{proposal.title} — {proposal.agency_name}</title>
		<meta name="robots" content="noindex" />
	{:else}
		<title>Proposal</title>
	{/if}
</svelte:head>

{#if revoked}
	<!-- Revoked stub -->
	<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
		<div class="bg-white rounded-2xl shadow-sm border border-gray-100 p-10 w-full max-w-sm text-center">
			<div class="w-14 h-14 bg-gray-100 rounded-2xl flex items-center justify-center mx-auto mb-5">
				<svg class="w-7 h-7 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
					<path stroke-linecap="round" stroke-linejoin="round" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"/>
				</svg>
			</div>
			<h1 class="text-xl font-bold text-gray-900 mb-2">{$_('viewer.revoked.heading')}</h1>
			<p class="text-sm text-gray-500 leading-relaxed">{$_('viewer.revoked.description')}</p>
		</div>
	</div>

{:else if passwordRequired}
	<!-- Password gate -->
	<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
		<div class="bg-white rounded-2xl shadow-sm border border-gray-100 p-8 w-full max-w-sm text-center">
			<div class="w-12 h-12 bg-indigo-50 rounded-xl flex items-center justify-center mx-auto mb-4">
				<svg class="w-6 h-6 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
				</svg>
			</div>
			<h2 class="text-lg font-semibold text-gray-900 mb-2">{$_('viewer.password.heading')}</h2>
			<p class="text-gray-500 text-sm mb-6">{$_('viewer.password.description')}</p>
			{#if passwordError}
				<p class="text-red-500 text-sm mb-3">{passwordError}</p>
			{/if}
			<form on:submit|preventDefault={submitPassword} class="space-y-3">
				<input
					type="password"
					bind:value={password}
					placeholder={$_('viewer.password.placeholder')}
					class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
				/>
				<button
					type="submit"
					disabled={passwordLoading}
					class="w-full py-2 px-4 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50"
				>
					{passwordLoading ? $_('viewer.password.checking') : $_('viewer.password.submit')}
				</button>
			</form>
		</div>
	</div>

{:else if proposal}
	<!-- Proposal viewer -->
	<div class="min-h-screen bg-white">
		<!-- Header with agency branding -->
		<header class="border-b border-gray-100 px-6 py-4">
			<div class="max-w-3xl mx-auto flex items-center justify-between">
				<div class="flex items-center gap-3">
					{#if proposal.logo_url}
						<img src={proposal.logo_url} alt={proposal.agency_name} class="h-8 w-auto object-contain" />
					{:else}
						<span class="text-lg font-bold" style="color: var(--color-primary)">{proposal.agency_name}</span>
					{/if}
				</div>
				{#if !approved}
					<button
						on:click={() => showApproveModal = true}
						class="px-4 py-2 text-sm font-medium text-white rounded-lg transition-colors"
						style="background-color: var(--color-primary)"
					>
						{$_('viewer.approve')}
					</button>
				{:else}
					<span class="flex items-center gap-2 text-sm text-green-600 font-medium">
						<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
							<path d="M5 13l4 4L19 7"/>
						</svg>
						{$_('viewer.approved_badge')}
					</span>
				{/if}
			</div>
		</header>

		<!-- Proposal content -->
		<div class="max-w-3xl mx-auto px-4 sm:px-6 py-10">
			<div class="mb-8">
				<h1 class="text-3xl font-bold text-gray-900">{proposal.title}</h1>
				{#if proposal.client_name}
					<p class="text-gray-500 mt-2">{$_('viewer.prepare')} <strong>{proposal.client_name}</strong></p>
				{/if}
			</div>

			<!-- Blocks -->
			<div class="space-y-8">
				{#each proposal.blocks as block (block.id)}
					<section data-block-id={block.id} class="proposal-block">

						{#if block.type === 'text'}
							<!-- Rich text block — editor stores data as { html } -->
							<div class="prose prose-gray max-w-none">
								{#if block.data.heading}
									<h2>{block.data.heading}</h2>
								{/if}
								{@html typeof block.data.html === 'string' ? block.data.html : ''}
							</div>

						{:else if block.type === 'price_table'}
							<!-- Price table block -->
							<div>
								{#if block.data.heading}
									<h2 class="text-xl font-semibold text-gray-900 mb-4">{block.data.heading}</h2>
								{/if}
								<div class="overflow-x-auto">
									<table class="w-full border border-gray-200 rounded-xl overflow-hidden min-w-[360px]">
										<thead>
											<tr class="bg-gray-50 text-left">
												<th class="px-4 py-3 text-sm font-medium text-gray-600">{$_('viewer.price_table.service')}</th>
												<th class="px-4 py-3 text-sm font-medium text-gray-600 text-right">{$_('viewer.price_table.qty')}</th>
												<th class="px-4 py-3 text-sm font-medium text-gray-600 text-right">{$_('viewer.price_table.price')}</th>
												<th class="px-4 py-3 text-sm font-medium text-gray-600 text-right">{$_('viewer.price_table.total')}</th>
											</tr>
										</thead>
										<tbody>
											{#each priceRows(block.data) as row}
												<tr class="border-t border-gray-100">
													<td class="px-4 py-3 text-sm text-gray-800">{row.service}</td>
													<td class="px-4 py-3 text-sm text-gray-600 text-right">{row.qty}</td>
													<td class="px-4 py-3 text-sm text-gray-600 text-right">{fmtNumber(row.price)}</td>
													<td class="px-4 py-3 text-sm font-medium text-gray-900 text-right">{fmtNumber(row.qty * row.price)}</td>
												</tr>
											{/each}
										</tbody>
										<tfoot>
											<tr class="border-t-2 border-gray-200 bg-gray-50">
												<td colspan="3" class="px-4 py-3 text-sm font-semibold text-gray-900">{$_('viewer.price_table.grand_total')}</td>
												<td class="px-4 py-3 text-sm font-bold text-gray-900 text-right">{fmtNumber(priceTotal(block.data))}</td>
											</tr>
										</tfoot>
									</table>
								</div>
							</div>

						{:else if block.type === 'case_study'}
							<!-- Case study block -->
							<div class="rounded-xl overflow-hidden border border-gray-200">
								{#if block.data.image_url}
									<img
										src={String(block.data.image_url)}
										alt={String(block.data.title ?? '')}
										class="w-full h-56 object-cover"
									/>
								{/if}
								<div class="p-5">
									{#if block.data.title}
										<h3 class="text-lg font-semibold text-gray-900 mb-2">{block.data.title}</h3>
									{/if}
									{#if block.data.description}
										<p class="text-sm text-gray-600 leading-relaxed">{block.data.description}</p>
									{/if}
								</div>
							</div>

						{:else if block.type === 'team_member'}
							<!-- Team member block -->
							<div class="flex flex-col sm:flex-row items-start gap-4 p-4 bg-gray-50 rounded-xl">
								{#if block.data.photo_url}
									<img
										src={String(block.data.photo_url)}
										alt={String(block.data.name ?? '')}
										class="w-16 h-16 rounded-full object-cover shrink-0"
									/>
								{:else}
									<div class="w-16 h-16 rounded-full bg-gray-200 shrink-0 flex items-center justify-center">
										<svg class="w-8 h-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
											<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z"/>
										</svg>
									</div>
								{/if}
								<div>
									{#if block.data.name}
										<p class="font-semibold text-gray-900">{block.data.name}</p>
									{/if}
									{#if block.data.role}
										<p class="text-sm text-gray-500">{block.data.role}</p>
									{/if}
									{#if block.data.bio}
										<p class="text-sm text-gray-700 mt-2">{block.data.bio}</p>
									{/if}
								</div>
							</div>

						{:else if block.type === 'terms'}
							<!-- Terms & conditions block -->
							<div>
								<h2 class="text-xl font-semibold text-gray-900 mb-4">{$_('viewer.terms.heading')}</h2>
								<ol class="list-decimal list-inside space-y-2">
									{#each termsItems(block.data) as item}
										<li class="text-sm text-gray-700">{item}</li>
									{/each}
								</ol>
							</div>
						{/if}

					</section>
				{/each}
			</div>

			<!-- Footer -->
			{#if !proposal.hide_proply_footer}
				<div class="mt-16 pt-8 border-t border-gray-100 text-center">
					<p class="text-xs text-gray-400">
						{$_('viewer.powered_by')} <a
							href="https://proply.io?utm_source=proposal&utm_medium=footer&utm_campaign=powered_by"
							class="hover:text-gray-600 transition-colors"
							target="_blank"
							rel="noopener noreferrer"
						>Proply</a>
					</p>
				</div>
			{/if}
		</div>
	</div>

	<!-- Approve modal -->
	{#if showApproveModal}
		<div
			class="fixed inset-0 bg-black/40 flex items-center justify-center z-50 px-4"
			on:click|self={() => showApproveModal = false}
			role="dialog"
			aria-modal="true"
			aria-labelledby="approve-title"
		>
			<div class="bg-white rounded-2xl p-6 w-full max-w-sm shadow-xl">
				<h3 id="approve-title" class="text-lg font-semibold text-gray-900 mb-2">{$_('viewer.approve.modal_heading')}</h3>
				<p class="text-sm text-gray-500 mb-4">{$_('viewer.approve.modal_description')}</p>

				{#if approveError}
					<p class="text-red-500 text-sm mb-3">{approveError}</p>
				{/if}

				<form on:submit|preventDefault={handleApprove} class="space-y-3">
					<input
						type="email"
						bind:value={clientEmail}
						placeholder={$_('viewer.approve.email_placeholder')}
						required
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
					/>
					<div class="flex gap-3">
						<button
							type="button"
							on:click={() => showApproveModal = false}
							class="flex-1 py-2 px-4 border border-gray-200 text-gray-700 font-medium rounded-lg hover:bg-gray-50"
						>
							{$_('viewer.approve.cancel')}
						</button>
						<button
							type="submit"
							disabled={approveLoading}
							class="flex-1 py-2 px-4 text-white font-medium rounded-lg disabled:opacity-50 transition-colors"
							style="background-color: var(--color-primary)"
						>
							{approveLoading ? $_('viewer.approve.submitting') : $_('viewer.approve.submit')}
						</button>
					</div>
				</form>
			</div>
		</div>
	{/if}
{/if}
