<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { SyncService, TreeService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import { app, listenSyncStatus } from '$lib/state.svelte';
	import Tree from '$lib/components/Tree.svelte';
	import SyncPill from '$lib/components/SyncPill.svelte';
	import type { GitLogEntry, TreeNode } from '$lib/types';

	listenSyncStatus();

	let selected = $state<TreeNode | null>(null);
	let showLog = $state(false);
	let history = $state<GitLogEntry[]>([]);
	let error = $state('');

	async function loadTree() {
		error = '';
		try {
			const cols = (await TreeService.ListCollections()) ?? [];
			app.collections = cols as TreeNode[];
		} catch (e) {
			error = String(e);
		}
	}

	onMount(async () => {
		if (!app.vault?.valid) {
			goto('/', { replaceState: true });
			return;
		}
		await loadTree();
	});

	async function commitNow() {
		error = '';
		try {
			await SyncService.CommitNow('');
			await loadTree();
		} catch (e) {
			error = String(e);
		}
	}
	async function pushNow() {
		error = '';
		try {
			await SyncService.PushNow();
		} catch (e) {
			error = String(e);
		}
	}
	async function pullNow() {
		error = '';
		try {
			await SyncService.PullNow();
			await loadTree();
		} catch (e) {
			error = String(e);
		}
	}
	async function toggleLog() {
		showLog = !showLog;
		if (showLog) {
			history = (await SyncService.History()) ?? [];
		}
	}
</script>

<div class="shell">
	<header class="topbar">
		<span class="brand"><span class="r">r</span>API</span>
		<span class="vault" title={app.vault?.path}>{app.vault?.name}</span>
		<div class="spacer"></div>
		{#if error}<span class="apperror" title={error}>⚠ {error.slice(0, 60)}</span>{/if}
		<div class="syncbtns">
			<button onclick={commitNow} title="Commit all changes now">Commit</button>
			<button onclick={pushNow} disabled={!app.vault?.hasRemote} title={app.vault?.hasRemote ? 'Push to origin' : 'No remote configured'}>Push</button>
			<button onclick={pullNow} disabled={!app.vault?.hasRemote} title="Pull --rebase from origin">Pull</button>
		</div>
		<button class="pillbtn" onclick={toggleLog}>
			<SyncPill sync={app.sync} />
		</button>
	</header>

	<div class="body">
		<aside class="sidebar">
			<div class="sidehead">
				<span>Collections</span>
				<button onclick={loadTree} title="Refresh tree">↻</button>
			</div>
			{#if app.collections.length === 0}
				<p class="empty">No collections yet. Add files to <code>{app.vault?.path}</code> — each folder with an <code>opencollection.yml</code> is a collection.</p>
			{:else}
				<Tree nodes={app.collections} selected={selected?.id ?? ''} onselect={(n) => (selected = n)} />
			{/if}
		</aside>

		<main class="content">
			{#if selected}
				<h2>
					{#if selected.type === 'http'}<span class="method">{selected.method}</span>{/if}
					{selected.name}
				</h2>
				{#if selected.url}<code class="url">{selected.url}</code>{/if}
				{#if selected.envs?.length}
					<div class="envs">
						{#each selected.envs as env}<span class="env">{env}</span>{/each}
					</div>
				{/if}
				{#if selected.examples}
					<p class="meta">{selected.examples} sample response{selected.examples > 1 ? 's' : ''} · {selected.docs ? 'has docs' : 'no docs'}</p>
				{/if}
				<p class="placeholder">Request editor &amp; execution arrive in phase 2.</p>
			{:else}
				<div class="empty-state">
					<p>Select a request from the tree.</p>
					<p class="muted">Editing, environments and execution come with phase 2.</p>
				</div>
			{/if}
		</main>
	</div>

	{#if showLog}
		<div class="drawer">
			<div class="drawerhead">
				<span>Git log (last {history.length})</span>
				<button onclick={toggleLog}>×</button>
			</div>
			{#if history.length === 0}
				<p class="empty">No failed git operations recorded.</p>
			{:else}
				{#each history as h}
					<div class="logentry">
						<span class="op">{h.op}</span>
						<span class="time">{new Date(h.time).toLocaleTimeString()}</span>
						<pre>{h.stderr || h.err || 'ok'}</pre>
					</div>
				{/each}
			{/if}
		</div>
	{/if}
</div>

<style>
	.shell {
		display: flex;
		flex-direction: column;
		height: 100vh;
	}
	.topbar {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 16px;
		background: var(--surface);
		border-bottom: 1px solid var(--border);
	}
	.brand {
		font-weight: 800;
		font-size: 16px;
		letter-spacing: 0.5px;
	}
	.brand .r {
		color: var(--accent);
	}
	.vault {
		color: var(--muted);
		font-size: 12.5px;
		max-width: 220px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.spacer {
		flex: 1;
	}
	.apperror {
		color: var(--danger);
		font-size: 12px;
		max-width: 300px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.syncbtns button,
	.pillbtn {
		border: 1px solid var(--border);
		background: var(--surface-2);
		color: var(--text);
		border-radius: var(--radius-ctl);
		padding: 6px 12px;
		font: 500 12.5px Inter, system-ui, sans-serif;
		cursor: pointer;
	}
	.syncbtns button:hover,
	.pillbtn:hover {
		background: var(--accent-soft);
	}
	.syncbtns button:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.pillbtn {
		padding: 2px 4px;
	}
	.body {
		display: flex;
		flex: 1;
		min-height: 0;
	}
	.sidebar {
		width: 300px;
		flex: none;
		border-right: 1px solid var(--border);
		background: var(--surface);
		overflow: auto;
		padding: 12px;
	}
	.sidehead {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font: 600 11px/1 Inter, system-ui, sans-serif;
		text-transform: uppercase;
		letter-spacing: 0.8px;
		color: var(--muted);
		padding: 4px 8px 10px;
	}
	.sidehead button {
		border: none;
		background: none;
		color: var(--muted);
		cursor: pointer;
		font-size: 14px;
	}
	.empty {
		color: var(--muted);
		font-size: 12.5px;
		padding: 8px;
	}
	.content {
		flex: 1;
		padding: 24px;
		overflow: auto;
	}
	h2 {
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 0 0 8px;
	}
	.method {
		font: 700 12px/1 var(--mono);
		color: var(--accent);
		background: var(--accent-soft);
		padding: 5px 8px;
		border-radius: 6px;
	}
	.url {
		display: block;
		font: 12.5px var(--mono);
		color: var(--muted);
		background: var(--surface-2);
		padding: 8px 10px;
		border-radius: var(--radius-ctl);
		margin: 0 0 12px;
		word-break: break-all;
	}
	.envs {
		display: flex;
		gap: 6px;
		margin-bottom: 12px;
	}
	.env {
		font: 600 11px/1 Inter, system-ui, sans-serif;
		color: var(--muted);
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 4px 10px;
	}
	.meta {
		color: var(--muted);
		font-size: 12.5px;
	}
	.placeholder {
		margin-top: 24px;
		padding: 16px;
		border: 1px dashed var(--border);
		border-radius: var(--radius-card);
		color: var(--muted);
	}
	.empty-state {
		height: 100%;
		display: grid;
		place-content: center;
		text-align: center;
		color: var(--text);
	}
	.empty-state .muted,
	.muted {
		color: var(--muted);
		font-size: 13px;
	}
	.drawer {
		position: fixed;
		right: 16px;
		top: 56px;
		width: min(460px, calc(100vw - 32px));
		max-height: 60vh;
		overflow: auto;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow);
		padding: 12px;
		z-index: 10;
	}
	.drawerhead {
		display: flex;
		justify-content: space-between;
		font-weight: 600;
		font-size: 13px;
		margin-bottom: 8px;
	}
	.drawerhead button {
		border: none;
		background: none;
		font-size: 16px;
		cursor: pointer;
		color: var(--muted);
	}
	.logentry {
		border-top: 1px solid var(--border);
		padding: 8px 0;
	}
	.logentry .op {
		font: 600 12px var(--mono);
		color: var(--accent);
	}
	.logentry .time {
		float: right;
		font-size: 11px;
		color: var(--muted);
	}
	.logentry pre {
		white-space: pre-wrap;
		word-break: break-word;
		font: 11px var(--mono);
		color: var(--muted);
		margin: 4px 0 0;
	}
</style>
