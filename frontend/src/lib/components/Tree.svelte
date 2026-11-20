<script lang="ts">
	import type { TreeNode } from '../types';
	import Tree from './Tree.svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import Icon from './Icon.svelte';

	let {
		nodes,
		selected = '',
		onselect,
		collapsed,
		ontoggle,
		depth = 0,
	}: {
		nodes: TreeNode[];
		selected?: string;
		onselect: (node: TreeNode) => void;
		collapsed: SvelteSet<string>;
		ontoggle: (id: string) => void;
		depth?: number;
	} = $props();

	// recursive HTTP-request count for a collection section header
	function countRequests(n: TreeNode): number {
		if (n.type === 'http') return 1;
		return (n.children ?? []).reduce((s, c) => s + countRequests(c), 0);
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
{#if depth === 0}
	<!-- top level: one visual section per collection, hairline separated -->
	{#each nodes as n, i (n.id)}
		<section class="csection" class:notfirst={i > 0}>
			<div
				class="row coll"
				class:selected={selected === n.id}
				role="button"
				tabindex="0"
				onclick={() => onselect(n)}
				onkeydown={(e) => e.key === 'Enter' && onselect(n)}
			>
				<button
					class="chevbtn"
					class:open={!collapsed.has(n.id)}
					title={collapsed.has(n.id) ? 'Expand' : 'Collapse'}
					onclick={(e) => { e.stopPropagation(); ontoggle(n.id); }}
				>
					<Icon name="chevron-right" size={13} strokeWidth={2} />
				</button>
				<span class="colicon"><Icon name="box" size={14} strokeWidth={1.8} /></span>
				<span class="name">{n.name}</span>
				<span class="count">{countRequests(n)}</span>
			</div>
			{#if !collapsed.has(n.id) && n.children?.length}
				<div class="children">
					<Tree nodes={n.children} {selected} {onselect} {collapsed} {ontoggle} depth={1} />
				</div>
			{/if}
		</section>
	{/each}
{:else}
	<ul>
		{#each nodes as n (n.id)}
			<li>
				{#if n.children?.length}
					<div
						class="row folder"
						class:selected={selected === n.id}
						role="button"
						tabindex="0"
						onclick={() => onselect(n)}
						onkeydown={(e) => e.key === 'Enter' && onselect(n)}
					>
						<button
							class="chevbtn"
							class:open={!collapsed.has(n.id)}
							onclick={(e) => { e.stopPropagation(); ontoggle(n.id); }}
						>
							<Icon name="chevron-right" size={12} strokeWidth={2} />
						</button>
						<span class="ficon"><Icon name="folder" size={13} /></span>
						<span class="name">{n.name}</span>
					</div>
					{#if !collapsed.has(n.id)}
						<Tree nodes={n.children} {selected} {onselect} {collapsed} {ontoggle} depth={depth + 1} />
					{/if}
				{:else}
					<button class="row req" class:selected={selected === n.id} onclick={() => onselect(n)}>
						<span class="chevspace"></span>
						{#if n.type === 'http'}
							<span class="method m-{n.method}">{n.method}</span>
						{:else}
							<span class="ficon"><Icon name="folder" size={13} /></span>
						{/if}
						<span class="name">{n.name}</span>
						{#if n.examples}
							<span class="badge" title="{n.examples} sample responses">{n.examples}</span>
						{/if}
						{#if n.docs}
							<span class="docs" title="has markdown docs">md</span>
						{/if}
					</button>
				{/if}
			</li>
		{/each}
	</ul>
{/if}

<style>
	/* ---- collection sections (top level) ---- */
	.csection {
		min-width: 0;
	}
	.csection.notfirst {
		border-top: 1px solid var(--border);
		margin-top: 6px;
		padding-top: 6px;
	}

	.row {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		min-width: 0;
		padding: 5px 6px;
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

	.coll {
		padding: 6px 6px;
	}
	.colicon {
		flex: none;
		display: inline-flex;
		color: var(--accent);
	}
	.count {
		flex: none;
		font: 600 10px/1 var(--font-mono);
		color: var(--muted);
		background: var(--surface-2);
		border-radius: 999px;
		padding: 3px 6px;
	}
	.row.selected .count {
		background: rgba(155, 17, 30, 0.08);
	}

	.chevbtn {
		flex: none;
		width: 18px;
		height: 18px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0;
		border: none;
		background: none;
		color: var(--muted);
		border-radius: 4px;
		cursor: pointer;
	}
	.chevbtn:hover {
		color: var(--text);
		background: var(--border);
	}
	.chevbtn :global(svg) {
		transition: transform 120ms ease;
	}
	.chevbtn.open :global(svg) {
		transform: rotate(90deg);
	}

	.ficon {
		flex: none;
		display: inline-flex;
		color: var(--muted);
	}
	.chevspace {
		flex: none;
		width: 18px;
	}

	/* ---- nested tree ---- */
	ul {
		list-style: none;
		margin: 0;
		padding: 0 0 0 12px;
	}
	li {
		min-width: 0;
	}
	:global(.children) > :global(ul) {
		margin-left: 8px;
		padding-left: 10px;
		border-left: 1px solid var(--border);
	}

	.name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.req {
		padding-left: 2px;
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
