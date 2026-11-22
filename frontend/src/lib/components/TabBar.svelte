<script lang="ts">
	import Icon from './Icon.svelte';

	interface TabItem {
		id: string;
		name: string;
		method?: string;
	}

	let {
		tabs = [],
		activeID = '',
		dirty,
		onselect,
		onclose
	}: {
		tabs?: TabItem[];
		activeID?: string;
		dirty?: Set<string>;
		onselect: (id: string) => void;
		onclose: (id: string) => void;
	} = $props();

	// truncate long names in the middle so head and tail both stay readable
	const NAME_MAX = 26;
	function shortName(name: string): string {
		if (name.length <= NAME_MAX) return name;
		const keep = NAME_MAX - 1;
		const head = Math.ceil(keep / 2);
		const tail = Math.floor(keep / 2);
		return name.slice(0, head) + '…' + (tail > 0 ? name.slice(-tail) : '');
	}

	// middle-click anywhere on the pill closes the tab
	function middleClose(e: MouseEvent, id: string) {
		if (e.button !== 1) return;
		e.preventDefault();
		onclose(id);
	}
</script>

{#if tabs.length}
	<div class="tabbar" role="tablist" aria-label="Open requests">
		{#each tabs as t (t.id)}
			<div
				class="tab"
				class:active={t.id === activeID}
				role="tab"
				tabindex="0"
				aria-selected={t.id === activeID}
				title={t.name}
				onclick={() => onselect(t.id)}
				onkeydown={(e) => e.key === 'Enter' && onselect(t.id)}
				onauxclick={(e) => middleClose(e, t.id)}
				onmousedown={(e) => e.button === 1 && e.preventDefault()}
			>
				{#if t.method}<span class="method m-{t.method}">{t.method}</span>{/if}
				<span class="tname">{shortName(t.name)}</span>
				{#if dirty?.has(t.id)}<span class="dirtydot" title="Unsaved changes"></span>{/if}
				<button
					class="tabclose"
					title="Close tab"
					onclick={(e) => {
						e.stopPropagation();
						onclose(t.id);
					}}
				>
					<Icon name="x" size={11} />
				</button>
			</div>
		{/each}
	</div>
{/if}

<style>
	.tabbar {
		display: flex;
		align-items: center;
		gap: 5px;
		flex: none;
		min-width: 0;
		overflow-x: auto; /* horizontal scroll is allowed on the tab bar */
		overflow-y: hidden;
		padding: 0 2px 10px;
		scrollbar-width: thin;
	}
	.tab {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex: none;
		max-width: 240px;
		padding: 3px 5px 3px 10px;
		border: 1px solid var(--border);
		border-radius: 999px;
		background: var(--surface-2);
		color: var(--muted);
		font-size: 12.5px;
		cursor: pointer;
		user-select: none;
		transition: color 0.12s, border-color 0.12s, background 0.12s;
	}
	.tab:hover {
		color: var(--text);
		border-color: var(--border-strong);
	}
	.tab.active {
		color: var(--text);
		background: var(--surface);
		border-color: var(--accent);
		box-shadow: var(--shadow-1);
	}
	.tname {
		min-width: 0;
		overflow: hidden;
		white-space: nowrap;
	}
	.dirtydot {
		flex: none;
		width: 7px;
		height: 7px;
		border-radius: 999px;
		background: var(--accent);
	}
	.tabclose {
		flex: none;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		padding: 0;
		border: none;
		background: none;
		color: var(--muted);
		border-radius: 999px;
		cursor: pointer;
	}
	.tabclose:hover {
		color: var(--text);
		background: var(--border);
	}
	/* same method chip colors as Tree.svelte */
	.method {
		flex: none;
		font: 600 10px/1 var(--font-mono);
		padding: 3px 5px;
		border-radius: 4px;
		min-width: 30px;
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
</style>
