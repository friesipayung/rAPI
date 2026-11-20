<script lang="ts">
	import Icon from './Icon.svelte';
	import type { KV } from '#lib/types';

	let { rows = $bindable() as KV[], keyPlaceholder = 'Name', valPlaceholder = 'Value' } = $props();

	function add() {
		rows.push({ name: '', value: '', disabled: false });
	}
</script>

<div class="kvtable">
	{#if rows.length === 0}
		<p class="hint-text">Empty — <button class="link" onclick={add}>add a row</button></p>
	{:else}
		{#each rows as row, i (i)}
			<div class="row" class:off={row.disabled}>
				<input class="check" type="checkbox" bind:checked={row.disabled} title="Disabled" />
				<input class="name" placeholder={keyPlaceholder} bind:value={row.name} />
				<input class="value" placeholder={valPlaceholder} bind:value={row.value} />
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
	.row.off {
		opacity: 0.45;
	}
	.row .name,
	.row .value {
		font-family: var(--font-mono);
		font-size: 12.5px;
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
