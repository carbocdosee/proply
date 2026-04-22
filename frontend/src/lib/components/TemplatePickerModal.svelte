<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { createEventDispatcher } from 'svelte';
	import { TEMPLATES } from '$lib/templates';

	export let planUsed: number = 0;
	export let planLimit: number | null = null;

	const dispatch = createEventDispatcher<{
		create: { templateId: string | null; title: string; clientName: string };
		close: void;
	}>();

	let selectedTemplateId: string | null = null;
	let title = '';
	let clientName = '';
	let step: 'pick' | 'details' = 'pick';

	const atPlanLimit = planLimit !== null && planUsed >= planLimit;

	function selectTemplate(id: string | null) {
		selectedTemplateId = id;
		step = 'details';
	}

	function goBack() {
		step = 'pick';
	}

	function handleCreate() {
		dispatch('create', {
			templateId: selectedTemplateId,
			title: title.trim(),
			clientName: clientName.trim()
		});
	}

	function handleBackdrop(e: MouseEvent) {
		if (e.target === e.currentTarget) dispatch('close');
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') dispatch('close');
	}

	$: selectedTemplate = TEMPLATES.find((t) => t.id === selectedTemplateId) ?? null;
</script>

<svelte:window on:keydown={handleKeydown} />

<!-- Backdrop -->
<div
	data-testid="template-picker-modal"
	class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
	role="dialog"
	aria-modal="true"
	aria-label={$_('template_picker.heading')}
	on:click={handleBackdrop}
