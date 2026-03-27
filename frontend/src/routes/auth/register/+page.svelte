<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/api';
	import { authStore } from '$lib/stores/auth';

	let name = '';
	let email = '';
	let password = '';
	let errors: Record<string, string> = {};
	let serverError = '';
	let loading = false;

	const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

	function validate(): boolean {
		errors = {};
		if (!name.trim()) errors.name = 'Name is required';
		if (!email) errors.email = 'Email is required';
		else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) errors.email = 'Enter a valid email';
		if (!password) errors.password = 'Password is required';
		else if (password.length < 8) errors.password = 'Password must be at least 8 characters';
		return Object.keys(errors).length === 0;
	}

	async function handleRegister() {
		if (!validate()) return;
		serverError = '';
		loading = true;
		try {
			const { access_token, email_verified } = await auth.register(email, password, name);
			const user = await auth.me(access_token);
			authStore.setUser(user, access_token, email_verified);
			goto('/dashboard');
		} catch (e: unknown) {
			if (e instanceof Error && e.message.includes('EMAIL_EXISTS')) {
				serverError = 'An account with this email already exists.';
			} else if (e instanceof Error && e.message.includes('VALIDATION_ERROR')) {
				serverError = 'Please check your inputs and try again.';
			} else {
				serverError = 'Registration failed. Please try again.';
			}
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Create account — Proply</title>
</svelte:head>

<h2 class="text-xl font-semibold text-gray-900 mb-6">Create account</h2>

{#if serverError}
	<div class="mb-4 p-3 bg-red-50 border border-red-200 text-red-700 rounded-lg text-sm">
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
	Continue with Google
</a>

<div class="relative mb-5">
	<div class="absolute inset-0 flex items-center">
		<div class="w-full border-t border-gray-100"></div>
	</div>
	<div class="relative flex justify-center text-xs text-gray-400 bg-white px-3">or</div>
</div>

<form on:submit|preventDefault={handleRegister} class="space-y-4" novalidate>
	<div>
		<label for="name" class="block text-sm font-medium text-gray-700 mb-1">Your name</label>
		<input
			id="name"
			type="text"
			bind:value={name}
			class="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent
				{errors.name ? 'border-red-300 bg-red-50' : 'border-gray-300'}"
			placeholder="Jane Smith"
			autocomplete="name"
		/>
		{#if errors.name}<p class="mt-1 text-xs text-red-500">{errors.name}</p>{/if}
	</div>

	<div>
		<label for="email" class="block text-sm font-medium text-gray-700 mb-1">Email</label>
		<input
			id="email"
			type="email"
			bind:value={email}
			class="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent
				{errors.email ? 'border-red-300 bg-red-50' : 'border-gray-300'}"
			placeholder="you@agency.com"
			autocomplete="email"
		/>
		{#if errors.email}<p class="mt-1 text-xs text-red-500">{errors.email}</p>{/if}
	</div>

	<div>
		<label for="password" class="block text-sm font-medium text-gray-700 mb-1">Password</label>
		<input
			id="password"
			type="password"
			bind:value={password}
			class="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent
				{errors.password ? 'border-red-300 bg-red-50' : 'border-gray-300'}"
			placeholder="Min. 8 characters"
			autocomplete="new-password"
		/>
		{#if errors.password}<p class="mt-1 text-xs text-red-500">{errors.password}</p>{/if}
	</div>

	<button
		type="submit"
		disabled={loading}
		class="w-full py-2.5 px-4 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
	>
		{loading ? 'Creating account...' : 'Create account'}
	</button>
</form>

<p class="mt-4 text-xs text-center text-gray-400">
	By creating an account you agree to our
	<a href="/legal/terms" class="underline">Terms</a> and
	<a href="/legal/privacy" class="underline">Privacy Policy</a>.
</p>

<p class="mt-4 text-center text-sm text-gray-500">
	Already have an account?
	<a href="/auth/login" class="text-indigo-600 hover:underline font-medium">Sign in</a>
</p>
