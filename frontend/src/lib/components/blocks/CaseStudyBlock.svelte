<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { createEventDispatcher } from 'svelte';
	import { upload, HttpError } from '$lib/api';

	export let data: { title: string; description: string; image_url: string | null };
	export let readonly = false;
	export let token = '';
	export let proposalId = '';
	export let blockId = '';

	const dispatch = createEventDispatcher<{ update: typeof data }>();

	let title = data.title ?? '';
	let description = data.description ?? '';
	let imageUrl = data.image_url ?? null;

	let uploading = false;
	let uploadError = '';
	let fileInput: HTMLInputElement;

	function notify() {
		dispatch('update', { title, description, image_url: imageUrl });
	}

	async function onFileSelected(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		if (!file) return;

		uploadError = '';
		const allowed = ['image/png', 'image/jpeg', 'image/webp'];
		if (!allowed.includes(file.type)) { uploadError = $_('case_study.invalid_type'); return; }
		if (file.size > 5 * 1024 * 1024) { uploadError = $_('case_study.too_large'); return; }

		uploading = true;
		try {
			const { upload_url, file_url } = await upload.presign(token, {
				file_type: 'case_study',
				content_type: file.type,
				size_bytes: file.size,
				proposal_id: proposalId,
				block_id: blockId
			});
			await upload.putToS3(upload_url, file);
			imageUrl = file_url;
			notify();
		} catch (err) {
			uploadError = err instanceof HttpError
				? $_('case_study.upload_failed_status', { values: { status: err.status } })
				: $_('case_study.upload_failed');
		} finally {
			uploading = false;
		}
	}

	function removeImage() { imageUrl = null; notify(); }
</script>

{#if readonly}
	{#if imageUrl}
		<img src={imageUrl} alt={title} class="w-full rounded-lg object-cover max-h-72 mb-4" />
	{/if}
	{#if title}<h3 class="font-semibold text-gray-900 mb-1">{title}</h3>{/if}
	{#if description}<p class="text-gray-600 text-sm">{description}</p>{/if}
{:else}
	<div class="space-y-4">
		<!-- Image -->
		<div>
			<label class="block text-xs font-medium text-gray-500 mb-1.5 uppercase tracking-wide">{$_('case_study.image_label')}</label>
			{#if imageUrl}
				<div class="relative group rounded-lg overflow-hidden border border-gray-200">
					<img src={imageUrl} alt="Case study" class="w-full object-cover max-h-52" />
					<div class="absolute inset-0 bg-black/0 group-hover:bg-black/30 transition-colors flex items-center justify-center opacity-0 group-hover:opacity-100">
						<button type="button" on:click={removeImage}
							class="px-3 py-1.5 bg-white text-red-600 text-sm font-medium rounded-lg shadow hover:bg-red-50">
							{$_('case_study.remove')}
						</button>
					</div>
				</div>
			{:else}
				<div class="flex items-center gap-3">
					<button type="button" on:click={() => fileInput?.click()} disabled={uploading}
						class="px-3 py-2 border border-dashed border-gray-300 rounded-lg text-sm text-gray-500 hover:border-indigo-400 hover:text-indigo-600 hover:bg-indigo-50/40 transition-colors disabled:opacity-50">
						{uploading ? $_('case_study.uploading') : $_('case_study.upload')}
					</button>
					<span class="text-xs text-gray-400">{$_('case_study.file_hint')}</span>
				</div>
				{#if uploadError}
					<p class="mt-1 text-xs text-red-600">{uploadError}</p>
				{/if}
			{/if}
			<input bind:this={fileInput} type="file" accept="image/png,image/jpeg,image/webp" class="hidden" on:change={onFileSelected} />
		</div>

		<!-- Title -->
		<div>
			<label class="block text-xs font-medium text-gray-500 mb-1 uppercase tracking-wide">{$_('case_study.title_label')}</label>
			<input type="text" bind:value={title} on:input={notify} placeholder={$_('case_study.title_placeholder')}
				class="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400 focus:border-transparent" />
		</div>

		<!-- Description -->
		<div>
			<label class="block text-xs font-medium text-gray-500 mb-1 uppercase tracking-wide">{$_('case_study.description_label')}</label>
			<textarea bind:value={description} on:input={notify} rows="3" placeholder={$_('case_study.description_placeholder')}
				class="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400 focus:border-transparent resize-none"></textarea>
		</div>
	</div>
{/if}
