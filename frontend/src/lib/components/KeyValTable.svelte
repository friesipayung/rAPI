<script lang="ts">
	import type { KV } from '#lib/types';

	let { rows = $bindable() as KV[], keyPlaceholder = 'Name', valPlaceholder = 'Value' } = $props();

	function add() {
		rows.push({ name: '', value: '', disabled: false });
	}
</script>

<div class="kvtable">
	{#if rows.length === 0}
		<p class="hint">Empty — <button class="link" onclick={add}>add a row</button></p>
	{:else}
		{#each rows as row, i (i)}
			<div class="row" class:off={row.disabled}>
				<input class="check" type="checkbox" bind:checked={row.disabled} title="Disabled" />
				<input class="name" placeholder={keyPlaceholder} bind:value={row.name} />
				<input class="value" placeholder={valPlaceholder} bind:value={row.value} />
				<button class="del" onclick={() => rows.splice(i, 1)} title="Remove">×</button>
			</div>
		{/each}
	{/if}
	<button class="add" onclick={add}>+ Add</button>
</div>

<style>
	.kvtable {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.row {
		display: grid;
		grid-template-columns: 22px 1fr 1.4fr 26px;
		gap: 6px;
		align-items: center;
	}
	.row.off {
		opacity: 0.45;
	}
	input:not(.check) {
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		color: var(--text);
		padding: 7px 10px;
		font: 12.5px var(--mono);
	}
	input:focus {
		outline: 1px solid var(--accent);
	}
	.del {
		border: none;
		background: none;
		color: var(--muted);
		font-size: 15px;
		cursor: pointer;
	}
	.del:hover {
		color: var(--danger);
	}
	.add,
	.link {
		border: 1px dashed var(--border);
		background: none;
		color: var(--muted);
		border-radius: var(--radius-ctl);
		padding: 5px 12px;
		font: 500 12px Inter, system-ui, sans-serif;
		cursor: pointer;
		align-self: flex-start;
	}
	.add:hover {
		color: var(--accent);
		border-color: var(--accent);
	}
	.link {
		border: none;
		padding: 0;
		color: var(--accent);
		text-decoration: underline;
	}
	.hint {
		color: var(--muted);
		font-size: 12.5px;
		margin: 0;
	}
</style>
