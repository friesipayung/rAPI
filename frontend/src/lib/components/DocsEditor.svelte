<script lang="ts">
	import { RequestService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import DOMPurify from 'dompurify';
	import { marked } from 'marked';
	import Icon from './Icon.svelte';
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
	<label class="field-label" for="docname">Name</label>
	<input id="docname" bind:value={name} oninput={() => (dirty = true)} />
	{#if kind === 'collection'}
		<label class="field-label" for="docsummary">Summary</label>
		<input id="docsummary" bind:value={summary} oninput={() => (dirty = true)} placeholder="One-line description" />
	{/if}
	<div class="dochead">
		<span class="field-label"><Icon name="file-text" size={12} /> Documentation (markdown)</span>
		<div class="spacer"></div>
		<button class="chip" class:active={!preview} onclick={() => (preview = false)}>Edit</button>
		<button class="chip" class:active={preview} onclick={() => (preview = true)}>Preview</button>
	</div>
	{#if preview}
		<div class="preview">{@html render()}</div>
	{:else}
		<textarea rows="12" bind:value={docs} oninput={() => (dirty = true)} placeholder={'# ' + (name || 'Title') + '\n\nOptional markdown documentation.'}></textarea>
	{/if}
	{#if error}<span class="error-text">{error}</span>{/if}
	<div class="foot">
		<button class="btn primary" onclick={save} disabled={!dirty}>{dirty ? 'Save' : 'Saved'}</button>
	</div>
</div>

<style>
	.docsedit {
		display: flex;
		flex-direction: column;
		gap: 8px;
		max-width: 760px;
	}
	.dochead {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-top: 8px;
	}
	.dochead .field-label {
		display: inline-flex;
		align-items: center;
		gap: 5px;
	}
	.spacer {
		flex: 1;
	}
	.preview {
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		padding: 12px 14px;
		background: var(--surface-2);
		font: 13.5px/1.6 var(--font-ui);
		min-height: 120px;
	}
	.preview :global(h1), .preview :global(h2) {
		font-size: 16px;
		margin: 0.4em 0;
	}
	.preview :global(code) {
		font-family: var(--font-mono);
		background: var(--accent-soft);
		padding: 1px 5px;
		border-radius: 4px;
		font-size: 12px;
	}
	.preview :global(pre code) {
		display: block;
		padding: 10px;
	}
	.foot {
		margin-top: 8px;
	}
</style>
