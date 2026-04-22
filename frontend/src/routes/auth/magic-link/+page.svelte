<script lang="ts">
	import { page } from '$app/stores';
	import { _ } from 'svelte-i18n';
	import { auth } from '$lib/api';

	let email = '';
	let emailError = '';
	let loading = false;
	let sent = false;

	$: linkError = $page.url.searchParams.get('error');

	const ERROR_KEYS: Record<string, string> = {
		invalid_or_expired: 'auth.magic_link.expired',
		missing_token: 'auth.magic_link.invalid',
		token_error: 'auth.magic_link.error'
	};

	function validate(): boolean {
		emailError = '';
		if (!email) { emailError = $_('validation.email_required'); return false; }
		if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) { emailError = $_('validation.email_invalid'); return false; }
		return true;
	}

	async function handleSubmit() {
		if (!validate()) return;
		loading = true;
		try {
			await auth.sendMagicLink(email);
			sent = true;
		} catch {
			// Always show success to avoid email enumeration
			sent = true;
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>{$_('auth.magic_link.page_title')}</title>
</svelte:head>

{#if linkError}
	<div class="mb-4 p-3 bg-red-50 border border-red-200 text-red-700 rounded-lg text-sm">
		{$_(ERROR_KEYS[linkError] ?? 'auth.magic_link.error')}
	</div>
{/if}

{#if sent}
	<div class="text-center py-4">
		<div class="w-14 h-14 bg-indigo-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
			<svg class="w-7 h-7 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
			</svg>
		</div>
		<h2 class="text-lg font-semibold text-gray-900 mb-2">{$_('auth.magic_link.check_inbox')}</h2>
		<p class="text-sm text-gray-500 mb-6">
			{$_('auth.magic_link.sent_to')} <strong>{email}</strong>.<br />
			{$_('auth.magic_link.expires')}
		</p>
		<button
			on:click={() => { sent = false; email = ''; }}
			class="text-sm text-indigo-500 hover:underline"
		>
			{$_('auth.magic_link.different_email')}
		</button>
	</div>
{:else}
	<h2 class="text-xl font-semibold text-gray-900 mb-2">{$_('auth.magic_link.heading')}</h2>
	<p class="text-sm text-gray-500 mb-6">{$_('auth.magic_link.description')}</p>

	<form on:submit|preventDefault={handleSubmit} class="space-y-4" novalidate>
		<div>
			<label for="email" class="block text-sm font-medium text-gray-700 mb-1">{$_('auth.email')}</label>
			<input
				id="email"
				type="email"
				bind:value={email}
				class="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500
					{emailError ? 'border-red-300 bg-red-50' : 'border-gray-300'}"
				placeholder={$_('auth.email.placeholder')}
				autocomplete="email"
			/>
			{#if emailError}<p class="mt-1 text-xs text-red-500">{emailError}</p>{/if}
		</div>

		<button
			type="submit"
			disabled={loading}
			class="w-full py-2.5 px-4 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition-colors"
		>
			{loading ? $_('auth.magic_link.sending') : $_('auth.magic_link.send')}
		</button>
	</form>

	<p class="mt-6 text-center text-sm text-gray-500">
		<a href="/auth/login" class="text-indigo-600 hover:underline">{$_('auth.magic_link.with_password')}</a>
	</p>
{/if}
