<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { _ } from 'svelte-i18n';
	import { auth } from '$lib/api';
	import { authStore } from '$lib/stores/auth';

	let errorKey = '';

	onMount(async () => {
		const token = $page.url.searchParams.get('token');
		const errorParam = $page.url.searchParams.get('error');

		if (errorParam) {
			const keyMap: Record<string, string> = {
				state_mismatch: 'auth.callback.auth_failed',
				oauth_not_configured: 'auth.login.google_not_configured'
			};
			errorKey = keyMap[errorParam] ?? 'auth.callback.error';
			return;
		}

		if (!token) {
			errorKey = 'auth.callback.no_token';
			return;
		}

		try {
			const user = await auth.me(token);
			authStore.setUser(user, token, !!user.email_verified_at);
			goto('/dashboard');
		} catch {
			errorKey = 'auth.callback.auth_failed';
		}
	});
</script>

<svelte:head>
	<title>{$_('auth.callback.page_title')}</title>
</svelte:head>

{#if errorKey}
	<div class="text-center py-8">
		<div class="w-14 h-14 bg-red-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
			<svg class="w-7 h-7 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"/>
			</svg>
		</div>
		<h2 class="text-lg font-semibold text-gray-900 mb-2">{$_('auth.callback.failed_heading')}</h2>
		<p class="text-sm text-gray-500 mb-6">{$_(errorKey)}</p>
		<a href="/auth/login" class="text-sm text-indigo-500 hover:underline">{$_('auth.callback.back')}</a>
	</div>
{:else}
	<div class="text-center py-8">
		<div class="w-14 h-14 bg-indigo-50 rounded-2xl flex items-center justify-center mx-auto mb-4 animate-pulse">
			<svg class="w-7 h-7 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"/>
			</svg>
		</div>
		<p class="text-sm text-gray-500">{$_('auth.magic_link.success.heading')}</p>
	</div>
{/if}
