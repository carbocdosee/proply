<script lang="ts">
	import { page } from '$app/stores';
	import { auth } from '$lib/api';

	let email = '';
	let emailError = '';
	let loading = false;
	let sent = false;

	$: linkError = $page.url.searchParams.get('error');

	const ERROR_MESSAGES: Record<string, string> = {
		invalid_or_expired: 'This link has expired or already been used. Request a new one.',
		missing_token: 'Invalid link. Please request a new one.',
		token_error: 'Something went wrong. Please try again.'
	};

	function validate(): boolean {
		emailError = '';
		if (!email) { emailError = 'Email is required'; return false; }
		if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) { emailError = 'Enter a valid email'; return false; }
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
	<title>Sign in with email — Proply</title>
</svelte:head>

{#if linkError}
	<div class="mb-4 p-3 bg-red-50 border border-red-200 text-red-700 rounded-lg text-sm">
		{ERROR_MESSAGES[linkError] ?? 'An error occurred. Please try again.'}
	</div>
{/if}

{#if sent}
	<div class="text-center py-4">
		<div class="w-14 h-14 bg-indigo-50 rounded-2xl flex items-center justify-center mx-auto mb-4">
			<svg class="w-7 h-7 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
			</svg>
		</div>
		<h2 class="text-lg font-semibold text-gray-900 mb-2">Check your inbox</h2>
		<p class="text-sm text-gray-500 mb-6">
			We sent a sign-in link to <strong>{email}</strong>.<br />
			The link expires in 15 minutes.
		</p>
		<button
			on:click={() => { sent = false; email = ''; }}
			class="text-sm text-indigo-500 hover:underline"
		>
			Use a different email
		</button>
	</div>
{:else}
	<h2 class="text-xl font-semibold text-gray-900 mb-2">Sign in with email</h2>
	<p class="text-sm text-gray-500 mb-6">We'll send you a one-time login link. No password needed.</p>

	<form on:submit|preventDefault={handleSubmit} class="space-y-4" novalidate>
		<div>
			<label for="email" class="block text-sm font-medium text-gray-700 mb-1">Email</label>
			<input
				id="email"
				type="email"
				bind:value={email}
				class="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500
					{emailError ? 'border-red-300 bg-red-50' : 'border-gray-300'}"
				placeholder="you@agency.com"
				autocomplete="email"
			/>
			{#if emailError}<p class="mt-1 text-xs text-red-500">{emailError}</p>{/if}
		</div>

		<button
			type="submit"
			disabled={loading}
			class="w-full py-2.5 px-4 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition-colors"
		>
			{loading ? 'Sending...' : 'Send magic link'}
		</button>
	</form>

	<p class="mt-6 text-center text-sm text-gray-500">
		<a href="/auth/login" class="text-indigo-600 hover:underline">Sign in with password instead</a>
	</p>
{/if}
