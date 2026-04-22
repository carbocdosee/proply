<script lang="ts">
	import { _ } from 'svelte-i18n';
	import type { PageData } from './$types';

	export let data: PageData;

	function fmtDate(iso: string | null): string {
		if (!iso) return '';
		return new Intl.DateTimeFormat(undefined, {
			day: 'numeric',
			month: 'long',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
		}).format(new Date(iso));
	}
</script>

<svelte:head>
	<title>{$_('confirmed.page_title')}</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
	<div class="bg-white rounded-2xl shadow-sm border border-gray-100 p-10 w-full max-w-sm text-center">
		<div class="w-16 h-16 bg-green-50 rounded-2xl flex items-center justify-center mx-auto mb-6">
			<svg class="w-8 h-8 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
				<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
			</svg>
		</div>

		<h1 class="text-2xl font-bold text-gray-900 mb-3">{$_('confirmed.heading')}</h1>

		<p class="text-gray-500 leading-relaxed">
			{#if data.agencyName}
				{$_('confirmed.message_agency', { values: { agency: data.agencyName } })}
			{:else}
				{$_('confirmed.message')}
			{/if}
			<br />
			{$_('confirmed.next')}
		</p>

		{#if data.approvedAt}
			<p class="mt-5 text-xs text-gray-400">{fmtDate(data.approvedAt)}</p>
		{/if}
	</div>
</div>
