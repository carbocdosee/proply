<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { auth } from '$lib/api';
	import { authStore } from '$lib/stores/auth';

	let error = '';

	onMount(async () => {
		const token = $page.url.searchParams.get('token');
		const errorParam = $page.url.searchParams.get('error');

		if (errorParam) {
			const messages: Record<string, string> = {
				state_mismatch: 'Authentication failed. Please try again.',
				oauth_not_configured: 'Google sign-in is not yet configured.',
			};
			error = messages[errorParam] ?? 'Sign-in error. Please try again.';
			return;
		}

		if (!token) {
			error = 'Authentication failed. No token received.';
			return;
		}

		try {
			const user = await auth.me(token);
			authStore.setUser(user, token, !!user.email_verified_at);
			goto('/dashboard');
		} catch {
			error = 'Authentication failed. Please try again.';
		}
	});
</script>

<svelte:head>
	<title>Signing you in… — Proply</title>
</svelte:head>

{#if error}
	<div class="text-center py-8">
		<div class="w-14 h-14 bg-red-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
			<svg class="w-7 h-7 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"/>
			</svg>
		</div>
		<h2 class="text-lg font-semibold text-gray-900 mb-2">Sign-in failed</h2>
		<p class="text-sm text-gray-500 mb-6">{error}</p>
		<a href="/auth/login" class="text-sm text-indigo-500 hover:underline">Back to sign in</a>
	</div>
{:else}
	<div class="text-center py-8">
		<div class="w-14 h-14 bg-indigo-50 rounded-2xl flex items-center justify-center mx-auto mb-4 animate-pulse">
			<svg class="w-7 h-7 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"/>
			</svg>
		</div>
		<p class="text-sm text-gray-500">Signing you in…</p>
	</div>
{/if}
