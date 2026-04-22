<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { _ } from 'svelte-i18n';
	import { authStore } from '$lib/stores/auth';

	let status: 'loading' | 'success' | 'error' = 'loading';
	let errorKey = '';

	onMount(() => {
		const success = $page.url.searchParams.get('success');
		const error = $page.url.searchParams.get('error');

		if (success === '1') {
			authStore.markEmailVerified();
			status = 'success';
			setTimeout(() => goto('/dashboard'), 2000);
		} else {
			const keyMap: Record<string, string> = {
				invalid_or_expired: 'auth.verify.expired',
				missing_token: 'auth.verify.invalid',
				token_error: 'auth.verify.error'
			};
			errorKey = keyMap[error ?? ''] ?? 'auth.verify.error';
			status = 'error';
		}
	});
</script>

<svelte:head>
	<title>{$_('auth.verify.page_title')}</title>
</svelte:head>

{#if status === 'loading'}
	<div class="text-center py-8">
		<p class="text-sm text-gray-500">{$_('auth.verify.verifying')}</p>
	</div>

{:else if status === 'success'}
	<div class="text-center py-8">
		<div class="w-14 h-14 bg-green-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
			<svg class="w-7 h-7 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/>
			</svg>
		</div>
		<h2 class="text-lg font-semibold text-gray-900 mb-2">{$_('auth.verify.success')}</h2>
		<p class="text-sm text-gray-500">{$_('auth.verify.redirecting')}</p>
	</div>

{:else}
	<div class="text-center py-8">
		<div class="w-14 h-14 bg-red-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
			<svg class="w-7 h-7 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"/>
			</svg>
		</div>
		<h2 class="text-lg font-semibold text-gray-900 mb-2">{$_('auth.verify.failed_heading')}</h2>
		<p class="text-sm text-gray-500 mb-6">{$_(errorKey)}</p>
		<a href="/dashboard" class="text-sm text-indigo-500 hover:underline">{$_('auth.verify.go_dashboard')}</a>
	</div>
{/if}
