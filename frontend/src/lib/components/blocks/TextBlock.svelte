<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { onMount, onDestroy, createEventDispatcher } from 'svelte';

	export let data: { html: string };
	export let readonly = false;

	const dispatch = createEventDispatcher<{ update: { html: string } }>();

	let editorEl: HTMLDivElement;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	let editor: any;

	let activeStates = { bold: false, italic: false, bulletList: false, orderedList: false, link: false };
	let linkInputVisible = false;
	let linkUrl = '';

	onMount(async () => {
		if (readonly) return;

		const [{ Editor }, { default: StarterKit }, { Link }] = await Promise.all([
			import('@tiptap/core'),
			import('@tiptap/starter-kit'),
			import('@tiptap/extension-link')
		]);

		editor = new Editor({
			element: editorEl,
			extensions: [
				StarterKit,
				Link.configure({ openOnClick: false, HTMLAttributes: { rel: 'noopener noreferrer', target: '_blank' } })
			],
			content: data.html || '<p></p>',
			editorProps: {
				attributes: {
					class: 'prose prose-sm max-w-none focus:outline-none min-h-[80px] px-1 py-1'
				}
			},
			onUpdate({ editor: e }) {
				dispatch('update', { html: e.getHTML() });
			},
			onTransaction() {
				// Trigger Svelte reactivity for isActive checks
				activeStates = {
					bold: editor?.isActive('bold') ?? false,
					italic: editor?.isActive('italic') ?? false,
					bulletList: editor?.isActive('bulletList') ?? false,
					orderedList: editor?.isActive('orderedList') ?? false,
					link: editor?.isActive('link') ?? false
				};
			}
		});
	});

	onDestroy(() => editor?.destroy());

	function toggleBold() { editor?.chain().focus().toggleBold().run(); }
	function toggleItalic() { editor?.chain().focus().toggleItalic().run(); }
	function toggleBulletList() { editor?.chain().focus().toggleBulletList().run(); }
	function toggleOrderedList() { editor?.chain().focus().toggleOrderedList().run(); }

	function handleLinkClick() {
		if (activeStates.link) {
			editor?.chain().focus().unsetLink().run();
		} else {
			linkUrl = editor?.getAttributes('link').href ?? 'https://';
			linkInputVisible = true;
		}
	}

	function applyLink(e: Event) {
		e.preventDefault();
		if (linkUrl) editor?.chain().focus().setLink({ href: linkUrl }).run();
		linkInputVisible = false;
		linkUrl = '';
	}

	function cancelLink() {
		linkInputVisible = false;
		linkUrl = '';
	}

	function toolbarBtn(active: boolean) {
		return `p-1.5 rounded text-sm font-medium transition-colors ${active
			? 'bg-indigo-100 text-indigo-700'
			: 'text-gray-500 hover:bg-gray-100 hover:text-gray-800'}`;
	}
</script>

