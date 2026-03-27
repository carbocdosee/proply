<script lang="ts">
	import { authStore } from '$lib/stores/auth';

	let name = $authStore.user?.name ?? '';
	let primaryColor = $authStore.user?.primary_color ?? '#6366F1';
	let accentColor = $authStore.user?.accent_color ?? '#F59E0B';
	let language = $authStore.user?.language ?? 'en';
	let saved = false;

	async function handleSave() {
		// TODO: call PATCH /api/v1/users/me
		saved = true;
		setTimeout(() => (saved = false), 2000);
	}
</script>

<svelte:head>
	<title>Settings — Proply</title>
</svelte:head>

<div class="max-w-2xl mx-auto">
	<h1 class="text-2xl font-bold text-gray-900 mb-8">Settings</h1>

	<form on:submit|preventDefault={handleSave} class="space-y-8">
		<!-- Profile -->
		<section class="bg-white rounded-xl border border-gray-100 p-6">
			<h2 class="text-base font-semibold text-gray-900 mb-4">Profile</h2>
			<div class="space-y-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Name</label>
					<input
						type="text"
						bind:value={name}
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
					/>
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Email</label>
					<input
						type="email"
						value={$authStore.user?.email ?? ''}
						disabled
						class="w-full px-3 py-2 border border-gray-200 rounded-lg bg-gray-50 text-gray-500 cursor-not-allowed"
					/>
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Language</label>
					<select
						bind:value={language}
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
					>
						<option value="en">English</option>
						<option value="ru">Русский</option>
					</select>
				</div>
			</div>
		</section>

		<!-- Branding -->
		<section class="bg-white rounded-xl border border-gray-100 p-6">
			<h2 class="text-base font-semibold text-gray-900 mb-4">Branding</h2>
			<div class="space-y-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">Logo</label>
					<div class="flex items-center gap-3">
						{#if $authStore.user?.logo_url}
							<img src={$authStore.user.logo_url} alt="Logo" class="h-10 w-auto object-contain" />
						{:else}
							<div class="w-10 h-10 bg-gray-100 rounded-lg flex items-center justify-center text-gray-400 text-xs">
								Logo
							</div>
						{/if}
						<button
							type="button"
							class="px-3 py-1.5 text-sm border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors"
						>
							Upload logo
						</button>
					</div>
				</div>
				<div class="grid grid-cols-2 gap-4">
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">Primary color</label>
						<div class="flex items-center gap-2">
							<input type="color" bind:value={primaryColor} class="w-10 h-10 rounded-lg border border-gray-200 cursor-pointer p-1" />
							<input
								type="text"
								bind:value={primaryColor}
								class="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 font-mono"
							/>
						</div>
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-1">Accent color</label>
						<div class="flex items-center gap-2">
							<input type="color" bind:value={accentColor} class="w-10 h-10 rounded-lg border border-gray-200 cursor-pointer p-1" />
							<input
								type="text"
								bind:value={accentColor}
								class="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 font-mono"
							/>
						</div>
					</div>
				</div>
			</div>
		</section>

		<div class="flex items-center justify-end gap-3">
			{#if saved}
				<span class="text-sm text-green-600 font-medium">Changes saved</span>
			{/if}
			<button
				type="submit"
				class="px-4 py-2 bg-indigo-600 text-white font-medium rounded-lg hover:bg-indigo-700 transition-colors"
			>
				Save changes
			</button>
		</div>
	</form>
</div>
