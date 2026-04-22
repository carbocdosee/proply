<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { page } from '$app/stores';
	import { authStore } from '$lib/stores/auth';
	import { billing } from '$lib/api';

	let checkoutLoading: string | null = null; // plan being checked out
	let portalLoading = false;
	let actionError = '';

	// Show success banner when Stripe redirects back with ?success=1
	$: successParam = $page.url.searchParams.get('success');

	const PLANS = [
		{
			id: 'free',
			period: false,
			highlighted: false,
			hasCta: false,
			featureCount: 5
		},
		{
			id: 'pro',
			period: true,
			highlighted: true,
			hasCta: true,
			featureCount: 5
		},
		{
			id: 'team',
			period: true,
			highlighted: false,
			hasCta: true,
			featureCount: 5
		}
	] as const;

	async function handleUpgrade(plan: 'pro' | 'team') {
		if (!$authStore.accessToken || checkoutLoading) return;
		actionError = '';
		checkoutLoading = plan;
		try {
			const { checkout_url } = await billing.createCheckout($authStore.accessToken, plan);
			window.location.href = checkout_url;
		} catch {
			actionError = $_('billing.checkout_failed');
			checkoutLoading = null;
		}
	}

	async function handleManage() {
		if (!$authStore.accessToken || portalLoading) return;
		actionError = '';
		portalLoading = true;
		try {
			const { portal_url } = await billing.createPortal($authStore.accessToken);
			window.location.href = portal_url;
		} catch {
			actionError = $_('billing.portal_failed');
			portalLoading = false;
		}
	}

	$: currentPlan = $authStore.user?.plan ?? 'free';
	$: activatedPlanName = currentPlan === 'free' ? 'Pro' : currentPlan.charAt(0).toUpperCase() + currentPlan.slice(1);

	// Pre-compute translated feature strings for each plan so that $_ (a store)
	// is referenced at the top level of the component rather than inside nested
	// {#each} blocks — required by the Svelte 4 compiler.
	$: plansWithFeatures = PLANS.map((plan) => ({
		...plan,
		features: Array.from({ length: plan.featureCount }, (_, i) =>
			$_('billing.plan.' + plan.id + '.f' + (i + 1))
		)
	}));
</script>

<svelte:head>
	<title>{$_('billing.page_title')}</title>
</svelte:head>

<div class="max-w-4xl mx-auto">
	<h1 class="text-2xl font-bold text-gray-900 mb-2">{$_('billing.heading')}</h1>
	<p class="text-gray-500 mb-8">{$_('billing.subtitle')}</p>

	<!-- Success banner -->
	{#if successParam === '1'}
		<div class="mb-6 flex items-center gap-3 px-4 py-3 bg-green-50 border border-green-200 rounded-xl text-sm text-green-800">
			<svg class="w-4 h-4 text-green-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
				<path d="M5 13l4 4L19 7"/>
			</svg>
			{$_('billing.activated', { values: { plan: activatedPlanName } })}
		</div>
	{/if}

	{#if actionError}
		<div class="mb-4 px-4 py-3 bg-red-50 border border-red-200 rounded-xl text-sm text-red-700">
			{actionError}
		</div>
	{/if}

	<!-- Plan cards -->
	<div class="grid grid-cols-1 sm:grid-cols-3 gap-5 mb-8">
		{#each plansWithFeatures as plan}
			{@const isCurrent = currentPlan === plan.id}
			<div class="relative bg-white rounded-2xl border p-6 flex flex-col {plan.highlighted ? 'border-indigo-300 ring-2 ring-indigo-100' : 'border-gray-100'}">
				{#if plan.highlighted}
					<span class="absolute -top-3 left-1/2 -translate-x-1/2 text-xs font-semibold px-3 py-1 bg-indigo-600 text-white rounded-full">{$_('billing.popular')}</span>
				{/if}

				<div class="mb-4">
					<div class="flex items-center justify-between mb-1">
						<h3 class="text-lg font-semibold text-gray-900">{$_('billing.plan.' + plan.id + '.name')}</h3>
						{#if isCurrent}
							<span class="text-xs px-2 py-0.5 bg-green-50 text-green-700 rounded-full font-medium">{$_('billing.current_badge')}</span>
						{/if}
					</div>
					<p class="text-sm text-gray-500">{$_('billing.plan.' + plan.id + '.description')}</p>
				</div>

				<div class="mb-5">
					<span class="text-3xl font-bold text-gray-900">{$_('billing.plan.' + plan.id + '.price')}</span>
					{#if plan.period}
						<span class="text-gray-400 text-sm">{$_('billing.plan.' + plan.id + '.period')}</span>
					{/if}
				</div>

				<ul class="space-y-2 mb-6 flex-1">
					{#each plan.features as feature}
						<li class="flex items-start gap-2 text-sm text-gray-600">
							<svg class="w-4 h-4 text-green-500 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
								<path d="M5 13l4 4L19 7"/>
							</svg>
							{feature}
						</li>
					{/each}
				</ul>

				<!-- CTA button -->
				{#if isCurrent && plan.id !== 'free'}
					<button
						on:click={handleManage}
						disabled={portalLoading}
						class="w-full py-2 px-4 border border-gray-200 text-gray-700 font-medium rounded-lg hover:bg-gray-50 disabled:opacity-50 transition-colors"
					>
						{portalLoading ? $_('billing.opening') : $_('billing.manage')}
					</button>
				{:else if !isCurrent && plan.id !== 'free'}
					<button
						on:click={() => handleUpgrade(plan.id)}
						disabled={checkoutLoading !== null}
						class="w-full py-2 px-4 font-medium rounded-lg transition-colors disabled:opacity-50
							{plan.highlighted
								? 'bg-indigo-600 text-white hover:bg-indigo-700'
								: 'border border-indigo-300 text-indigo-700 hover:bg-indigo-50'}"
					>
						{checkoutLoading === plan.id ? $_('billing.redirecting') : $_('billing.plan.' + plan.id + '.cta')}
					</button>
				{:else if plan.id === 'free' && currentPlan !== 'free'}
					<button
						on:click={handleManage}
						disabled={portalLoading}
						class="w-full py-2 px-4 border border-gray-200 text-gray-500 font-medium rounded-lg hover:bg-gray-50 disabled:opacity-50 transition-colors text-sm"
					>
						{portalLoading ? $_('billing.opening') : $_('billing.downgrade')}
					</button>
				{:else}
					<div class="py-2 px-4 text-center text-sm text-gray-400">{$_('billing.current_plan_label')}</div>
				{/if}
			</div>
		{/each}
	</div>

	<!-- Manage subscription link for Pro/Team users -->
	{#if currentPlan !== 'free'}
		<p class="text-sm text-gray-500 text-center">
			{$_('billing.manage_link')}
			<button
				on:click={handleManage}
				disabled={portalLoading}
				class="text-indigo-600 hover:underline disabled:opacity-50"
			>
				{$_('billing.open_portal')}
			</button>
		</p>
	{/if}
</div>
