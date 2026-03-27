<script lang="ts">
	import type { PageData } from './$types';
	import { publicApi } from '$lib/api';
	import type { PublicProposal } from '$lib/api';

	export let data: PageData;

	let proposal: PublicProposal | null = data.proposal;
	let passwordRequired = data.passwordRequired;
	let password = '';
	let passwordError = '';
	let passwordLoading = false;

	// Approval state
	let showApproveModal = false;
	let clientEmail = '';
	let approveLoading = false;
	let approveError = '';
	let approved = proposal?.status === 'approved';
	let approvedAt = proposal?.approved_at;

	interface PriceRow { service: string; qty: number; price: number }
	function priceRows(data: Record<string, unknown>): PriceRow[] {
		return (data.rows as PriceRow[]) ?? [];
	}
	function termsItems(data: Record<string, unknown>): string[] {
		return (data.items as string[]) ?? [];
	}

	// Apply branding CSS variables
	$: if (proposal) {
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
			passwordError = 'Incorrect password. Try again.';
		} finally {
			passwordLoading = false;
		}
	}

	async function handleApprove() {
		approveError = '';
		approveLoading = true;
		try {
			const result = await publicApi.approve(data.slug, clientEmail);
			approved = true;
			approvedAt = result.approved_at;
			showApproveModal = false;
		} catch (e: unknown) {
			if (e instanceof Error && e.message.includes('ALREADY_APPROVED')) {
				approved = true;
				showApproveModal = false;
			} else {
				approveError = 'Failed to approve. Please try again.';
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

{#if passwordRequired}
	<!-- Password gate -->
	<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
		<div class="bg-white rounded-2xl shadow-sm border border-gray-100 p-8 w-full max-w-sm text-center">
			<div class="w-12 h-12 bg-indigo-50 rounded-xl flex items-center justify-center mx-auto mb-4">
				<svg class="w-6 h-6 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
				</svg>
			</div>
			<h2 class="text-lg font-semibold text-gray-900 mb-2">Password protected</h2>
			<p class="text-gray-500 text-sm mb-6">This proposal requires a password to view.</p>
			{#if passwordError}
				<p class="text-red-500 text-sm mb-3">{passwordError}</p>
			{/if}
			<form on:submit|preventDefault={submitPassword} class="space-y-3">
				<input
					type="password"
					bind:value={password}
					placeholder="Enter password"
					class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
				/>
				<button
					type="submit"
					disabled={passwordLoading}
					class="w-full py-2 px-4 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50"
				>
					{passwordLoading ? 'Checking...' : 'View proposal'}
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
						Approve proposal
					</button>
				{:else}
					<span class="flex items-center gap-2 text-sm text-green-600 font-medium">
						<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
							<path d="M5 13l4 4L19 7"/>
						</svg>
						Approved
					</span>
				{/if}
			</div>
		</header>

		<!-- Proposal content -->
		<div class="max-w-3xl mx-auto px-6 py-10">
			<div class="mb-8">
				<h1 class="text-3xl font-bold text-gray-900">{proposal.title}</h1>
				{#if proposal.client_name}
					<p class="text-gray-500 mt-2">Prepared for <strong>{proposal.client_name}</strong></p>
				{/if}
			</div>

			<!-- Blocks -->
			<div class="space-y-8">
				{#each proposal.blocks as block (block.id)}
					<section data-block-id={block.id} class="proposal-block">
						{#if block.type === 'text'}
							<div class="prose prose-gray max-w-none">
								{#if block.data.heading}
									<h2>{block.data.heading}</h2>
								{/if}
								<!-- TipTap content rendered as HTML -->
								{@html typeof block.data.content === 'string' ? block.data.content : ''}
							</div>

						{:else if block.type === 'price_table'}
							<div>
								{#if block.data.heading}
									<h2 class="text-xl font-semibold text-gray-900 mb-4">{block.data.heading}</h2>
								{/if}
								<table class="w-full border border-gray-200 rounded-xl overflow-hidden">
									<thead>
										<tr class="bg-gray-50 text-left">
											<th class="px-4 py-3 text-sm font-medium text-gray-600">Service</th>
											<th class="px-4 py-3 text-sm font-medium text-gray-600 text-right">Qty</th>
											<th class="px-4 py-3 text-sm font-medium text-gray-600 text-right">Price</th>
										</tr>
									</thead>
									<tbody>
										{#each priceRows(block.data) as row}
											<tr class="border-t border-gray-100">
												<td class="px-4 py-3 text-sm text-gray-800">{row.service}</td>
												<td class="px-4 py-3 text-sm text-gray-600 text-right">{row.qty}</td>
												<td class="px-4 py-3 text-sm font-medium text-gray-900 text-right">
													{block.data.currency} {row.price.toLocaleString()}
												</td>
											</tr>
										{/each}
									</tbody>
									<tfoot>
										<tr class="border-t-2 border-gray-200 bg-gray-50">
											<td colspan="2" class="px-4 py-3 text-sm font-semibold text-gray-900">Total</td>
											<td class="px-4 py-3 text-sm font-bold text-gray-900 text-right">
												{block.data.currency}
												{priceRows(block.data)
													.reduce((sum, r) => sum + r.qty * r.price, 0)
													.toLocaleString()}
											</td>
										</tr>
									</tfoot>
								</table>
							</div>

						{:else if block.type === 'team_member'}
							<div class="flex items-start gap-4 p-4 bg-gray-50 rounded-xl">
								{#if block.data.photo_url}
									<img src={String(block.data.photo_url)} alt={String(block.data.name)} class="w-16 h-16 rounded-full object-cover shrink-0" />
								{/if}
								<div>
									<p class="font-semibold text-gray-900">{block.data.name}</p>
									<p class="text-sm text-gray-500">{block.data.role}</p>
									{#if block.data.bio}
										<p class="text-sm text-gray-700 mt-2">{block.data.bio}</p>
									{/if}
								</div>
							</div>

						{:else if block.type === 'terms'}
							<div>
								<h2 class="text-xl font-semibold text-gray-900 mb-4">Terms & Conditions</h2>
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
						Powered by <a href="https://proply.io" class="hover:text-gray-600 transition-colors">Proply</a>
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
				<h3 id="approve-title" class="text-lg font-semibold text-gray-900 mb-2">Approve proposal</h3>
				<p class="text-sm text-gray-500 mb-4">Enter your email to confirm approval.</p>

				{#if approveError}
					<p class="text-red-500 text-sm mb-3">{approveError}</p>
				{/if}

				<form on:submit|preventDefault={handleApprove} class="space-y-3">
					<input
						type="email"
						bind:value={clientEmail}
						placeholder="your@email.com"
						required
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
					/>
					<div class="flex gap-3">
						<button
							type="button"
							on:click={() => showApproveModal = false}
							class="flex-1 py-2 px-4 border border-gray-200 text-gray-700 font-medium rounded-lg hover:bg-gray-50"
						>
							Cancel
						</button>
						<button
							type="submit"
							disabled={approveLoading}
							class="flex-1 py-2 px-4 text-white font-medium rounded-lg disabled:opacity-50 transition-colors"
							style="background-color: var(--color-primary)"
						>
							{approveLoading ? 'Confirming...' : 'Confirm'}
						</button>
					</div>
				</form>
			</div>
		</div>
	{/if}
{/if}
