<script lang="ts">
	import { onMount } from 'svelte';
	import { RequestService, RunService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import KeyValTable from './KeyValTable.svelte';
	import ResponsePanel from './ResponsePanel.svelte';
	import Icon from './Icon.svelte';
	import { BODY_TYPES, METHODS, type Auth, type HttpRequestExample, type RequestDoc, type RunResult } from '#lib/types';

	let { requestID, envName = '', onSaved = () => {} } = $props();

	let doc = $state<RequestDoc | null>(null);
	let error = $state('');
	let notice = $state('');
	let dirty = $state(false);
	let tab = $state<'params' | 'headers' | 'auth' | 'body' | 'settings' | 'docs' | 'examples'>('params');

	let running = $state(false);
	let result = $state<RunResult | null>(null);
	let examples = $state<HttpRequestExample[]>([]);

	$effect(() => {
		requestID;
		load();
	});

	async function load() {
		notice = '';
		error = '';
		dirty = false;
		result = null;
		tab = 'params';
		try {
			doc = (await RequestService.Get(requestID)) as RequestDoc;
			doc.http.params ??= [];
			doc.http.headers ??= [];
			doc.settings ??= {};
			doc.settings.followRedirects ??= true;
			examples = ((await RequestService.ListExamples(requestID)) ?? []) as HttpRequestExample[];
		} catch (e) {
			error = String(e);
		}
	}

	// expose params/headers on the doc shape the editor mutates
	function params(): any[] { return (doc!.http.params ??= []); }
	function headers(): any[] { return (doc!.http.headers ??= []); }
	function touch() {
		dirty = true;
		notice = '';
	}

	async function save() {
		error = '';
		notice = '';
		if (!doc) return;
		try {
			await RequestService.Save(requestID, doc as any);
			dirty = false;
			notice = 'Saved — auto-commit will pick it up';
			onSaved();
		} catch (e) {
			error = String(e);
		}
	}

	async function send() {
		if (!doc?.http?.url?.trim()) {
			error = 'URL is empty';
			return;
		}
		if (dirty) await save();
		error = '';
		running = true;
		result = null;
		try {
			result = (await RunService.Execute(requestID, envName)) as RunResult;
		} catch (e) {
			result = { status: 0, statusText: '', timeMs: 0, sizeBytes: 0, headers: [], error: String(e) };
		} finally {
			running = false;
		}
	}

	function ensureAuth(t: string): Auth {
		doc!.http.auth ??= { type: t } as Auth;
		const a = doc!.http.auth;
		if (a.type !== t) {
			a.type = t;
			a.basic = undefined; a.bearer = undefined; a.apikey = undefined; a.digest = undefined;
		}
		if (t === 'basic' && !a.basic) a.basic = { username: '', password: '' };
		if (t === 'bearer' && !a.bearer) a.bearer = { token: '' };
		if (t === 'apikey' && !a.apikey) a.apikey = { key: '', value: '', in: 'header' };
		if (t === 'digest' && !a.digest) a.digest = { username: '', password: '' };
		return a;
	}

	const TABS = [
		['params', 'Params'],
		['headers', 'Headers'],
		['auth', 'Auth'],
		['body', 'Body'],
		['settings', 'Settings'],
		['docs', 'Docs'],
		['examples', 'Examples']
	] as const;

	function countParams() { return (doc?.http.params ?? []).filter((p: any) => p.name).length; }
	function countHeaders() { return (doc?.http.headers ?? []).filter((h: any) => h.name).length; }

	async function saveExample() {
		if (!result || result.error) return;
		const name = (result.statusText || String(result.status)) + ' — ' + new Date().toLocaleString();
		const label = window.prompt('Example name', name);
		if (label === null) return;
		try {
			await RunService.SaveAsExample(requestID, result as any, label || name);
			examples = ((await RequestService.ListExamples(requestID)) ?? []) as HttpRequestExample[];
			doc = (await RequestService.Get(requestID)) as RequestDoc;
			notice = 'Response saved as example';
		} catch (e) {
			error = String(e);
		}
	}

	async function deleteExample(i: number) {
		if (!window.confirm(`Delete example "${examples[i]?.name ?? i}"?`)) return;
		try {
			await RequestService.DeleteExample(requestID, i);
			examples = ((await RequestService.ListExamples(requestID)) ?? []) as HttpRequestExample[];
			doc = (await RequestService.Get(requestID)) as RequestDoc;
		} catch (e) {
			error = String(e);
		}
	}
</script>

{#if !doc}
	<div class="placeholder">{error || 'Loading…'}</div>
{:else}
	<div class="editor">
		<div class="reqbar">
			<select class="method" bind:value={doc.http.method} onchange={touch}>
				{#each METHODS as m}<option value={m}>{m}</option>{/each}
			</select>
			<input class="url" placeholder="https://api.example.com/anything or {'{{baseUrl}}'}/anything" bind:value={doc.http.url} oninput={touch} />
			<button class="btn primary lg" onclick={send} disabled={running}>
				{#if running}<span class="spinner"></span>{:else}<Icon name="send" size={14} />{/if}
				Send
			</button>
			<button class="btn lg" onclick={save} disabled={!dirty || running} title="Save to vault">
				<Icon name="save" size={14} /> {dirty ? 'Save' : 'Saved'}
			</button>
		</div>
		{#if notice}<span class="notice-text">{notice}</span>{/if}
		{#if error}<span class="error-text">{error}</span>{/if}

		<div class="tabs">
			{#each TABS as [id, label]}
				<button class:active={tab === id} onclick={() => (tab = id)}>
					{label}{id === 'params' && countParams() ? ` (${countParams()})` : ''}{id === 'headers' && countHeaders() ? ` (${countHeaders()})` : ''}{id === 'examples' && examples.length ? ` (${examples.length})` : ''}
				</button>
			{/each}
		</div>

		<div class="tabbody">
			{#if tab === 'params'}
				<KeyValTable rows={params()} keyPlaceholder="Param" valPlaceholder={'Value ({{var}} ok)'} />
			{:else if tab === 'headers'}
				<KeyValTable rows={headers()} keyPlaceholder="Header" valPlaceholder={'Value ({{var}} ok)'} />
			{:else if tab === 'auth'}
				<div class="auth">
					<select bind:value={doc.http.auth!.type} onchange={(e) => ensureAuth((e.target as HTMLSelectElement).value)}>
						<option value="">No auth</option>
						<option value="inherit">Inherit (folder/collection)</option>
						<option value="basic">Basic</option>
						<option value="bearer">Bearer token</option>
						<option value="apikey">API key</option>
						<option value="digest">Digest</option>
					</select>
					{#if doc.http.auth?.type === 'basic'}
						<input placeholder="Username" bind:value={doc.http.auth.basic!.username} oninput={touch} />
						<input type="password" placeholder="Password" bind:value={doc.http.auth.basic!.password} oninput={touch} />
					{:else if doc.http.auth?.type === 'bearer'}
						<input type="password" placeholder={"Token ({{var}} ok)"} bind:value={doc.http.auth.bearer!.token} oninput={touch} />
					{:else if doc.http.auth?.type === 'apikey'}
						<input placeholder="Key (e.g. X-Api-Key)" bind:value={doc.http.auth.apikey!.key} oninput={touch} />
						<input type="password" placeholder={"Value ({{var}} ok)"} bind:value={doc.http.auth.apikey!.value} oninput={touch} />
						<select bind:value={doc.http.auth.apikey!.in} onchange={touch}>
							<option value="header">Header</option>
							<option value="query">Query param</option>
						</select>
					{:else if doc.http.auth?.type === 'digest'}
						<input placeholder="Username" bind:value={doc.http.auth.digest!.username} oninput={touch} />
						<input type="password" placeholder="Password" bind:value={doc.http.auth.digest!.password} oninput={touch} />
					{/if}
					<p class="hint-text">Credentials live in the vault (git). Secrets support arrives in phase 4 (keyring).</p>
				</div>
			{:else if tab === 'body'}
				<div class="bodytab">
					<div class="bodytypes">
						{#each BODY_TYPES as bt}
							<button class="chip" class:active={(doc.http.body?.type ?? '') === bt}
								onclick={() => { doc!.http.body ??= { type: bt, data: '', form: [] }; doc!.http.body.type = bt; touch(); }}>{bt}</button>
						{/each}
					</div>
					{#if doc.http.body && (doc.http.body.type === 'form-urlencoded' || doc.http.body.type === 'multipart-form')}
						<KeyValTable rows={doc.http.body.form ??= []} keyPlaceholder="Field" valPlaceholder={'Value ({{var}} ok)'} />
					{:else if doc.http.body}
						<textarea rows="8" placeholder={'{ "key": "{{value}}" }'} bind:value={doc.http.body.data} oninput={touch}></textarea>
					{:else}
						<p class="hint-text">No body. Pick a type above.</p>
					{/if}
				</div>
			{:else if tab === 'settings'}
				<div class="settings">
					<label><input type="checkbox" bind:checked={doc.settings!.followRedirects} onchange={touch} /> Follow redirects</label>
					<label>Timeout (s, 0 = 30) <input type="number" min="0" bind:value={doc.settings!.timeout} onchange={touch} /></label>
					<label>Max redirects <input type="number" min="0" bind:value={doc.settings!.maxRedirects} onchange={touch} /></label>
				</div>
			{:else if tab === 'examples'}
				<div class="examples">
					{#if examples.length === 0}
						<p class="hint-text">No examples yet. Send a request, then use “Save as example” on the response.</p>
					{:else}
						{#each examples as ex, i}
							<div class="exrow">
								<span class="exname">{ex.name ?? `Example ${i + 1}`}</span>
								{#if ex.response?.status}<span class="chip status">{ex.response.status} {ex.response.statusText ?? ''}</span>{/if}
								<button class="iconbtn" title="Delete example" onclick={() => deleteExample(i)}><Icon name="x" size={13} /></button>
							</div>
						{/each}
					{/if}
				</div>
			{:else}
				<div class="docs">
					<textarea rows="10" placeholder="Markdown documentation for this request (optional)" bind:value={doc.docs} oninput={touch}></textarea>
					<p class="hint-text">Markdown, stored in the request file. Preview and per-folder/collection docs arrive next.</p>
				</div>
			{/if}
		</div>

		<div class="response">
			<ResponsePanel {result} {running} onsaveexample={saveExample} />
		</div>
	</div>
{/if}

<style>
	.editor {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		gap: 8px;
	}
	.reqbar {
		display: flex;
		gap: 8px;
		align-items: center;
	}
	.method {
		flex: none;
		color: var(--accent);
		font: 700 12.5px var(--font-mono);
		width: 108px;
	}
	.url {
		flex: 1;
		font: 13px var(--font-mono);
	}
	.spinner {
		width: 13px;
		height: 13px;
		border: 2px solid rgba(255, 255, 255, 0.35);
		border-top-color: #fff;
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}
	@keyframes spin {
		to { transform: rotate(360deg); }
	}
	.tabs {
		display: flex;
		gap: 2px;
		border-bottom: 1px solid var(--border);
	}
	.tabs button {
		border: none;
		background: none;
		color: var(--muted);
		font: 500 12.5px var(--font-ui);
		padding: 7px 12px;
		cursor: pointer;
		border-bottom: 2px solid transparent;
	}
	.tabs button:hover {
		color: var(--text);
	}
	.tabs button.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
	}
	.tabbody {
		padding: 4px 0;
		max-height: 40%;
		overflow: auto;
	}
	.auth, .bodytab, .docs, .settings {
		display: flex;
		flex-direction: column;
		gap: 8px;
		max-width: 720px;
	}
	.auth select, .auth input {
		max-width: 320px;
	}
	.auth select {
		max-width: 220px;
		font-family: var(--font-ui);
	}
	.bodytypes {
		display: flex;
		gap: 4px;
		flex-wrap: wrap;
	}
	.settings {
		flex-direction: row;
		align-items: center;
		gap: 20px;
		flex-wrap: wrap;
	}
	.settings label {
		display: flex;
		align-items: center;
		gap: 8px;
		color: var(--text);
		font-size: 12.5px;
	}
	.settings input[type='number'] {
		width: 90px;
	}
	.response {
		flex: 1;
		min-height: 0;
		margin: 0 -24px;
	}
	.examples {
		display: flex;
		flex-direction: column;
		gap: 6px;
		max-width: 720px;
	}
	.exrow {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 8px;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm, 6px);
		background: var(--surface);
	}
	.exname {
		flex: 1;
		font-size: 12.5px;
		color: var(--text);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.chip.status {
		font: 600 11px var(--font-mono);
		color: var(--accent);
	}
	.placeholder {
		padding: 24px;
		color: var(--muted);
	}
</style>