>
	<div class="bg-white rounded-2xl shadow-2xl w-full max-w-2xl flex flex-col max-h-[90vh] overflow-hidden">

		<!-- Header -->
		<div class="flex items-center justify-between px-6 py-4 border-b border-gray-100">
			{#if step === 'details'}
				<div class="flex items-center gap-2">
					<button
						type="button"
						on:click={goBack}
						class="p-1 rounded-lg hover:bg-gray-100 transition-colors text-gray-400 hover:text-gray-700"
						aria-label="Back"
					>
						<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
							<path d="M15 19l-7-7 7-7"/>
						</svg>
					</button>
					<h2 class="text-base font-semibold text-gray-900">
						{selectedTemplate
							? $_('template.' + selectedTemplate.id + '.name')
							: $_('template_picker.blank_name')}
						{$_('template_picker.details_suffix')}
					</h2>
				</div>
			{:else}
				<h2 class="text-base font-semibold text-gray-900">{$_('template_picker.heading')}</h2>
			{/if}
			<button
				type="button"
				on:click={() => dispatch('close')}
				class="p-1.5 rounded-lg hover:bg-gray-100 transition-colors text-gray-400 hover:text-gray-600"
				aria-label="Close"
			>
				<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path d="M6 18L18 6M6 6l12 12"/>
				</svg>
			</button>
		</div>

		<!-- Plan limit banner -->
		{#if atPlanLimit}
			<div class="mx-6 mt-4 px-4 py-3 bg-amber-50 border border-amber-200 rounded-xl flex items-start gap-3">
				<svg class="w-4 h-4 text-amber-500 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
					<path fill-rule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd"/>
				</svg>
				<div>
					<p class="text-sm font-semibold text-amber-800">{$_('template_picker.limit.title')}</p>
					<p class="text-xs text-amber-700 mt-0.5">
						{$_('template_picker.limit.description', { values: { used: planUsed, limit: planLimit } })}
						<a href="/dashboard/billing" class="underline font-medium">{$_('template_picker.limit.cta')}</a>
					</p>
				</div>
			</div>
		{/if}

		<!-- Step: Template picker -->
		{#if step === 'pick'}
			<div class="flex-1 overflow-y-auto p-6">
				<p class="text-sm text-gray-500 mb-5">{$_('template_picker.description')}</p>

				<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
					<!-- Blank option -->
					<button
						data-testid="template-card-blank"
						type="button"
						on:click={() => selectTemplate(null)}
						disabled={atPlanLimit}
						class="text-left p-4 rounded-xl border-2 border-dashed border-gray-200 hover:border-indigo-300 hover:bg-indigo-50/30 transition-all group disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:border-gray-200 disabled:hover:bg-transparent"
					>
						<div class="flex items-center gap-3 mb-2">
							<div class="w-9 h-9 rounded-lg bg-gray-100 flex items-center justify-center text-lg group-hover:bg-indigo-100 transition-colors">
								📄
							</div>
							<span class="font-medium text-gray-900 text-sm">{$_('template_picker.blank_name')}</span>
						</div>
						<p class="text-xs text-gray-500 leading-relaxed">{$_('template_picker.blank_description')}</p>
					</button>

					<!-- Template cards -->
					{#each TEMPLATES as tpl}
						<button
							data-testid="template-card-{tpl.id}"
							type="button"
							on:click={() => selectTemplate(tpl.id)}
							disabled={atPlanLimit}
							class="text-left p-4 rounded-xl border-2 border-gray-100 hover:border-indigo-300 hover:shadow-sm transition-all group disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:border-gray-100 disabled:hover:shadow-none"
						>
							<div class="flex items-center gap-3 mb-2">
								<div class="w-9 h-9 rounded-lg {tpl.color} flex items-center justify-center text-lg flex-shrink-0">
									{tpl.icon}
								</div>
								<span class="font-medium text-gray-900 text-sm">{$_('template.' + tpl.id + '.name')}</span>
							</div>
							<p class="text-xs text-gray-500 leading-relaxed mb-3">{$_('template.' + tpl.id + '.description')}</p>
							<div class="flex flex-wrap gap-1">
								{#each tpl.blockTypes as bt}
									<span class="px-1.5 py-0.5 text-[10px] font-medium rounded bg-gray-100 text-gray-500">
										{$_('block_type.' + bt)}
									</span>
								{/each}
							</div>
						</button>
					{/each}
				</div>
			</div>

		<!-- Step: Proposal details -->
		{:else}
			<form
				class="flex-1 overflow-y-auto p-6"
				on:submit|preventDefault={handleCreate}
			>
				<!-- Template summary -->
				{#if selectedTemplate}
					<div class="flex items-center gap-3 p-3 bg-gray-50 rounded-xl mb-5">
						<div class="w-9 h-9 rounded-lg {selectedTemplate.color} flex items-center justify-center text-lg flex-shrink-0">
							{selectedTemplate.icon}
						</div>
						<div>
							<p class="text-sm font-medium text-gray-900">{$_('template.' + selectedTemplate.id + '.name')}</p>
							<div class="flex flex-wrap gap-1 mt-0.5">
								{#each selectedTemplate.blockTypes as bt}
									<span class="px-1.5 py-0.5 text-[10px] font-medium rounded bg-white border border-gray-200 text-gray-500">
										{$_('block_type.' + bt)}
									</span>
								{/each}
							</div>
						</div>
					</div>
				{:else}
					<div class="flex items-center gap-3 p-3 bg-gray-50 rounded-xl mb-5">
						<div class="w-9 h-9 rounded-lg bg-gray-100 flex items-center justify-center text-lg">📄</div>
						<p class="text-sm font-medium text-gray-900">{$_('template_picker.blank_name')}</p>
					</div>
				{/if}

				<div class="space-y-4">
					<div>
						<label for="modal-title" class="block text-sm font-medium text-gray-700 mb-1">
							{$_('template_picker.title_label')} <span class="text-gray-400 font-normal">{$_('template_picker.title_optional')}</span>
						</label>
						<input
							data-testid="picker-title"
							id="modal-title"
							type="text"
							bind:value={title}
							placeholder={$_('template_picker.title_placeholder')}
							class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
						/>
					</div>
					<div>
						<label for="modal-client" class="block text-sm font-medium text-gray-700 mb-1">
							{$_('template_picker.client_label')} <span class="text-gray-400 font-normal">{$_('template_picker.client_optional')}</span>
						</label>
						<input
							data-testid="picker-client"
							id="modal-client"
							type="text"
							bind:value={clientName}
							placeholder={$_('template_picker.client_placeholder')}
							class="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
						/>
					</div>
				</div>

				<div class="mt-6 flex gap-3">
					<button
						type="button"
						on:click={goBack}
						class="px-4 py-2 border border-gray-200 text-sm font-medium rounded-lg hover:bg-gray-50 transition-colors"
					>
						{$_('template_picker.back')}
					</button>
					<button
						data-testid="picker-create-btn"
						type="submit"
						class="flex-1 px-4 py-2 bg-indigo-600 text-white text-sm font-semibold rounded-lg hover:bg-indigo-700 transition-colors"
					>
						{$_('template_picker.create')}
					</button>
				</div>
			</form>
		{/if}

	</div>
</div>
