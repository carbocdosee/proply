<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { createEventDispatcher } from 'svelte';
	import { upload, HttpError } from '$lib/api';

	export let data: { name: string; role: string; bio: string; photo_url: string | null };
	export let readonly = false;
	export let token = '';
	export let proposalId = '';
	export let blockId = '';

	const dispatch = createEventDispatcher<{ update: typeof data }>();

	let name = data.name ?? '';
	let role = data.role ?? '';
	let bio = data.bio ?? '';
	let photoUrl = data.photo_url ?? null;

	let uploading = false;
	let uploadError = '';
	let fileInput: HTMLInputElement;

	function notify() {
		dispatch('update', { name, role, bio, photo_url: photoUrl });
	}

	async function onFileSelected(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		if (!file) return;

		uploadError = '';
		const allowed = ['image/png', 'image/jpeg', 'image/webp'];
		if (!allowed.includes(file.type)) { uploadError = $_('team_member.invalid_type'); return; }
		if (file.size > 2 * 1024 * 1024) { uploadError = $_('team_member.too_large'); return; }

		uploading = true;
		try {
			const { upload_url, file_url } = await upload.presign(token, {
				file_type: 'team_member',
				content_type: file.type,
				size_bytes: file.size,
				proposal_id: proposalId,
				block_id: blockId
			});
			await upload.putToS3(upload_url, file);
			photoUrl = file_url;
			notify();
		} catch (err) {
			uploadError = err instanceof HttpError
				? $_('team_member.upload_failed_status', { values: { status: err.status } })
				: $_('team_member.upload_failed');
		} finally {
			uploading = false;
		}
	}

	function removePhoto() { photoUrl = null; notify(); }
</script>

{#if readonly}
	<div class="flex items-start gap-4">
		{#if photoUrl}
			<img src={photoUrl} alt={name} class="w-16 h-16 rounded-full object-cover flex-shrink-0" />
		{:else}
			<div class="w-16 h-16 rounded-full bg-gray-100 flex items-center justify-center flex-shrink-0 text-gray-400 text-xl">
				{name?.[0]?.toUpperCase() ?? '?'}
			</div>
		{/if}
		<div>
			{#if name}<p class="font-semibold text-gray-900">{name}</p>{/if}
			{#if role}<p class="text-sm text-indigo-600 mb-1">{role}</p>{/if}
			{#if bio}<p class="text-sm text-gray-600">{bio}</p>{/if}
		</div>
	</div>
{:else}
	<div class="flex items-start gap-5">
		<!-- Photo -->
		<div class="flex-shrink-0">
			{#if photoUrl}
				<div class="relative group">
					<img src={photoUrl} alt="Team member photo" class="w-20 h-20 rounded-full object-cover" />
					<button type="button" on:click={removePhoto}
						class="absolute inset-0 rounded-full bg-black/0 group-hover:bg-black/40 text-white text-xs opacity-0 group-hover:opacity-100 flex items-center justify-center transition-all">
						{$_('team_member.remove')}
					</button>
				</div>
			{:else}
				<button type="button" on:click={() => fileInput?.click()} disabled={uploading}
					class="w-20 h-20 rounded-full border-2 border-dashed border-gray-300 hover:border-indigo-400 flex flex-col items-center justify-center gap-1 text-gray-400 hover:text-indigo-500 transition-colors disabled:opacity-50">
					{#if uploading}
						<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"/>
						</svg>
					{:else}
						<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
							<path d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5"/>
						</svg>
						<span class="text-[10px] leading-tight text-center">{$_('team_member.photo_label')}</span>
					{/if}
				</button>
			{/if}
			{#if uploadError}
				<p class="mt-1 text-xs text-red-600 w-20 text-center">{uploadError}</p>
			{/if}
			<input bind:this={fileInput} type="file" accept="image/png,image/jpeg,image/webp" class="hidden" on:change={onFileSelected} />
		</div>

		<!-- Fields -->
		<div class="flex-1 space-y-3">
			<input type="text" bind:value={name} on:input={notify} placeholder={$_('team_member.name_placeholder')}
				class="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm font-semibold focus:outline-none focus:ring-2 focus:ring-indigo-400" />
			<input type="text" bind:value={role} on:input={notify} placeholder={$_('team_member.role_placeholder')}
				class="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm text-indigo-600 focus:outline-none focus:ring-2 focus:ring-indigo-400" />
			<textarea bind:value={bio} on:input={notify} rows="3" placeholder={$_('team_member.bio_placeholder')}
				class="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400 resize-none"></textarea>
		</div>
	</div>
{/if}
