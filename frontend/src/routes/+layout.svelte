<script lang="ts">
	import '../app.css';
	import { setupI18n, locale } from '$lib/i18n';
	import { authStore } from '$lib/stores/auth';

	// Initialize i18n synchronously — JSON messages are bundled, not loaded async
	setupI18n($authStore.user?.language ?? 'en');

	// Keep locale in sync when authenticated user language changes (e.g. settings page).
	// Only override locale when a user is logged in — public viewer pages set their own locale.
	$: if ($authStore.user) {
		const lang = $authStore.user.language ?? 'en';
		if ($locale !== lang) locale.set(lang);
	}
</script>

<slot />
