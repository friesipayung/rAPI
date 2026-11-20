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
	import Icon from '#lib/components/Icon.svelte';
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

	// focus the create/rename input when the form appears
	function focusNow(node: HTMLElement) {
		node.focus();
	}

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
	<header class="topbar drag-region">
		<span class="brand no-drag"><Icon name="box" size={17} strokeWidth={2} /> <span class="brandtext"><span class="r">r</span>API</span></span>
		<span class="vault no-drag" title={app.vault?.path}>{app.vault?.name}</span>
		<div class="spacer"></div>
		{#if error}<span class="apperror no-drag" title={error}><Icon name="alert" size={13} /> {error.slice(0, 60)}</span>{/if}
		<div class="syncbtns no-drag">
			<button class="btn" onclick={commitNow} title="Commit all changes now"><Icon name="commit" /> Commit</button>
			<button class="btn" onclick={pushNow} disabled={!app.vault?.hasRemote} title={app.vault?.hasRemote ? 'Push to origin' : 'No remote configured'}><Icon name="upload" /> Push</button>
			<button class="btn" onclick={pullNow} disabled={!app.vault?.hasRemote} title="Pull --rebase from origin"><Icon name="download" /> Pull</button>
		</div>
		<button class="pillbtn no-drag" onclick={toggleLog} title="Sync details & git log">
			<SyncPill sync={app.sync} />
		</button>
	</header>

	<div class="body">
		<aside class="sidebar">
			<div class="sidehead">
				<span>Collections</span>
				<span class="sidebtns">
					<button class="btn" onclick={() => startCreate('request')} disabled={!selected} title="New request (inside selection)"><Icon name="plus" size={12} /> Req</button>
					<button class="btn" onclick={() => startCreate('folder')} disabled={!selected} title="New folder (inside selection)"><Icon name="folder" size={12} /> Dir</button>
					<button class="btn" onclick={() => startCreate('collection')} title="New collection"><Icon name="box" size={12} /> Col</button>
				</span>
			</div>
			{#if creating}
				<form class="createform" onsubmit={(e) => { e.preventDefault(); doCreate(); }}>
					<span class="field-label">{creating === 'rename' ? `Rename “${selected?.name}”` : `New ${creating}`}</span>
					<input bind:value={newName} placeholder="Name" use:focusNow />
					<div class="cbtns">
						<button type="submit" class="btn primary" disabled={busy || !newName.trim()}>Create</button>
						<button type="button" class="btn ghost" onclick={() => (creating = '')}>Cancel</button>
					</div>
				</form>
			{/if}
			{#if app.collections.length === 0}
				<p class="empty">No collections yet. Click <strong>Col</strong> to create one.</p>
			{:else}
				<Tree nodes={app.collections} selected={selected?.id ?? ''} onselect={(n) => (selected = n)} />
			{/if}
		</aside>

		<main class="content">
			{#if !selected}
				<div class="empty-state">
					<Icon name="box" size={36} strokeWidth={1.2} />
					<p>Select a request from the tree.</p>
					<p class="muted">Or create a collection with <strong>Col</strong>.</p>
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
					<button class="btn" onclick={renameSelected}><Icon name="pencil" size={13} /> Rename</button>
					<button class="btn danger" onclick={deleteSelected}><Icon name="trash" size={13} /> Delete</button>
				</div>
				<DocsEditor kind={selected.type} collectionID={selCollectionID} rel={selFolderRel} />
			{/if}
		</main>
	</div>

	{#if showLog}
		<div class="drawer">
			<div class="drawerhead">
				<span>Git log (last {history.length})</span>
				<button class="iconbtn" onclick={toggleLog} title="Close"><Icon name="x" size={14} /></button>
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
	/* clear the macOS traffic lights (hidden-inset titlebar); other platforms
	   get a native titlebar, so no extra inset is needed */
	.topbar {
		display: flex;
		align-items: center;
		gap: 12px;
		height: 50px;
		padding: 0 16px 0 var(--titlebar-inset, 0px);
		background: var(--surface);
		border-bottom: 1px solid var(--border);
		flex: none;
	}
	:global(html[data-platform='mac']) .topbar {
		--titlebar-inset: 84px;
	}
	.brand {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		color: var(--accent);
	}
	.brandtext {
		font-weight: 750;
		font-size: 15.5px;
		letter-spacing: 0.4px;
		color: var(--text);
	}
	.brandtext .r {
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
		display: inline-flex;
		align-items: center;
		gap: 5px;
		color: var(--danger);
		font-size: 12px;
		max-width: 300px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.syncbtns {
		display: flex;
		gap: 6px;
	}
	.pillbtn {
		border: none;
		background: none;
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
		font: 600 11px/1 var(--font-ui);
		text-transform: uppercase;
		letter-spacing: 0.8px;
		color: var(--muted);
		padding: 4px 4px 10px;
	}
	.sidebtns {
		display: flex;
		gap: 4px;
	}
	.sidebtns .btn {
		height: 24px;
		padding: 0 8px;
		font-size: 11px;
		background: none;
		border-color: var(--border);
		color: var(--muted);
	}
	.sidebtns .btn:hover:not(:disabled) {
		color: var(--accent);
		border-color: var(--accent);
		background: var(--accent-soft);
	}
	.createform {
		display: flex;
		flex-direction: column;
		gap: 8px;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		padding: 10px;
		margin-bottom: 10px;
	}
	.cbtns {
		display: flex;
		gap: 6px;
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
	.content :global(.editor) {
		flex: 1;
	}
	.empty-state {
		height: 100%;
		display: grid;
		place-content: center;
		justify-items: center;
		gap: 6px;
		text-align: center;
		color: var(--text);
	}
	.empty-state :global(svg) {
		color: var(--border-strong);
		margin-bottom: 6px;
	}
	.muted {
		color: var(--muted);
		font-size: 13px;
	}
	.drawer {
		position: fixed;
		right: 16px;
		top: 58px;
		width: min(460px, calc(100vw - 32px));
		max-height: 60vh;
		overflow: auto;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow-2);
		padding: 12px;
		z-index: var(--z-drawer);
	}
	.drawerhead {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font-weight: 600;
		font-size: 13px;
		margin-bottom: 8px;
	}
	.logentry {
		border-top: 1px solid var(--border);
		padding: 8px 0;
	}
	.logentry .op {
		font: 600 12px var(--font-mono);
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
		font: 11px/1.5 var(--font-mono);
		color: var(--muted);
		margin: 4px 0 0;
	}
</style>
