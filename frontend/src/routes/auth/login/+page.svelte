<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { _ } from 'svelte-i18n';
	import { auth } from '$lib/api';
	import { authStore } from '$lib/stores/auth';

	let email = '';
	let password = '';
	let errors: Record<string, string> = {};
	let serverError = '';
	let loading = false;

	$: oauthError = $page.url.searchParams.get('error');

	function validate(): boolean {
		errors = {};
		if (!email) errors.email = $_('validation.email_required');
		else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) errors.email = $_('validation.email_invalid');
		if (!password) errors.password = $_('validation.password_required');
		else if (password.length < 8) errors.password = $_('validation.password_min');
		return Object.keys(errors).length === 0;
	}

	async function handleLogin() {
		if (!validate()) return;
		serverError = '';
		loading = true;
		try {
			const { access_token, email_verified } = await auth.login(email, password);
			const user = await auth.me(access_token);
			authStore.setUser(user, access_token, email_verified);
			goto('/dashboard');
		} catch (e: unknown) {
			if (e instanceof Error && e.message.includes('INVALID_CREDENTIALS')) {
				serverError = $_('auth.login.wrong_credentials');
			} else {
				serverError = $_('auth.login.failed');
			}
		} finally {
			loading = false;
		}
	}

	const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';
</script>

<svelte:head>
	<title>{$_('auth.login.page_title')}</title>
</svelte:head>

<h2 class="text-xl font-semibold text-gray-900 mb-6">{$_('auth.login.heading')}</h2>

{#if oauthError}
	<div class="mb-4 p-3 bg-red-50 border border-red-200 text-red-700 rounded-lg text-sm">
		{oauthError === 'state_mismatch'
			? $_('auth.login.auth_failed')
			: oauthError === 'oauth_not_configured'
				? $_('auth.login.google_not_configured')
				: $_('auth.login.google_error') + oauthError}
	</div>
{/if}

{#if serverError}
	<div data-testid="login-error" class="mb-4 p-3 bg-red-50 border border-red-200 text-red-700 rounded-lg text-sm">
		{serverError}
	</div>
{/if}

<!-- Google OAuth -->
<a
	href="{API_URL}/api/v1/auth/google"
	class="flex items-center justify-center gap-3 w-full py-2.5 px-4 border border-gray-200 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors mb-5"
>
	<svg class="w-5 h-5" viewBox="0 0 24 24">
		<path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
		<path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
		<path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
		<path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
	</svg>
	{$_('auth.google')}
</a>

<div class="relative mb-5">
	<div class="absolute inset-0 flex items-center">
		<div class="w-full border-t border-gray-100"></div>
	</div>
	<div class="relative flex justify-center text-xs text-gray-400 bg-white px-3">{$_('auth.or')}</div>
</div>

<form on:submit|preventDefault={handleLogin} class="space-y-4" novalidate>
	<div>
		<label for="email" class="block text-sm font-medium text-gray-700 mb-1">{$_('auth.email')}</label>
		<input
			id="email"
			data-testid="login-email"
			type="email"
			bind:value={email}
			class="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent
				{errors.email ? 'border-red-300 bg-red-50' : 'border-gray-300'}"
			placeholder={$_('auth.email.placeholder')}
			autocomplete="email"
		/>
		{#if errors.email}<p class="mt-1 text-xs text-red-500">{errors.email}</p>{/if}
	</div>

	<div>
		<div class="flex items-center justify-between mb-1">
			<label for="password" class="text-sm font-medium text-gray-700">{$_('auth.password')}</label>
			<a href="/auth/magic-link" class="text-xs text-indigo-500 hover:underline">{$_('auth.login.forgot')}</a>
		</div>
		<input
			id="password"
			data-testid="login-password"
			type="password"
			bind:value={password}
			class="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent
				{errors.password ? 'border-red-300 bg-red-50' : 'border-gray-300'}"
			autocomplete="current-password"
		/>
		{#if errors.password}<p class="mt-1 text-xs text-red-500">{errors.password}</p>{/if}
	</div>

	<button
		type="submit"
		data-testid="login-submit"
		disabled={loading}
		class="w-full py-2.5 px-4 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
	>
		{loading ? $_('auth.login.submitting') : $_('auth.login.submit')}
	</button>
</form>

<p class="mt-6 text-center text-sm text-gray-500">
	{$_('auth.no_account')}
	<a href="/auth/register" class="text-indigo-600 hover:underline font-medium">{$_('auth.create_account_link')}</a>
</p>
