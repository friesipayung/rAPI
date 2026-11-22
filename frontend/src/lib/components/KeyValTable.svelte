<script lang="ts">
	import Icon from './Icon.svelte';
	import { DialogService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import type { KV } from '#lib/types';

	let {
		rows = $bindable() as KV[],
		keyPlaceholder = 'Name',
		valPlaceholder = 'Value',
		allowFiles = false
	}: {
		rows: KV[];
		keyPlaceholder?: string;
		valPlaceholder?: string;
		// Only multipart form tables offer the text|file toggle (file rows
		// carry a filesystem path in value); params/headers keep plain text.
		allowFiles?: boolean;
	} = $props();

	function add() {
		rows.push({ name: '', value: '', disabled: false });
	}

	// basename of a path (forward and back slashes) for the read-only display.
	function basename(p: string): string {
		if (!p) return '';
		const parts = p.split(/[\\/]/).filter(Boolean);
		return parts.length ? parts[parts.length - 1] : p;
	}

	function toggleType(row: KV) {
		if (row.type === 'file') {
			row.type = ''; // empty type = text (backward compatible)
			row.fileName = undefined;
		} else {
			row.type = 'file';
		}
	}

	async function browse(row: KV) {
		const path = (await DialogService.PickFile()) as unknown as string;
		if (!path) return; // user cancelled
		row.value = path;
		row.fileName = basename(path);
	}
</script>

<div class="kvtable" class:files={allowFiles}>
	{#if rows.length === 0}
		<p class="hint-text">Empty — <button class="link" onclick={add}>add a row</button></p>
	{:else}
		{#each rows as row, i (i)}
			<div class="row" class:off={row.disabled}>
				<input class="check" type="checkbox" bind:checked={row.disabled} title="Disabled" />
				<input class="name" placeholder={keyPlaceholder} bind:value={row.name} />
				{#if allowFiles}
					<div class="val">
						{#if row.type === 'file'}
							<span class="filepath" title={row.value}>{row.fileName || basename(row.value ?? '') || '—'}</span>
							<button class="chip browse" onclick={() => browse(row)} title="Choose file">Browse…</button>
						{:else}
							<input class="value" placeholder={valPlaceholder} bind:value={row.value} />
						{/if}
					</div>
					<button
						class="chip typechip"
						class:active={row.type === 'file'}
						onclick={() => toggleType(row)}
						title="Toggle text / file"
					>{row.type === 'file' ? 'file' : 'text'}</button>
				{:else}
					<input class="value" placeholder={valPlaceholder} bind:value={row.value} />
				{/if}
				<button class="iconbtn del" onclick={() => rows.splice(i, 1)} title="Remove"><Icon name="x" size={13} /></button>
			</div>
		{/each}
	{/if}
	<button class="btn dashed" onclick={add}><Icon name="plus" size={12} /> Add</button>
</div>

<style>
	.kvtable {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.row {
		display: grid;
		grid-template-columns: 22px 1fr 1.4fr 30px;
		gap: 6px;
		align-items: center;
	}
	/* file-enabled tables get one extra cell: the type chip (browse lives
	   inside the value cell) */
	.kvtable.files .row {
		grid-template-columns: 22px 1fr 1.4fr auto 30px;
	}
	.row.off {
		opacity: 0.45;
	}
	.row .name,
	.row .value {
		font-family: var(--font-mono);
		font-size: 12.5px;
	}
	.row .val {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}
	.row .val .value {
		width: 100%;
		min-width: 0;
	}
	.row .filepath {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-family: var(--font-mono);
		font-size: 12.5px;
		color: var(--muted);
	}
	.row .browse,
	.row .typechip {
		height: 22px;
		padding: 0 8px;
		font-size: 11px;
		flex: none;
	}
	.row .del {
		color: var(--muted);
	}
	.row .del:hover:not(:disabled) {
		color: var(--danger);
		background: var(--danger-soft);
	}
	.link {
		border: none;
		padding: 0;
		background: none;
		color: var(--accent);
		text-decoration: underline;
		font: inherit;
	}
</style>
