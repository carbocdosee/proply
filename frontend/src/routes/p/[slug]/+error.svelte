<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { page } from '$app/stores';
</script>

<svelte:head>
	<title>{$page.status === 404 ? $_('error.not_found.heading') : $_('error.server.heading')}</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
	<div class="bg-white rounded-2xl shadow-sm border border-gray-100 p-10 w-full max-w-sm text-center">
		<div class="w-14 h-14 bg-gray-100 rounded-2xl flex items-center justify-center mx-auto mb-5">
			<svg class="w-7 h-7 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
				<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z"/>
			</svg>
		</div>
		<h1 class="text-xl font-bold text-gray-900 mb-2">
			{$page.status === 404 ? $_('error.not_found.heading') : $_('error.server.heading')}
		</h1>
		<p class="text-sm text-gray-500 leading-relaxed">
			{$page.status === 404 ? $_('error.not_found.description') : $_('error.server.description')}
		</p>
	</div>
</div>
