<script lang="ts">
	import { onMount } from 'svelte';
	import { RequestService, RunService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import KeyValTable from './KeyValTable.svelte';
	import ResponsePanel from './ResponsePanel.svelte';
	import { BODY_TYPES, METHODS, type Auth, type RequestDoc, type RunResult } from '#lib/types';

	let { requestID, envName = '', onSaved = () => {} } = $props();

	let doc = $state<RequestDoc | null>(null);
	let error = $state('');
	let notice = $state('');
	let dirty = $state(false);
	let tab = $state<'params' | 'headers' | 'auth' | 'body' | 'settings' | 'docs'>('params');

	let running = $state(false);
	let result = $state<RunResult | null>(null);

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
		['docs', 'Docs']
	] as const;

	function countParams() { return (doc?.http.params ?? []).filter((p: any) => p.name).length; }
	function countHeaders() { return (doc?.http.headers ?? []).filter((h: any) => h.name).length; }
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
			<button class="send" onclick={send} disabled={running}>{running ? '…' : 'Send'}</button>
			<button class="savebtn" onclick={save} disabled={!dirty || running} title="Save to vault">{dirty ? 'Save •' : 'Saved'}</button>
		</div>
		{#if notice}<span class="notice">{notice}</span>{/if}
		{#if error}<span class="err">{error}</span>{/if}

		<div class="tabs">
			{#each TABS as [id, label]}
				<button class:active={tab === id} onclick={() => (tab = id)}>
					{label}{id === 'params' && countParams() ? ` (${countParams()})` : ''}{id === 'headers' && countHeaders() ? ` (${countHeaders()})` : ''}
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
					<p class="hint">Credentials live in the vault (git). Secrets support arrives in phase 4 (keyring).</p>
				</div>
			{:else if tab === 'body'}
				<div class="bodytab">
					<div class="bodytypes">
						{#each BODY_TYPES as bt}
							<button class:active={(doc.http.body?.type ?? '') === bt}
								onclick={() => { doc!.http.body ??= { type: bt, data: '', form: [] }; doc!.http.body.type = bt; touch(); }}>{bt}</button>
						{/each}
					</div>
					{#if doc.http.body && (doc.http.body.type === 'form-urlencoded' || doc.http.body.type === 'multipart-form')}
						<KeyValTable rows={doc.http.body.form ??= []} keyPlaceholder="Field" valPlaceholder={'Value ({{var}} ok)'} />
					{:else if doc.http.body}
						<textarea rows="8" placeholder={'{ "key": "{{value}}" }'} bind:value={doc.http.body.data} oninput={touch}></textarea>
					{:else}
						<p class="hint">No body. Pick a type above.</p>
					{/if}
				</div>
			{:else if tab === 'settings'}
				<div class="settings">
					<label><input type="checkbox" bind:checked={doc.settings!.followRedirects} onchange={touch} /> Follow redirects</label>
					<label>Timeout (s, 0 = 30) <input type="number" min="0" bind:value={doc.settings!.timeout} onchange={touch} /></label>
					<label>Max redirects <input type="number" min="0" bind:value={doc.settings!.maxRedirects} onchange={touch} /></label>
				</div>
			{:else}
				<div class="docs">
					<textarea rows="10" placeholder="Markdown documentation for this request (optional)" bind:value={doc.docs} oninput={touch}></textarea>
					<p class="hint">Markdown, stored in the request file. Preview and per-folder/collection docs arrive next.</p>
				</div>
			{/if}
		</div>

		<div class="response">
			<ResponsePanel {result} {running} />
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
		background: var(--surface-2);
		border: 1px solid var(--border);
		color: var(--accent);
		font: 700 12.5px var(--mono);
		border-radius: var(--radius-ctl);
		padding: 9px 8px;
	}
	.url {
		flex: 1;
		background: var(--surface-2);
		border: 1px solid var(--border);
		color: var(--text);
		font: 13px var(--mono);
		border-radius: var(--radius-ctl);
		padding: 9px 12px;
	}
	.url:focus {
		outline: 1px solid var(--accent);
	}
	.send {
		border: none;
		background: var(--accent);
		color: #fff;
		font: 700 13px Inter, system-ui, sans-serif;
		border-radius: var(--radius-ctl);
		padding: 9px 20px;
		cursor: pointer;
	}
	.send:hover {
		background: var(--accent-hover);
	}
	.send:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.savebtn {
		border: 1px solid var(--border);
		background: var(--surface-2);
		color: var(--text);
		font: 500 12.5px Inter, system-ui, sans-serif;
		border-radius: var(--radius-ctl);
		padding: 9px 14px;
		cursor: pointer;
	}
	.savebtn:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.notice {
		color: var(--ok, #1a7f37);
		font-size: 12px;
	}
	.err {
		color: var(--danger);
		font-size: 12px;
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
		font: 500 12.5px Inter, system-ui, sans-serif;
		padding: 7px 12px;
		cursor: pointer;
		border-bottom: 2px solid transparent;
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
	.auth select, .auth input, .settings input {
		background: var(--surface-2);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-ctl);
		padding: 8px 10px;
		font: 12.5px var(--mono);
		max-width: 320px;
	}
	.auth select {
		max-width: 220px;
		font-family: Inter, system-ui, sans-serif;
	}
	.hint {
		color: var(--muted);
		font-size: 12px;
		margin: 4px 0 0;
	}
	.bodytypes {
		display: flex;
		gap: 4px;
		flex-wrap: wrap;
	}
	.bodytypes button {
		border: 1px solid var(--border);
		background: var(--surface-2);
		color: var(--muted);
		border-radius: 999px;
		padding: 4px 12px;
		font: 500 11.5px Inter, system-ui, sans-serif;
		cursor: pointer;
	}
	.bodytypes button.active {
		color: var(--accent);
		border-color: var(--accent);
		background: var(--accent-soft);
	}
	textarea {
		background: var(--surface-2);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-ctl);
		padding: 10px;
		font: 12.5px/1.5 var(--mono);
		resize: vertical;
	}
	textarea:focus {
		outline: 1px solid var(--accent);
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
	.settings input[type='checkbox'] {
		max-width: none;
	}
	.response {
		flex: 1;
		min-height: 0;
		margin: 0 -24px;
	}
	.placeholder {
		padding: 24px;
		color: var(--muted);
	}
</style>
