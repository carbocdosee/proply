<script lang="ts">
	import { authStore } from '$lib/stores/auth';

	const PLANS = [
		{
			id: 'free',
			name: 'Free',
			price: '$0',
			features: ['3 published proposals', 'Public viewer', 'Email notifications', 'Tracking']
		},
		{
			id: 'pro',
			name: 'Pro',
			price: '$19/mo',
			features: ['Unlimited proposals', 'Block analytics', 'Remove Proply branding', 'Priority support'],
			highlighted: true
		}
	];
</script>

<svelte:head>
	<title>Billing — Proply</title>
</svelte:head>

<div class="max-w-3xl mx-auto">
	<h1 class="text-2xl font-bold text-gray-900 mb-8">Billing</h1>

	<div class="grid grid-cols-2 gap-6">
		{#each PLANS as plan}
			<div
				class="bg-white rounded-2xl border p-6 {plan.highlighted
					? 'border-indigo-300 ring-2 ring-indigo-100'
					: 'border-gray-100'}"
			>
				<div class="flex items-center justify-between mb-1">
					<h3 class="text-lg font-semibold text-gray-900">{plan.name}</h3>
					{#if $authStore.user?.plan === plan.id}
						<span class="text-xs px-2 py-0.5 bg-green-50 text-green-700 rounded-full font-medium">Current</span>
					{/if}
				</div>
				<p class="text-2xl font-bold text-gray-900 mb-4">{plan.price}</p>
				<ul class="space-y-2 mb-6">
					{#each plan.features as feature}
						<li class="flex items-center gap-2 text-sm text-gray-600">
							<svg class="w-4 h-4 text-green-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
								<path d="M5 13l4 4L19 7"/>
							</svg>
							{feature}
						</li>
					{/each}
				</ul>
				{#if $authStore.user?.plan !== plan.id && plan.id !== 'free'}
					<button
						class="w-full py-2 px-4 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 transition-colors"
					>
						Upgrade to {plan.name}
					</button>
				{:else if $authStore.user?.plan === plan.id && plan.id !== 'free'}
					<button
						class="w-full py-2 px-4 border border-gray-200 text-gray-700 font-medium rounded-lg hover:bg-gray-50 transition-colors"
					>
						Manage subscription
					</button>
				{/if}
			</div>
		{/each}
	</div>
</div>
