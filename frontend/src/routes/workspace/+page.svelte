<script lang="ts">
	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { goto } from '$app/navigation';
	import { EnvService, ImportService, RequestService, SyncService, TreeService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import { app, listenSyncStatus } from '#lib/state.svelte';
	import Tree from '#lib/components/Tree.svelte';
	import SyncPill from '#lib/components/SyncPill.svelte';
	import RequestEditor from '#lib/components/RequestEditor.svelte';
	import DocsEditor from '#lib/components/DocsEditor.svelte';
	import EnvPicker from '#lib/components/EnvPicker.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import type { GitLogEntry, ImportReport, ImportResult, TreeNode } from '#lib/types';

	listenSyncStatus();

	let selected = $state<TreeNode | null>(null);
	let showLog = $state(false);
	let history = $state<GitLogEntry[]>([]);
	let error = $state('');
	let envName = $state('');

	// import (phase 3)
	let importing = $state(false);
	let importResult = $state<ImportResult | null>(null);
	let importReport = $state<ImportReport | null>(null);

	// create form state
	let creating = $state<'' | 'collection' | 'folder' | 'request' | 'rename'>('');
	let newName = $state('');
	let busy = $state(false);

	// sidebar: collapsible sections + resizable width (persisted)
	const collapsed = new SvelteSet<string>();
	function toggleCollapsed(id: string) {
		if (collapsed.has(id)) collapsed.delete(id);
		else collapsed.add(id);
	}
	const SIDEBAR_MIN = 220;
	const SIDEBAR_MAX = 480;
	let sidebarW = $state(Number(localStorage.getItem('rapi.sidebar.width')) || 300);
	let resizing = $state(false);
	function startResize(e: PointerEvent) {
		e.preventDefault();
		resizing = true;
		const move = (ev: PointerEvent) => {
			sidebarW = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, ev.clientX));
		};
		const up = () => {
			resizing = false;
			localStorage.setItem('rapi.sidebar.width', String(sidebarW));
			window.removeEventListener('pointermove', move);
			window.removeEventListener('pointerup', up);
		};
		window.addEventListener('pointermove', move);
		window.addEventListener('pointerup', up);
	}

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

	async function doImport() {
		importing = true;
		error = '';
		importResult = null;
		importReport = null;
		try {
			const res = (await ImportService.PickAndImport()) as unknown as ImportResult | null;
			if (!res) return; // dialog cancelled
			importResult = res;
			importReport = (await ImportService.Validate(res.collectionId)) as unknown as ImportReport;
			await loadTree();
		} catch (e) {
			error = String(e);
		} finally {
			importing = false;
		}
	}

	// paste a curl/wget command as a new request
	let showPaste = $state(false);
	let pasteCmd = $state('');
	let pasteName = $state('');
	let pasteErr = $state('');
	let pasteBusy = $state(false);
	function startPaste() {
		pasteCmd = '';
		pasteName = '';
		pasteErr = '';
		showPaste = true;
	}
	async function doPaste() {
		if (!pasteCmd.trim() || pasteBusy) return;
		if (!selCollectionID) {
			pasteErr = 'Select or create a collection first.';
			return;
		}
		pasteBusy = true;
		pasteErr = '';
		try {
			const id = (await TreeService.CreateFromCommand(selCollectionID, selFolderRel, pasteName.trim(), pasteCmd)) as unknown as string;
			showPaste = false;
			await loadTree();
			// select the freshly created request
			const parts = String(id).split('/');
			const rel = parts.slice(1).join('/');
			for (const n of flatten(app.collections)) {
				if (n.id === String(id) || n.id.endsWith('/' + rel)) {
					selected = n;
					break;
				}
			}
		} catch (e) {
			pasteErr = String(e);
		} finally {
			pasteBusy = false;
		}
	}
	function flatten(nodes: TreeNode[]): TreeNode[] {
		const out: TreeNode[] = [];
		for (const n of nodes) {
			out.push(n);
			if (n.children?.length) out.push(...flatten(n.children));
		}
		return out;
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
		<aside class="sidebar" style:width="{sidebarW}px" class:resizing>
			<div class="sidehead">
				<span>Collections</span>
				<span class="sidebtns">
					<button class="iconbtn" onclick={() => startCreate('request')} disabled={!selected} title="New request (inside selection)"><Icon name="plus" size={13} /></button>
					<button class="iconbtn" onclick={() => startCreate('folder')} disabled={!selected} title="New folder (inside selection)"><Icon name="folder" size={13} /></button>
					<button class="iconbtn" onclick={() => startCreate('collection')} title="New collection"><Icon name="box" size={13} /></button>
					<button class="iconbtn" onclick={startPaste} disabled={!app.collections.length} title="Paste a curl / wget command as a new request"><Icon name="terminal" size={13} /></button>
					<button class="iconbtn" onclick={doImport} disabled={importing} title="Import Postman v2.1, Bruno folder or Insomnia v4 export"><Icon name="upload" size={13} /></button>
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
				<p class="empty">No collections yet. Use <strong>+</strong> above to create one, or the <strong>upload</strong> button to import a Postman/Bruno/Insomnia export.</p>
			{:else}
				<Tree nodes={app.collections} selected={selected?.id ?? ''} onselect={(n) => (selected = n)} {collapsed} ontoggle={toggleCollapsed} />
			{/if}
			{#if importResult}
				<div class="importnotice">
					<span>Imported <strong>{importResult.collections} collection(s)</strong> — {importResult.requests} request(s), {importResult.folders} folder(s), {importResult.environments} env(s)</span>
					{#if importReport?.problems?.length}
						<ul>
							{#each importReport.problems as p}
								<li class:err={p.severity === 'error'}>{p.severity === 'error' ? '✕' : '!'} {p.message}{p.path ? ` (${p.path})` : ''}</li>
							{/each}
						</ul>
					{/if}
					<button class="btn ghost sm" onclick={() => (importResult = null)}>Dismiss</button>
				</div>
			{/if}
			<div class="sizeresize" role="separator" aria-orientation="vertical" onpointerdown={startResize} title="Drag to resize"></div>
		</aside>

		<main class="content">
			{#if !selected}
				<div class="empty-state">
					<Icon name="box" size={36} strokeWidth={1.2} />
					<p>Select a request from the tree.</p>
			<p class="muted">Or create a collection with the <strong>+</strong> button in the sidebar.</p>
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

	{#if showPaste}
		<div class="modalback" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) showPaste = false; }}>
			<div class="modal">
				<div class="drawerhead">
					<span>Paste curl / wget</span>
					<button class="iconbtn" onclick={() => (showPaste = false)} title="Close"><Icon name="x" size={14} /></button>
				</div>
				<p class="hint-text">Paste a complete <code>curl</code> or <code>wget</code> command. Method, URL, headers, auth, body and form fields are converted automatically into a new request.</p>
				<textarea
					class="pasteta"
					bind:value={pasteCmd}
					placeholder={'curl -X POST https://api.example.com/users \\\n  -H "Content-Type: application/json" \\\n  -d \'{"name":"budi"}\''}
					spellcheck="false"
				></textarea>
				<label class="pastename">Request name <span class="muted">(empty = derived from the URL)</span>
					<input bind:value={pasteName} placeholder="auto" />
				</label>
				{#if pasteErr}<p class="apperror plain">{pasteErr}</p>{/if}
				<div class="cbtns">
					<button class="btn primary" onclick={doPaste} disabled={pasteBusy || !pasteCmd.trim()}>
						<Icon name="clipboard" size={13} /> {pasteBusy ? 'Creating…' : 'Create request'}
					</button>
					<button class="btn ghost" onclick={() => (showPaste = false)}>Cancel</button>
				</div>
			</div>
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
		position: relative;
		border-right: 1px solid var(--border);
		background: var(--surface);
		overflow-y: auto;
		overflow-x: hidden;
		padding: 12px;
	}
	.sidebar.resizing {
		cursor: col-resize;
		user-select: none;
	}
	.sizeresize {
		position: absolute;
		top: 0;
		right: 0;
		width: 5px;
		height: 100%;
		cursor: col-resize;
		z-index: 5;
		border-radius: 0 3px 3px 0;
	}
	.sizeresize:hover {
		background: var(--accent-soft);
	}
	.sidehead .iconbtn {
		width: 26px;
		height: 24px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0;
		background: none;
		border: 1px solid transparent;
		border-radius: var(--radius-ctl);
		color: var(--muted);
		cursor: pointer;
	}
	.sidehead .iconbtn:hover:not(:disabled) {
		color: var(--accent);
		background: var(--accent-soft);
	}
	.sidehead .iconbtn:disabled {
		opacity: 0.35;
		cursor: default;
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
	.importnotice {
		margin: 8px;
		padding: 10px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--surface);
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 12.5px;
		color: var(--text);
	}
	.importnotice ul {
		margin: 0;
		padding-left: 16px;
		color: var(--muted);
		font-size: 12px;
	}
	.importnotice li.err {
		color: var(--danger);
	}
	.modalback {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.45);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 50;
	}
	.modal {
		width: min(680px, 92vw);
		max-height: 86vh;
		overflow: auto;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 14px 16px 16px;
		display: flex;
		flex-direction: column;
		gap: 10px;
		box-shadow: 0 18px 50px rgba(0, 0, 0, 0.35);
	}
	.pasteta {
		width: 100%;
		min-height: 130px;
		resize: vertical;
		font: 12.5px/1.55 var(--font-mono);
		color: var(--text);
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		padding: 10px 12px;
	}
	.pasteta:focus {
		outline: none;
		border-color: var(--accent);
	}
	.pastename {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 12.5px;
		color: var(--text);
	}
	.pastename input {
		height: 30px;
		padding: 0 10px;
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		background: var(--surface-2);
		color: var(--text);
	}
	.apperror.plain {
		margin: 0;
		padding: 0;
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
