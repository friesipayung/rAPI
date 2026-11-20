<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { EnvService, RequestService, SyncService, TreeService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import { app, listenSyncStatus } from '#lib/state.svelte';
	import Tree from '#lib/components/Tree.svelte';
	import SyncPill from '#lib/components/SyncPill.svelte';
	import RequestEditor from '#lib/components/RequestEditor.svelte';
	import DocsEditor from '#lib/components/DocsEditor.svelte';
	import EnvPicker from '#lib/components/EnvPicker.svelte';
	import type { GitLogEntry, TreeNode } from '#lib/types';

	listenSyncStatus();

	let selected = $state<TreeNode | null>(null);
	let showLog = $state(false);
	let history = $state<GitLogEntry[]>([]);
	let error = $state('');
	let envName = $state('');

	// create form state
	let creating = $state<'' | 'collection' | 'folder' | 'request' | 'rename'>('');
	let newName = $state('');
	let busy = $state(false);

	// selection → ids
	const selCollectionID = $derived(selected ? selected.id.split('/')[0] : '');
	const selFolderRel = $derived.by(() => {
		if (!selected) return '';
		const parts = selected.id.split('/').slice(1);
		if (selected.type === 'folder') return parts.join('/');
		if (selected.type === 'http' && parts.length > 1) return parts.slice(0, -1).join('/');
		return '';
	});

	async function loadTree() {
		error = '';
		try {
			const cols = (await TreeService.ListCollections()) ?? [];
			app.collections = cols as TreeNode[];
		} catch (e) {
			error = String(e);
		}
	}

	// keep `selected` pointing at the refreshed tree node with the same id
	function reselect() {
		if (!selected) return;
		const find = (nodes: TreeNode[]): TreeNode | null => {
			for (const n of nodes) {
				if (n.id === selected!.id) return n;
				const hit = find(n.children ?? []);
				if (hit) return hit;
			}
			return null;
		};
		const hit = find(app.collections);
		selected = hit;
	}

	onMount(async () => {
		if (!app.vault?.valid) {
			goto('/', { replaceState: true });
			return;
		}
		await loadTree();
	});

	function startCreate(kind: 'collection' | 'folder' | 'request') {
		creating = kind;
		newName = '';
	}
	async function doCreate() {
		if (!newName.trim() || !creating) return;
		busy = true;
		error = '';
		try {
			if (creating === 'collection') {
				await TreeService.CreateCollection(newName.trim());
			} else if (creating === 'request') {
				await TreeService.CreateRequest(selCollectionID, selFolderRel, newName.trim());
			} else if (creating === 'folder') {
				await TreeService.CreateFolder(selCollectionID, selFolderRel, newName.trim());
			} else if (creating === 'rename' && selected) {
				await TreeService.RenameItem(selected.id, newName.trim());
				selected = null;
			}
			creating = '';
			await loadTree();
		} catch (e) {
			error = String(e);
		} finally {
			busy = false;
		}
	}

	async function deleteSelected() {
		if (!selected) return;
		const label = selected.type === 'collection' ? 'collection and ALL its contents' : selected.type === 'folder' ? 'folder and its requests' : 'request';
		if (!confirm(`Delete ${label} "${selected.name}"? This rewrites files in the vault.`)) return;
		error = '';
		try {
			await TreeService.DeleteItem(selected.id);
			selected = null;
			await loadTree();
		} catch (e) {
			error = String(e);
		}
	}

	function renameSelected() {
		if (!selected) return;
		newName = selected.name;
		creating = 'rename';
	}

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
				<span class="sidebtns">
					<button onclick={() => startCreate('request')} disabled={!selected} title="New request (inside selection)">+ Req</button>
					<button onclick={() => startCreate('folder')} disabled={!selected} title="New folder (inside selection)">+ Dir</button>
					<button onclick={() => startCreate('collection')} title="New collection">+ Col</button>
				</span>
			</div>
			{#if creating}
				<form class="createform" onsubmit={(e) => { e.preventDefault(); doCreate(); }}>
					<span class="clabel">{creating === 'rename' ? `Rename “${selected?.name}”` : `New ${creating}`}</span>
					<input bind:value={newName} placeholder="Name" autofocus />
					<div class="cbtns">
						<button type="submit" disabled={busy || !newName.trim()}>Create</button>
						<button type="button" class="ghost" onclick={() => (creating = '')}>Cancel</button>
					</div>
				</form>
			{/if}
			{#if app.collections.length === 0}
				<p class="empty">No collections yet. Click <strong>+ Col</strong> to create one.</p>
			{:else}
				<Tree nodes={app.collections} selected={selected?.id ?? ''} onselect={(n) => (selected = n)} />
			{/if}
		</aside>

		<main class="content">
			{#if !selected}
				<div class="empty-state">
					<p>Select a request from the tree.</p>
					<p class="muted">Or create a collection with <strong>+ Col</strong>.</p>
				</div>
			{:else if selected.type === 'http'}
				<div class="reqhead">
					<h2>{selected.name}</h2>
					<div class="spacer"></div>
					<EnvPicker collectionID={selCollectionID} />
				</div>
				<RequestEditor
					requestID={selected.id}
					onSaved={async () => { await loadTree(); reselect(); }}
				/>
			{:else}
				<div class="infohead">
					<h2>
						{#if selected.type === 'collection'}Collection{:else}Folder{/if}: {selected.name}
					</h2>
					<div class="spacer"></div>
					<button onclick={renameSelected}>Rename</button>
					<button class="danger" onclick={deleteSelected}>Delete</button>
				</div>
				<DocsEditor kind={selected.type} collectionID={selCollectionID} rel={selFolderRel} />
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
	.sidebtns {
		display: flex;
		gap: 4px;
		text-transform: none;
		letter-spacing: 0;
	}
	.sidebtns button {
		border: 1px solid var(--border);
		background: none;
		color: var(--muted);
		border-radius: var(--radius-ctl);
		padding: 3px 7px;
		font: 500 11px Inter, system-ui, sans-serif;
		cursor: pointer;
	}
	.sidebtns button:hover:not(:disabled) {
		color: var(--accent);
		border-color: var(--accent);
	}
	.sidebtns button:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.createform {
		display: flex;
		flex-direction: column;
		gap: 6px;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		padding: 10px;
		margin-bottom: 10px;
	}
	.clabel {
		font: 600 11px Inter, system-ui, sans-serif;
		text-transform: uppercase;
		color: var(--muted);
	}
	.createform input {
		background: var(--surface);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-ctl);
		padding: 7px 10px;
		font: 12.5px Inter, system-ui, sans-serif;
	}
	.cbtns {
		display: flex;
		gap: 6px;
	}
	.cbtns button {
		border: none;
		background: var(--accent);
		color: #fff;
		border-radius: var(--radius-ctl);
		padding: 6px 14px;
		font: 600 12px Inter, system-ui, sans-serif;
		cursor: pointer;
	}
	.cbtns button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.cbtns .ghost {
		background: none;
		color: var(--muted);
		border: 1px solid var(--border);
	}
	.empty {
		color: var(--muted);
		font-size: 12.5px;
		padding: 8px;
	}
	.content {
		flex: 1;
		padding: 20px 24px 0;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.reqhead,
	.infohead {
		display: flex;
		align-items: center;
		gap: 10px;
		margin-bottom: 10px;
	}
	h2 {
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 0;
		font-size: 17px;
	}
	.infohead button {
		border: 1px solid var(--border);
		background: var(--surface-2);
		color: var(--text);
		border-radius: var(--radius-ctl);
		padding: 6px 12px;
		font: 500 12px Inter, system-ui, sans-serif;
		cursor: pointer;
	}
	.infohead button.danger {
		color: var(--danger);
	}
	.content :global(.editor) {
		flex: 1;
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
