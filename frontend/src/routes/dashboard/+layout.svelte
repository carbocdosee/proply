<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { authStore, isEmailVerified } from '$lib/stores/auth';
	import { auth } from '$lib/api';

	let resendLoading = false;
	let resendSent = false;

	// Redirect to login if not authenticated
	onMount(async () => {
		try {
			const { access_token, email_verified } = await auth.refresh();
			const user = await auth.me(access_token);
			authStore.setUser(user, access_token, email_verified);
		} catch {
			goto('/auth/login');
		}
	});

	async function handleLogout() {
		await auth.logout().catch(() => {});
		authStore.logout();
		goto('/auth/login');
	}

	async function handleResendVerification() {
		const token = $authStore.accessToken;
		if (!token || resendLoading || resendSent) return;
		resendLoading = true;
		try {
			await auth.resendVerification(token);
			resendSent = true;
		} catch {
			// silently ignore
		} finally {
			resendLoading = false;
		}
	}
</script>

<div class="min-h-screen bg-gray-50 flex">
	<!-- Sidebar -->
	<aside class="w-56 bg-white border-r border-gray-100 flex flex-col py-6 px-4 fixed h-full">
		<a href="/dashboard" class="text-xl font-bold text-indigo-600 mb-8 block">{$_('app.name')}</a>

		<nav class="flex-1 space-y-1">
			<a
				href="/dashboard"
				class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 hover:text-indigo-600 transition-colors"
			>
				<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
				</svg>
				{$_('nav.proposals')}
			</a>
			<a
				href="/dashboard/settings"
				class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 hover:text-indigo-600 transition-colors"
			>
				<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/>
					<path d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
				</svg>
				{$_('nav.settings')}
			</a>
			<a
				href="/dashboard/billing"
				class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 hover:text-indigo-600 transition-colors"
			>
				<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z"/>
				</svg>
				{$_('nav.billing')}
			</a>
		</nav>

		<div class="mt-auto border-t border-gray-100 pt-4">
			{#if $authStore.user}
				<div class="px-3 py-2 mb-2">
					<p class="text-sm font-medium text-gray-900 truncate">{$authStore.user.name}</p>
					<p class="text-xs text-gray-500 truncate">{$authStore.user.email}</p>
					<span class="inline-block mt-1 text-xs px-2 py-0.5 rounded-full bg-indigo-50 text-indigo-700 font-medium capitalize">
						{$authStore.user.plan}
					</span>
				</div>
			{/if}
			<button
				on:click={handleLogout}
				class="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium text-gray-500 hover:bg-gray-50 hover:text-gray-700 transition-colors"
			>
				<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"/>
				</svg>
				{$_('nav.logout')}
			</button>
		</div>
	</aside>

	<!-- Main content -->
	<main data-testid="dashboard-root" class="ml-56 flex-1 flex flex-col">
		{#if !$isEmailVerified && $authStore.user}
			<div data-testid="verify-email-banner" class="bg-amber-50 border-b border-amber-200 px-8 py-3 flex items-center justify-between gap-4">
				<p class="text-sm text-amber-800">
					<strong>{$_('dashboard.verify_banner.heading')}</strong> — {$_('dashboard.verify_banner.text')}
				</p>
				<button
					on:click={handleResendVerification}
					disabled={resendLoading || resendSent}
					class="shrink-0 text-sm font-medium text-amber-700 underline hover:text-amber-900 disabled:opacity-50 disabled:no-underline"
				>
					{resendSent ? $_('dashboard.verify_banner.sent') : resendLoading ? $_('dashboard.verify_banner.sending') : $_('dashboard.verify_banner.resend')}
				</button>
			</div>
		{/if}
		<div class="p-8 flex-1">
			<slot />
		</div>
	</main>
</div>
