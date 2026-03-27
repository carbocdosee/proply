<script lang="ts">
	import '../app.css';
	import { setupI18n } from '$lib/i18n';
	import { onMount } from 'svelte';

	// Initialize i18n on the client side
	onMount(() => {
		setupI18n();
	});
</script>

<slot />
