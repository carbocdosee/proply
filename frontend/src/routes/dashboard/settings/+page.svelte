<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { locale } from '$lib/i18n';
	import { authStore, currentPlan } from '$lib/stores/auth';
	import { account, upload, HttpError } from '$lib/api';

	// ── Profile state ──────────────────────────────────────────────────
	let name = $authStore.user?.name ?? '';
	let language = $authStore.user?.language ?? 'en';
	let profileSaving = false;
	let profileSaved = false;
	let profileError = '';

	// ── Branding state ─────────────────────────────────────────────────
	let primaryColor = $authStore.user?.primary_color ?? '#6366F1';
	let accentColor = $authStore.user?.accent_color ?? '#F59E0B';
	let hideFooter = $authStore.user?.hide_proply_footer ?? false;
	let logoPreview: string | null = $authStore.user?.logo_url ?? null;
	let pendingLogoFile: File | null = null;
	let pendingLogoUrl: string | null = null; // URL returned by S3 after upload
	let brandingSaving = false;
	let brandingSaved = false;
	let brandingError = '';

	// ── Logo upload state ──────────────────────────────────────────────
	let logoUploading = false;
	let logoError = '';
	let fileInput: HTMLInputElement;

	// ── Paywall modal ──────────────────────────────────────────────────
	let showPaywall = false;

	// ── Color input sync ──────────────────────────────────────────────
	// Keep hex text inputs valid: revert to last valid color on blur if invalid
	let primaryColorText = primaryColor;
	let accentColorText = accentColor;

	function onPrimaryColorInput() {
		if (/^#[0-9A-Fa-f]{6}$/.test(primaryColorText)) {
			primaryColor = primaryColorText;
		}
	}

	function onPrimaryColorBlur() {
		if (!/^#[0-9A-Fa-f]{6}$/.test(primaryColorText)) {
			primaryColorText = primaryColor;
		}
	}

	function onAccentColorInput() {
		if (/^#[0-9A-Fa-f]{6}$/.test(accentColorText)) {
			accentColor = accentColorText;
		}
	}

	function onAccentColorBlur() {
		if (!/^#[0-9A-Fa-f]{6}$/.test(accentColorText)) {
			accentColorText = accentColor;
		}
	}

	// Sync text input when native color picker changes
	function onPrimaryPickerChange(e: Event) {
		primaryColor = (e.target as HTMLInputElement).value;
		primaryColorText = primaryColor;
	}

	function onAccentPickerChange(e: Event) {
		accentColor = (e.target as HTMLInputElement).value;
		accentColorText = accentColor;
	}

	// ── Hide footer toggle ─────────────────────────────────────────────
	function handleHideFooterChange(e: Event) {
		const checked = (e.target as HTMLInputElement).checked;
		if (checked && $currentPlan === 'free') {
			// Revert the checkbox — show paywall instead
			hideFooter = false;
			showPaywall = true;
		} else {
			hideFooter = checked;
		}
	}

	// ── Logo file selection ────────────────────────────────────────────
	function triggerFilePicker() {
		fileInput?.click();
	}

	function onFileSelected(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		if (!file) return;

		logoError = '';

		const allowedTypes = ['image/png', 'image/jpeg', 'image/webp'];
		if (!allowedTypes.includes(file.type)) {
			logoError = $_('settings.logo_invalid_type');
			return;
		}
		if (file.size > 2 * 1024 * 1024) {
			logoError = $_('settings.logo_too_large');
			return;
		}

		// Show local preview immediately
		logoPreview = URL.createObjectURL(file);
		pendingLogoFile = file;
		pendingLogoUrl = null; // will be set after upload
	}

	async function uploadLogo(): Promise<string | null> {
		if (!pendingLogoFile) return null;
		const token = $authStore.accessToken;
		if (!token) return null;

		logoUploading = true;
		logoError = '';

		try {
			const { upload_url, file_url } = await upload.presign(token, {
				file_type: 'logo',
				content_type: pendingLogoFile.type,
				size_bytes: pendingLogoFile.size
			});

			await upload.putToS3(upload_url, pendingLogoFile);

			pendingLogoUrl = file_url;
			pendingLogoFile = null;
			return file_url;
		} catch (err) {
			if (err instanceof HttpError) {
				if (err.status === 400) logoError = $_('settings.logo_too_large_api');
				else if (err.status === 422) logoError = $_('settings.logo_unsupported');
				else if (err.status === 503) logoError = $_('settings.logo_storage_off');
				else logoError = $_('settings.logo_upload_failed');
			} else {
				logoError = $_('settings.logo_upload_failed');
			}
			return null;
		} finally {
			logoUploading = false;
		}
	}

	function removeLogo() {
		logoPreview = null;
		pendingLogoFile = null;
		pendingLogoUrl = null;
		if (fileInput) fileInput.value = '';
	}

	// ── Profile save ──────────────────────────────────────────────────
	async function handleProfileSave() {
		const token = $authStore.accessToken;
		if (!token) return;

		profileSaving = true;
		profileError = '';

		try {
			await account.updateProfile(token, { name, language });
			authStore.patchUser({ name, language: language as 'en' | 'ru' });
			locale.set(language);
			profileSaved = true;
			setTimeout(() => (profileSaved = false), 3000);
		} catch {
			profileError = $_('settings.profile_save_failed');
		} finally {
			profileSaving = false;
		}
	}

	// ── GDPR state ────────────────────────────────────────────────────
	let retentionMonths: 12 | 24 | 36 = ($authStore.user?.data_retention_months as 12 | 24 | 36) ?? 12;
	let retentionSaving = false;
	let retentionSaved = false;
	let retentionError = '';

	let exportLoading = false;
	let exportError = '';

	let deleteConfirmText = '';
	let deleteLoading = false;
	let deleteError = '';

	async function handleRetentionSave() {
		const token = $authStore.accessToken;
		if (!token) return;
		retentionSaving = true;
		retentionError = '';
		try {
			await account.updateRetention(token, retentionMonths);
			authStore.patchUser({ data_retention_months: retentionMonths });
			retentionSaved = true;
			setTimeout(() => (retentionSaved = false), 3000);
		} catch {
			retentionError = $_('settings.retention_failed');
		} finally {
			retentionSaving = false;
		}
	}

	async function handleExportData() {
		const token = $authStore.accessToken;
		if (!token) return;
		exportLoading = true;
		exportError = '';
		try {
			const res = await account.exportData(token);
			if (!res.ok) throw new Error('export failed');
			const blob = await res.blob();
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = 'proply-export.json';
			a.click();
			URL.revokeObjectURL(url);
		} catch {
			exportError = $_('settings.export.failed');
		} finally {
			exportLoading = false;
		}
	}

	async function handleDeleteAccount() {
		const token = $authStore.accessToken;
		if (!token || deleteConfirmText !== 'DELETE') return;
		deleteLoading = true;
		deleteError = '';
		try {
			await account.deleteAccount(token);
			authStore.logout();
			window.location.href = '/';
		} catch {
			deleteError = $_('settings.delete.failed');
			deleteLoading = false;
		}
	}

	// ── Branding save ─────────────────────────────────────────────────
	async function handleBrandingSave() {
		const token = $authStore.accessToken;
		if (!token) return;

		brandingSaving = true;
		brandingError = '';

		try {
			// Upload logo if a new file is pending
			let logoUrl: string | null | undefined = undefined; // undefined = no change
			if (pendingLogoFile) {
				const uploaded = await uploadLogo();
				if (logoError) {
					brandingSaving = false;
					return;
				}
				logoUrl = uploaded;
			} else if (logoPreview === null && $authStore.user?.logo_url) {
				// User removed the logo
				logoUrl = null;
			}

			await account.updateBranding(token, {
				...(logoUrl !== undefined ? { logo_url: logoUrl } : {}),
				primary_color: primaryColor,
				accent_color: accentColor,
				hide_proply_footer: hideFooter
			});

			authStore.patchUser({
				...(logoUrl !== undefined ? { logo_url: logoUrl ?? undefined } : {}),
				primary_color: primaryColor,
				accent_color: accentColor,
				hide_proply_footer: hideFooter
			});

			brandingSaved = true;
			setTimeout(() => (brandingSaved = false), 3000);
		} catch (err) {
			if (err instanceof HttpError && err.status === 402) {
				hideFooter = false;
				showPaywall = true;
			} else if (err instanceof HttpError && err.status === 422) {
				brandingError = $_('settings.color_invalid');
			} else {
				brandingError = $_('settings.branding_save_failed');
			}
		} finally {
			brandingSaving = false;
		}
	}
