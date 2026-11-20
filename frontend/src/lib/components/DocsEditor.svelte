<script lang="ts">
	import { RequestService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import DOMPurify from 'dompurify';
	import { marked } from 'marked';
	import type { CollectionDoc, FolderDoc } from '#lib/types';

	let {
		kind, // 'collection' | 'folder'
		collectionID,
		rel = '' // folder rel path ('' for collection)
	} = $props();

	let name = $state('');
	let summary = $state('');
	let docs = $state('');
	let dirty = $state(false);
	let error = $state('');
	let preview = $state(false);
	let loadedID = $state('');

	$effect(() => {
		const k = `${kind}:${collectionID}:${rel}`;
		if (k !== loadedID) load();
	});

	async function load() {
		loadedID = `${kind}:${collectionID}:${rel}`;
		error = '';
		dirty = false;
		preview = false;
		try {
			if (kind === 'collection') {
				const d = (await RequestService.GetCollectionDoc(collectionID)) as CollectionDoc;
				name = d.info?.name ?? '';
				summary = d.info?.summary ?? '';
				docs = d.docs ?? '';
			} else {
				const d = (await RequestService.GetFolderDoc(collectionID + '/' + rel)) as FolderDoc;
				name = d.info?.name ?? '';
				docs = d.docs ?? '';
			}
		} catch (e) {
			error = String(e);
		}
	}

	async function save() {
		error = '';
		try {
			if (kind === 'collection') {
				const d = (await RequestService.GetCollectionDoc(collectionID)) as CollectionDoc;
				d.info.name = name;
				d.info.summary = summary;
				d.docs = docs;
				await RequestService.SaveCollectionDoc(collectionID, d as any);
			} else {
				const d = (await RequestService.GetFolderDoc(collectionID + '/' + rel)) as FolderDoc;
				d.info.name = name;
				d.docs = docs;
				await RequestService.SaveFolderDoc(collectionID + '/' + rel, d as any);
			}
			dirty = false;
		} catch (e) {
			error = String(e);
		}
	}

	function render() {
		return DOMPurify.sanitize(marked.parse(docs || '') as string);
	}
</script>

<div class="docsedit">
	<label class="lbl">Name</label>
	<input bind:value={name} oninput={() => (dirty = true)} />
	{#if kind === 'collection'}
		<label class="lbl">Summary</label>
		<input bind:value={summary} oninput={() => (dirty = true)} placeholder="One-line description" />
	{/if}
	<div class="dochead">
		<label class="lbl">Documentation (markdown)</label>
		<div class="spacer"></div>
		<button class:active={!preview} onclick={() => (preview = false)}>Edit</button>
		<button class:active={preview} onclick={() => (preview = true)}>Preview</button>
	</div>
	{#if preview}
		<div class="preview">{@html render()}</div>
	{:else}
		<textarea rows="12" bind:value={docs} oninput={() => (dirty = true)} placeholder={'# ' + (name || 'Title') + '\n\nOptional markdown documentation.'}></textarea>
	{/if}
	{#if error}<span class="err">{error}</span>{/if}
	<div class="foot">
		<button class="primary" onclick={save} disabled={!dirty}>{dirty ? 'Save' : 'Saved'}</button>
	</div>
</div>

<style>
	.docsedit {
		display: flex;
		flex-direction: column;
		gap: 6px;
		max-width: 760px;
	}
	.lbl {
		font: 600 11px/1 Inter, system-ui, sans-serif;
		text-transform: uppercase;
		letter-spacing: 0.6px;
		color: var(--muted);
		margin-top: 8px;
	}
	input {
		background: var(--surface-2);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-ctl);
		padding: 8px 10px;
		font: 13px Inter, system-ui, sans-serif;
	}
	textarea {
		background: var(--surface-2);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-ctl);
		padding: 10px;
		font: 12.5px/1.55 var(--mono);
		resize: vertical;
	}
	textarea:focus,
	input:focus {
		outline: 1px solid var(--accent);
	}
	.dochead {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-top: 8px;
	}
	.spacer {
		flex: 1;
	}
	.dochead button {
		border: none;
		background: none;
		color: var(--muted);
		font: 500 12px Inter, system-ui, sans-serif;
		padding: 3px 8px;
		border-radius: 6px;
		cursor: pointer;
	}
	.dochead button.active {
		color: var(--accent);
		background: var(--accent-soft);
	}
	.preview {
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		padding: 12px 14px;
		background: var(--surface-2);
		font: 13.5px/1.6 Inter, system-ui, sans-serif;
		min-height: 120px;
	}
	.preview :global(h1), .preview :global(h2) {
		font-size: 16px;
		margin: 0.4em 0;
	}
	.preview :global(code) {
		font-family: var(--mono);
		background: var(--accent-soft);
		padding: 1px 5px;
		border-radius: 4px;
		font-size: 12px;
	}
	.preview :global(pre code) {
		display: block;
		padding: 10px;
	}
	.err {
		color: var(--danger);
		font-size: 12px;
	}
	.foot {
		margin-top: 8px;
	}
	.primary {
		border: none;
		background: var(--accent);
		color: #fff;
		border-radius: var(--radius-ctl);
		padding: 8px 18px;
		font: 600 12.5px Inter, system-ui, sans-serif;
		cursor: pointer;
	}
	.primary:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.primary:hover:not(:disabled) {
		background: var(--accent-hover);
	}
</style>