{#if readonly}
	<!-- Readonly render -->
	<div class="prose prose-sm max-w-none px-1">{@html data.html}</div>
{:else}
	<div class="border border-gray-200 rounded-lg overflow-hidden focus-within:border-indigo-400 focus-within:ring-1 focus-within:ring-indigo-300 transition-colors">
		<!-- Toolbar -->
		<div class="flex items-center gap-0.5 px-2 py-1.5 border-b border-gray-100 bg-gray-50 flex-wrap">
			<button type="button" on:click={toggleBold} class={toolbarBtn(activeStates.bold)} title={$_('text_block.bold')}>
				<svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24"><path d="M15.6 10.79c.97-.67 1.65-1.77 1.65-2.79 0-2.26-1.75-4-4-4H7v14h7.04c2.09 0 3.71-1.7 3.71-3.79 0-1.52-.86-2.82-2.15-3.42zM10 6.5h3c.83 0 1.5.67 1.5 1.5s-.67 1.5-1.5 1.5h-3v-3zm3.5 9H10v-3h3.5c.83 0 1.5.67 1.5 1.5s-.67 1.5-1.5 1.5z"/></svg>
			</button>
			<button type="button" on:click={toggleItalic} class={toolbarBtn(activeStates.italic)} title={$_('text_block.italic')}>
				<svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24"><path d="M10 4v3h2.21l-3.42 8H6v3h8v-3h-2.21l3.42-8H18V4h-8z"/></svg>
			</button>
			<div class="w-px h-4 bg-gray-200 mx-1"></div>
			<button type="button" on:click={toggleBulletList} class={toolbarBtn(activeStates.bulletList)} title={$_('text_block.bullet_list')}>
				<svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24"><path d="M4 10.5c-.83 0-1.5.67-1.5 1.5s.67 1.5 1.5 1.5 1.5-.67 1.5-1.5-.67-1.5-1.5-1.5zm0-6c-.83 0-1.5.67-1.5 1.5S3.17 7.5 4 7.5 5.5 6.83 5.5 6 4.83 4.5 4 4.5zm0 12c-.83 0-1.5.68-1.5 1.5s.68 1.5 1.5 1.5 1.5-.68 1.5-1.5-.67-1.5-1.5-1.5zM7 19h14v-2H7v2zm0-6h14v-2H7v2zm0-8v2h14V5H7z"/></svg>
			</button>
			<button type="button" on:click={toggleOrderedList} class={toolbarBtn(activeStates.orderedList)} title={$_('text_block.ordered_list')}>
				<svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24"><path d="M2 17h2v.5H3v1h1v.5H2v1h3v-4H2v1zm1-9h1V4H2v1h1v3zm-1 3h1.8L2 13.1v.9h3v-1H3.2L5 10.9V10H2v1zm5-6v2h14V5H7zm0 14h14v-2H7v2zm0-6h14v-2H7v2z"/></svg>
			</button>
			<div class="w-px h-4 bg-gray-200 mx-1"></div>
			<button type="button" on:click={handleLinkClick} class={toolbarBtn(activeStates.link)} title={activeStates.link ? $_('text_block.remove_link') : $_('text_block.add_link')}>
				<svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24"><path d="M3.9 12c0-1.71 1.39-3.1 3.1-3.1h4V7H7c-2.76 0-5 2.24-5 5s2.24 5 5 5h4v-1.9H7c-1.71 0-3.1-1.39-3.1-3.1zM8 13h8v-2H8v2zm9-6h-4v1.9h4c1.71 0 3.1 1.39 3.1 3.1s-1.39 3.1-3.1 3.1h-4V17h4c2.76 0 5-2.24 5-5s-2.24-5-5-5z"/></svg>
			</button>

			{#if linkInputVisible}
				<form on:submit={applyLink} class="flex items-center gap-1 ml-2">
					<input
						type="url"
						bind:value={linkUrl}
						placeholder={$_('text_block.link_placeholder')}
						class="text-xs px-2 py-1 border border-gray-300 rounded focus:outline-none focus:ring-1 focus:ring-indigo-400 w-40"
						autofocus
					/>
					<button type="submit" class="text-xs px-2 py-1 bg-indigo-600 text-white rounded hover:bg-indigo-700">{$_('text_block.apply')}</button>
					<button type="button" on:click={cancelLink} class="text-xs px-2 py-1 text-gray-500 hover:text-gray-700">✕</button>
				</form>
			{/if}
		</div>

		<!-- Editor area -->
		<div bind:this={editorEl} class="px-3 py-2 min-h-[80px]"></div>
	</div>
{/if}

<style>
	/* Bring ProseMirror list styles back since Tailwind resets them */
	:global(.ProseMirror ul) { list-style-type: disc; padding-left: 1.5rem; }
	:global(.ProseMirror ol) { list-style-type: decimal; padding-left: 1.5rem; }
	:global(.ProseMirror p.is-editor-empty:first-child::before) {
		content: attr(data-placeholder);
		float: left;
		color: #adb5bd;
		pointer-events: none;
		height: 0;
	}
	:global(.ProseMirror a) { color: #4f46e5; text-decoration: underline; }
</style>
