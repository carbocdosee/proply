<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	// Root page: redirect to dashboard or login
	onMount(() => {
		goto('/dashboard');
	});
</script>

<div class="min-h-screen flex items-center justify-center bg-gray-50">
	<div class="text-center">
		<h1 class="text-2xl font-bold text-indigo-600">Proply</h1>
		<p class="mt-2 text-gray-500">Loading...</p>
	</div>
</div>
