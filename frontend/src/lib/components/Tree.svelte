<script lang="ts">
	import type { TreeNode } from '../types';
	import Tree from './Tree.svelte';
	import Icon from './Icon.svelte';

	let {
		nodes,
		selected = '',
		onselect,
	}: {
		nodes: TreeNode[];
		selected?: string;
		onselect: (node: TreeNode) => void;
	} = $props();
</script>

<ul>
	{#each nodes as n (n.id)}
		<li>
			<button class="row {n.type}" class:selected={selected === n.id} onclick={() => onselect(n)}>
				{#if n.type === 'http'}
					<span class="method m-{n.method}">{n.method}</span>
				{:else if n.type === 'folder'}
					<span class="chev"><Icon name="folder" size={13} /></span>
				{:else}
					<span class="chev col"><Icon name="box" size={13} /></span>
				{/if}
				<span class="name">{n.name}</span>
				{#if n.examples}
					<span class="badge" title="{n.examples} sample responses">{n.examples}</span>
				{/if}
				{#if n.docs}
					<span class="docs" title="has markdown docs">md</span>
				{/if}
			</button>
			{#if n.children?.length}
				<Tree nodes={n.children} {selected} {onselect} />
			{/if}
		</li>
	{/each}
</ul>

<style>
	ul {
		list-style: none;
		margin: 0;
		padding: 0 0 0 12px;
	}
	ul:first-child {
		padding-left: 0;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 6px 8px;
		border: none;
		background: none;
		color: var(--text);
		font: inherit;
		text-align: left;
		border-radius: var(--radius-ctl);
		cursor: pointer;
	}
	.row:hover {
		background: var(--surface-2);
	}
	.row.selected {
		background: var(--accent-soft);
		color: var(--accent);
		font-weight: 600;
	}
	.chev {
		color: var(--muted);
		width: 14px;
		flex: none;
		display: inline-flex;
		justify-content: center;
	}
	.chev.col {
		color: var(--accent);
	}
	.name {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.method {
		flex: none;
		font: 600 10px/1 var(--font-mono);
		padding: 3px 5px;
		border-radius: 4px;
		min-width: 38px;
		text-align: center;
	}
	.m-GET {
		color: var(--success);
		background: rgba(26, 127, 55, 0.12);
	}
	.m-POST {
		color: var(--warning);
		background: rgba(154, 103, 0, 0.12);
	}
	.m-PUT {
		color: #0b5bb5;
		background: rgba(11, 91, 181, 0.1);
	}
	.m-DELETE {
		color: var(--danger);
		background: rgba(209, 36, 47, 0.1);
	}
	.m-PATCH {
		color: #6929c4;
		background: rgba(105, 41, 196, 0.1);
	}
	.badge {
		flex: none;
		font: 600 10px/1 var(--font-mono);
		color: var(--accent);
		background: var(--accent-soft);
		border-radius: 999px;
		padding: 3px 6px;
	}
	.docs {
		flex: none;
		font: 600 9px/1 var(--font-mono);
		color: var(--muted);
		border: 1px solid var(--border);
		border-radius: 4px;
		padding: 2px 4px;
	}
</style>