</script>

<svelte:head>
	<title>{$_('settings.page_title')}</title>
</svelte:head>

<!-- Hidden file input for logo upload -->
<input
	bind:this={fileInput}
	data-testid="logo-file-input"
	type="file"
	accept="image/png,image/jpeg,image/webp"
	class="hidden"
	on:change={onFileSelected}
/>

<!-- Paywall modal -->
{#if showPaywall}
	<div
		data-testid="paywall-modal"
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
		role="dialog"
		aria-modal="true"
	>
		<div class="bg-white rounded-2xl shadow-xl p-8 max-w-sm w-full mx-4">
			<div class="text-3xl mb-3">✨</div>
			<h2 class="text-lg font-bold text-gray-900 mb-2">{$_('settings.paywall.title')}</h2>
			<p class="text-sm text-gray-500 mb-6">{$_('settings.paywall.description')}</p>
			<div class="flex gap-3">
				<a
					href="/dashboard/billing"
					class="flex-1 text-center px-4 py-2 bg-indigo-600 text-white text-sm font-semibold rounded-lg hover:bg-indigo-700 transition-colors"
				>
					{$_('settings.paywall.cta')}
				</a>
				<button
					type="button"
					class="flex-1 px-4 py-2 border border-gray-200 text-sm font-medium rounded-lg hover:bg-gray-50 transition-colors"
					on:click={() => (showPaywall = false)}
				>
					{$_('settings.paywall.later')}
				</button>
			</div>
		</div>
	</div>
{/if}

<div class="max-w-2xl mx-auto">
	<h1 class="text-2xl font-bold text-gray-900 mb-8">{$_('settings.heading')}</h1>

	<div class="space-y-8">

		<!-- Profile section -->
		<section class="bg-white rounded-xl border border-gray-100 p-6">
			<h2 class="text-base font-semibold text-gray-900 mb-4">{$_('settings.profile.heading')}</h2>
			<div class="space-y-4">
				<div>
					<label for="settings-name" class="block text-sm font-medium text-gray-700 mb-1">{$_('settings.name_label')}</label>
					<input
						id="settings-name"
						type="text"
						bind:value={name}
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 text-sm"
					/>
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-1">{$_('settings.email_label')}</label>
					<input
						type="email"
						value={$authStore.user?.email ?? ''}
						disabled
						class="w-full px-3 py-2 border border-gray-200 rounded-lg bg-gray-50 text-gray-500 text-sm cursor-not-allowed"
					/>
				</div>
				<div>
					<label for="settings-language" class="block text-sm font-medium text-gray-700 mb-1">{$_('settings.language_label')}</label>
					<select
						id="settings-language"
						bind:value={language}
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 text-sm"
					>
						<option value="en">{$_('settings.language.en')}</option>
						<option value="ru">{$_('settings.language.ru')}</option>
					</select>
				</div>
			</div>

			{#if profileError}
				<p class="mt-3 text-sm text-red-600">{profileError}</p>
			{/if}

			<div class="mt-5 flex items-center justify-end gap-3">
				{#if profileSaved}
					<span class="text-sm text-green-600 font-medium">{$_('settings.profile_saved')}</span>
				{/if}
				<button
					type="button"
					disabled={profileSaving}
					on:click={handleProfileSave}
					class="px-4 py-2 bg-indigo-600 text-white text-sm font-semibold rounded-lg hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
				>
					{profileSaving ? $_('settings.profile_saving') : $_('settings.save_profile')}
				</button>
			</div>
		</section>

		<!-- Branding section -->
		<section class="bg-white rounded-xl border border-gray-100 p-6">
			<h2 class="text-base font-semibold text-gray-900 mb-1">{$_('settings.branding.heading')}</h2>
			<p class="text-sm text-gray-500 mb-5">{$_('settings.branding.subtitle')}</p>

			<div class="space-y-6">

				<!-- Logo -->
				<div>
					<label class="block text-sm font-medium text-gray-700 mb-2">{$_('settings.logo_label')}</label>
					<div class="flex items-center gap-4">
						<!-- Preview -->
						<div class="w-16 h-16 rounded-xl border border-gray-200 bg-gray-50 flex items-center justify-center overflow-hidden flex-shrink-0">
							{#if logoPreview}
								<img data-testid="logo-preview-img" src={logoPreview} alt="Agency logo" class="w-full h-full object-contain p-1" />
							{:else}
								<span class="text-gray-300 text-xs text-center leading-tight">{$_('settings.logo_empty')}</span>
							{/if}
						</div>

						<!-- Actions -->
						<div class="flex flex-col gap-2">
							<button
								data-testid="logo-upload-btn"
								type="button"
								disabled={logoUploading}
								on:click={triggerFilePicker}
								class="px-3 py-1.5 text-sm font-medium border border-gray-200 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
							>
								{logoUploading ? $_('settings.logo_uploading') : $_('settings.logo_upload')}
							</button>
							{#if logoPreview}
								<button
									data-testid="logo-remove-btn"
									type="button"
									on:click={removeLogo}
									class="px-3 py-1.5 text-sm font-medium text-red-600 border border-red-100 rounded-lg hover:bg-red-50 transition-colors"
								>
									{$_('settings.logo_remove')}
								</button>
							{/if}
							<p class="text-xs text-gray-400">{$_('settings.logo_hint')}</p>
						</div>
					</div>

					{#if logoError}
						<p data-testid="logo-error" class="mt-2 text-sm text-red-600">{logoError}</p>
					{/if}

					{#if pendingLogoFile}
						<p class="mt-2 text-xs text-amber-600 font-medium">{$_('settings.logo_unsaved')}</p>
					{/if}
				</div>

				<!-- Colors -->
				<div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-2">{$_('settings.primary_color')}</label>
						<div class="flex items-center gap-2">
							<input
								type="color"
								value={primaryColor}
								on:input={onPrimaryPickerChange}
								class="w-10 h-10 rounded-lg border border-gray-200 cursor-pointer p-0.5 flex-shrink-0"
							/>
							<input
								data-testid="primary-color-text"
								type="text"
								bind:value={primaryColorText}
								maxlength="7"
								placeholder="#6366F1"
								on:input={onPrimaryColorInput}
								on:blur={onPrimaryColorBlur}
								class="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500"
							/>
						</div>
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 mb-2">{$_('settings.accent_color')}</label>
						<div class="flex items-center gap-2">
							<input
								type="color"
								value={accentColor}
								on:input={onAccentPickerChange}
								class="w-10 h-10 rounded-lg border border-gray-200 cursor-pointer p-0.5 flex-shrink-0"
							/>
							<input
								data-testid="accent-color-text"
								type="text"
								bind:value={accentColorText}
								maxlength="7"
								placeholder="#F59E0B"
								on:input={onAccentColorInput}
								on:blur={onAccentColorBlur}
								class="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500"
							/>
						</div>
					</div>
				</div>

				<!-- Hide footer toggle -->
				<div class="flex items-start justify-between gap-4 py-4 border-t border-gray-100">
					<div class="flex-1">
						<div class="flex items-center gap-2">
							<p class="text-sm font-medium text-gray-700">{$_('settings.hide_footer_label')}</p>
							{#if $currentPlan === 'free'}
								<span class="px-1.5 py-0.5 text-xs font-semibold bg-indigo-50 text-indigo-600 rounded">{$_('settings.pro_badge')}</span>
							{/if}
						</div>
						<p class="text-xs text-gray-400 mt-0.5">{$_('settings.hide_footer_desc')}</p>
					</div>
					<label class="relative inline-flex items-center cursor-pointer flex-shrink-0 mt-0.5">
						<input
							data-testid="hide-footer-checkbox"
							type="checkbox"
							bind:checked={hideFooter}
							on:change={handleHideFooterChange}
							class="sr-only peer"
						/>
						<div
							class="w-10 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-indigo-500 rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-indigo-600"
						></div>
					</label>
				</div>

			</div>

			{#if brandingError}
				<p data-testid="branding-error" class="mt-3 text-sm text-red-600">{brandingError}</p>
			{/if}

			<div class="mt-5 flex items-center justify-end gap-3">
				{#if brandingSaved}
					<span data-testid="branding-saved" class="text-sm text-green-600 font-medium">{$_('settings.branding_saved')}</span>
				{/if}
				<button
					data-testid="save-branding-btn"
					type="button"
					disabled={brandingSaving || logoUploading}
					on:click={handleBrandingSave}
					class="px-4 py-2 bg-indigo-600 text-white text-sm font-semibold rounded-lg hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
				>
					{brandingSaving ? $_('settings.branding_saving') : $_('settings.save_branding')}
				</button>
			</div>
		</section>

		<!-- Danger zone / plan info -->
		<section class="bg-white rounded-xl border border-gray-100 p-6">
			<h2 class="text-base font-semibold text-gray-900 mb-1">{$_('settings.plan.heading')}</h2>
			<div class="flex items-center justify-between">
				<div>
					<p class="text-sm text-gray-600">
						{$_('settings.plan.current')} <span class="font-semibold capitalize text-gray-900">{$authStore.user?.plan ?? 'free'}</span>
					</p>
				</div>
				{#if $currentPlan === 'free'}
					<a
						href="/dashboard/billing"
						class="px-4 py-2 bg-indigo-600 text-white text-sm font-semibold rounded-lg hover:bg-indigo-700 transition-colors"
					>
						{$_('settings.plan.upgrade')}
					</a>
				{/if}
			</div>
		</section>

		<!-- Data & Privacy section -->
		<section class="bg-white rounded-xl border border-gray-100 p-6">
			<h2 class="text-base font-semibold text-gray-900 mb-1">{$_('settings.privacy.heading')}</h2>
			<p class="text-sm text-gray-500 mb-6">{$_('settings.privacy.subtitle')}</p>

			<div class="space-y-6">

				<!-- Data retention -->
				<div>
					<label for="settings-retention" class="block text-sm font-medium text-gray-700 mb-1">
						{$_('settings.retention_label')}
					</label>
					<p class="text-xs text-gray-400 mb-2">{$_('settings.retention_hint')}</p>
					<div class="flex items-center gap-3">
						<select
							id="settings-retention"
							bind:value={retentionMonths}
							class="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
						>
							<option value={12}>{$_('settings.retention.12')}</option>
							<option value={24}>{$_('settings.retention.24')}</option>
							<option value={36}>{$_('settings.retention.36')}</option>
						</select>
						<button
							type="button"
							disabled={retentionSaving}
							on:click={handleRetentionSave}
							class="px-4 py-2 bg-indigo-600 text-white text-sm font-semibold rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition-colors"
						>
							{retentionSaving ? $_('settings.retention_saving') : $_('settings.retention_save')}
						</button>
						{#if retentionSaved}
							<span class="text-sm text-green-600 font-medium">{$_('settings.retention_saved')}</span>
						{/if}
					</div>
					{#if retentionError}
						<p class="mt-2 text-sm text-red-600">{retentionError}</p>
					{/if}
				</div>

				<!-- Export data -->
				<div class="py-5 border-t border-gray-100">
					<p class="text-sm font-medium text-gray-700 mb-1">{$_('settings.export.heading')}</p>
					<p class="text-xs text-gray-400 mb-3">{$_('settings.export.description')}</p>
					<button
						type="button"
						disabled={exportLoading}
						on:click={handleExportData}
						class="px-4 py-2 border border-gray-200 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-50 disabled:opacity-50 transition-colors"
					>
						{exportLoading ? $_('settings.export.loading') : $_('settings.export.cta')}
					</button>
					{#if exportError}
						<p class="mt-2 text-sm text-red-600">{exportError}</p>
					{/if}
				</div>

				<!-- Delete account -->
				<div class="py-5 border-t border-red-100">
					<p class="text-sm font-semibold text-red-700 mb-1">{$_('settings.delete.heading')}</p>
					<p class="text-xs text-gray-500 mb-4">{$_('settings.delete.description')}</p>
					<div class="space-y-3">
						<div>
							<label for="delete-confirm" class="block text-xs text-gray-500 mb-1">
								{$_('settings.delete.confirm_label')}
							</label>
							<input
								id="delete-confirm"
								type="text"
								bind:value={deleteConfirmText}
								placeholder="DELETE"
								class="w-full max-w-xs px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-red-400"
							/>
						</div>
						<button
							type="button"
							disabled={deleteConfirmText !== 'DELETE' || deleteLoading}
							on:click={handleDeleteAccount}
							class="px-4 py-2 bg-red-600 text-white text-sm font-semibold rounded-lg hover:bg-red-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
						>
							{deleteLoading ? $_('settings.delete.deleting') : $_('settings.delete.cta')}
						</button>
						{#if deleteError}
							<p class="text-sm text-red-600">{deleteError}</p>
						{/if}
					</div>
				</div>

			</div>
		</section>

	</div>
</div>
