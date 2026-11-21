<script lang="ts">
	import { onMount } from 'svelte';
	import { RequestService, RunService, CodegenService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import DOMPurify from 'dompurify';
	import { marked } from 'marked';
	import KeyValTable from './KeyValTable.svelte';
	import ResponsePanel from './ResponsePanel.svelte';
	import Icon from './Icon.svelte';
	import { pushRun, getRuns, type RunHistoryEntry } from '#lib/runhistory';
	import { BODY_TYPES, METHODS, ASSERTION_OPERATORS, type Assertion, type Auth, type HttpRequestExample, type RequestDoc, type RunResult, type Script } from '#lib/types';

	let { requestID, envName = '', onSaved = () => {} } = $props();

	let doc = $state<RequestDoc | null>(null);
	let error = $state('');
	let notice = $state('');
	let dirty = $state(false);
	let tab = $state<'params' | 'headers' | 'auth' | 'body' | 'settings' | 'docs' | 'examples' | 'scripts' | 'tests' | 'code'>('params');

	// code generation tab
	interface CodegenTargetInfo { key: string; label: string; syntax: string }
	let codeTargets = $state<CodegenTargetInfo[]>([]);
	let codeTarget = $state('curl');
	let codeOut = $state('');
	let codeErr = $state('');
	let codeBusy = $state(false);
	let copied = $state(false);
	async function loadCode() {
		if (!codeTargets.length) {
			codeTargets = ((await CodegenService.Targets()) ?? []) as unknown as CodegenTargetInfo[];
		}
		codeBusy = true;
		codeErr = '';
		try {
			codeOut = ((await CodegenService.Generate(requestID, codeTarget)) ?? '') as unknown as string;
		} catch (e) {
			codeErr = String(e);
			codeOut = '';
		} finally {
			codeBusy = false;
		}
	}
	async function copyCode() {
		try {
			await navigator.clipboard.writeText(codeOut);
			copied = true;
			setTimeout(() => (copied = false), 1500);
		} catch { /* clipboard unavailable */ }
	}

	let running = $state(false);
	let result = $state<RunResult | null>(null);
	let examples = $state<HttpRequestExample[]>([]);

	// run history (session-scoped, never persisted)
	let runs = $state<RunHistoryEntry[]>([]);
	let runSel = $state(-1); // index into runs; -1 = current
	function showRun(i: number) {
		runSel = i;
		result = i >= 0 && i < runs.length ? runs[i].result : result;
	}

	$effect(() => {
		requestID;
		load();
	});

	// lazy-load the snippet the first time the Code tab opens
	let codeLoadedOnce = false;
	$effect(() => {
		if (tab === 'code' && !codeLoadedOnce) {
			codeLoadedOnce = true;
			loadCode();
		}
	});

	// request docs preview
	let docsPreview = $state(false);
	function docsRender() {
		return DOMPurify.sanitize(marked.parse(doc?.docs || '') as string);
	}

	async function load() {
		notice = '';
		error = '';
		dirty = false;
		tab = 'params';
		runs = getRuns(requestID);
		runSel = -1;
		result = runs[0]?.result ?? null;
		try {
			doc = (await RequestService.Get(requestID)) as RequestDoc;
			doc.http.params ??= [];
			doc.http.headers ??= [];
			doc.settings ??= {};
			doc.settings.followRedirects ??= true;
			doc.settings.proxy ??= { url: '' };
			doc.settings.clientCert ??= { cert: '', key: '' };
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
			pushRun(requestID, result);
			runs = getRuns(requestID);
			runSel = 0;
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
			a.basic = undefined; a.bearer = undefined; a.apikey = undefined; a.digest = undefined; a.oauth2 = undefined; a.awsv4 = undefined; a.wsse = undefined;
		}
		if (t === 'basic' && !a.basic) a.basic = { username: '', password: '' };
		if (t === 'bearer' && !a.bearer) a.bearer = { token: '' };
		if (t === 'apikey' && !a.apikey) a.apikey = { key: '', value: '', in: 'header' };
		if (t === 'digest' && !a.digest) a.digest = { username: '', password: '' };
		if (t === 'oauth2' && !a.oauth2) a.oauth2 = { grantType: 'client_credentials', tokenUrl: '', clientId: '', clientSecret: '' };
		if (t === 'awsv4' && !a.awsv4) a.awsv4 = { accessKey: '', secretKey: '', region: '', service: '' };
		if (t === 'wsse' && !a.wsse) a.wsse = { username: '', password: '' };
		if (t === 'ntlm' && !a.digest) a.digest = { username: '', password: '' }; // NTLM reuses the digest creds shape
		return a;
	}

	const TABS = [
		['params', 'Params'],
		['headers', 'Headers'],
		['auth', 'Auth'],
		['body', 'Body'],
		['scripts', 'Scripts'],
		['tests', 'Tests'],
		['settings', 'Settings'],
		['docs', 'Docs'],
		['examples', 'Examples'],
		['code', 'Code']
	] as const;

	function countParams() { return (doc?.http.params ?? []).filter((p: any) => p.name).length; }
	function countHeaders() { return (doc?.http.headers ?? []).filter((h: any) => h.name).length; }
	function countAssertions() { return (doc?.runtime?.assertions ?? []).filter((a) => a.expression.trim()).length; }

	function script(kind: 'pre' | 'post'): Script {
		const list = (doc!.runtime ??= {}).scripts ??= [];
		let s = list.find((x) => x.type === kind);
		if (!s) {
			s = { type: kind, content: '' };
			list.push(s);
		}
		return s;
	}
	function assertions(): Assertion[] { return (doc!.runtime ??= {}).assertions ??= []; }
	function addAssertion() {
		assertions().push({ expression: '', operator: 'equal', value: '' });
		touch();
	}

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
						<option value="oauth2">OAuth 2</option>
						<option value="awsv4">AWS SigV4</option>
						<option value="wsse">WSSE</option>
						<option value="ntlm">NTLM</option>
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
					{:else if doc.http.auth?.type === 'oauth2'}
						<div class="auth">
							<select bind:value={doc.http.auth.oauth2!.grantType} onchange={touch}>
								<option value="client_credentials">Client credentials</option>
								<option value="password">Password</option>
							</select>
							<input class="wide" placeholder="Token URL (https://…/oauth/token, {'{{var}}'} ok)" bind:value={doc.http.auth.oauth2!.tokenUrl} oninput={touch} />
							<input placeholder="Client ID" bind:value={doc.http.auth.oauth2!.clientId} oninput={touch} />
							<input type="password" placeholder="Client secret" bind:value={doc.http.auth.oauth2!.clientSecret} oninput={touch} />
							{#if doc.http.auth.oauth2!.grantType === 'password'}
								<input placeholder="Username" bind:value={doc.http.auth.oauth2!.username} oninput={touch} />
								<input type="password" placeholder="Password" bind:value={doc.http.auth.oauth2!.password} oninput={touch} />
							{/if}
							<input placeholder="Scopes (space separated, optional)" bind:value={doc.http.auth.oauth2!.scopes} oninput={touch} />
						</div>
					{:else if doc.http.auth?.type === 'awsv4'}
						<div class="auth">
							<input placeholder="Access key" bind:value={doc.http.auth.awsv4!.accessKey} oninput={touch} />
							<input type="password" placeholder="Secret key" bind:value={doc.http.auth.awsv4!.secretKey} oninput={touch} />
							<input placeholder="Session token (optional)" bind:value={doc.http.auth.awsv4!.sessionToken} oninput={touch} />
							<input placeholder="Region (e.g. us-east-1)" bind:value={doc.http.auth.awsv4!.region} oninput={touch} />
							<input placeholder="Service (e.g. s3, iam)" bind:value={doc.http.auth.awsv4!.service} oninput={touch} />
						</div>
					{:else if doc.http.auth?.type === 'wsse'}
						<div class="auth">
							<input placeholder="Username" bind:value={doc.http.auth.wsse!.username} oninput={touch} />
							<input type="password" placeholder="Password" bind:value={doc.http.auth.wsse!.password} oninput={touch} />
						</div>
					{:else if doc.http.auth?.type === 'ntlm'}
						<div class="auth">
							<input placeholder="Username (DOMAIN\\user)" bind:value={doc.http.auth.digest!.username} oninput={touch} />
							<input type="password" placeholder="Password" bind:value={doc.http.auth.digest!.password} oninput={touch} />
							<p class="hint-text">Credentials are used for the NTLM/Negotiate handshake (handled by the transport).</p>
						</div>
					{/if}
					<p class="hint-text">Credentials live in the vault (git). Secret variables (env) are stored in the OS keyring, never in git.</p>
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
					<label>Proxy URL <input class="proxyurl" placeholder="http://proxy:8080 (empty = off)" bind:value={doc.settings!.proxy!.url} oninput={touch} /></label>
					<label>Client cert (PEM) <input class="proxyurl" placeholder="/path/to/cert.pem" bind:value={doc.settings!.clientCert!.cert} oninput={touch} /></label>
					<label>Client key (PEM) <input class="proxyurl" placeholder="/path/to/key.pem" bind:value={doc.settings!.clientCert!.key} oninput={touch} /></label>
				</div>
			{:else if tab === 'scripts'}
				<div class="scripts">
					<label class="field-label" for="prescript">Pre-request script</label>
					<textarea id="prescript" class="code" rows="6" placeholder={'// rapi.vars.set("ts", Date.now())\n// rapi.env.get("baseUrl") → rapi.env.set("baseUrl", …)'} value={script('pre').content} oninput={(e) => { script('pre').content = (e.target as HTMLTextAreaElement).value; touch(); }}></textarea>
					<label class="field-label" for="postscript">Post-response script</label>
					<textarea id="postscript" class="code" rows="6" placeholder={'// rapi.response.status → 200\n// rapi.response.body → parsed JSON\n// rapi.vars.set("token", rapi.response.body.access_token)'} value={script('post').content} oninput={(e) => { script('post').content = (e.target as HTMLTextAreaElement).value; touch(); }}></textarea>
					<p class="hint-text">Plain JavaScript (goja). rapi.request, rapi.response, rapi.env / rapi.vars (get/set), console.log.</p>
				</div>
			{:else if tab === 'tests'}
				<div class="tests">
					{#if assertions().length === 0}
						<p class="hint-text">No assertions. Add a check like res.status equal 200.</p>
					{:else}
						{#each assertions() as a, i}
							<div class="testrow">
								<input class="expr" placeholder="res.status" bind:value={a.expression} oninput={touch} />
								<select bind:value={a.operator} onchange={touch}>
									{#each ASSERTION_OPERATORS as op}<option value={op}>{op}</option>{/each}
								</select>
								<input class="val" placeholder="expected (empty for exists)" bind:value={a.value} oninput={touch} />
								<button class="iconbtn" title="Remove assertion" onclick={() => { assertions().splice(i, 1); touch(); }}><Icon name="x" size={13} /></button>
							</div>
						{/each}
					{/if}
					<button class="btn dashed sm" onclick={addAssertion}><Icon name="plus" size={12} /> Assertion</button>
					<p class="hint-text">Expressions run against res: res.status, res.body.…, res.headers['name'], res.timeMs.</p>
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
			{:else if tab === 'code'}
				<div class="codegen">
					<div class="coderow">
						<select bind:value={codeTarget} onchange={loadCode}>
							{#each codeTargets as t}<option value={t.key}>{t.label}</option>{/each}
						</select>
						<button class="btn" onclick={loadCode} disabled={codeBusy}><Icon name="history" size={13} /> {codeBusy ? 'Generating…' : 'Regenerate'}</button>
						<div class="spacer"></div>
						<button class="btn primary" onclick={copyCode} disabled={!codeOut}><Icon name={copied ? 'save' : 'clipboard'} size={13} /> {copied ? 'Copied' : 'Copy'}</button>
					</div>
					{#if codeErr}
						<p class="codeerr">{codeErr}</p>
					{:else}
						<pre class="snippet">{codeOut}</pre>
					{/if}
					<p class="hint-text">Generated from the saved request — save changes first (Ctrl+S). {'{{variables}}'} stay verbatim; replace them before running.</p>
				</div>
			{:else}
				<div class="docs">
					<div class="dochead">
						<div class="spacer"></div>
						<button class="chip" class:active={!docsPreview} onclick={() => (docsPreview = false)}>Edit</button>
						<button class="chip" class:active={docsPreview} onclick={() => (docsPreview = true)}>Preview</button>
					</div>
					{#if docsPreview}
						<div class="docpreview">{@html docsRender()}</div>
					{:else}
						<textarea rows="10" placeholder="Markdown documentation for this request (optional)" bind:value={doc.docs} oninput={touch}></textarea>
					{/if}
					<p class="hint-text">Markdown, stored in the request file.</p>
				</div>
			{/if}
		</div>

		<div class="response">
			{#if runs.length}
				<div class="runhist">
					{#each runs as r, i}
						<button class="rhitem" class:active={runSel === i} onclick={() => showRun(i)} title={r.result.error ? String(r.result.error) : r.result.statusText}>
							<span class="rhstatus" class:ok={r.result.status >= 200 && r.result.status < 300} class:warn={r.result.status >= 300 && r.result.status < 400} class:err={r.result.status >= 400 || r.result.status === 0}>{r.result.status || 'ERR'}</span>
							<span class="rhtime">{new Date(r.at).toLocaleTimeString()}</span>
						</button>
					{/each}
					{#if runSel > 0}
						<button class="rhitem current" onclick={() => showRun(-1)} title="Show the latest run">latest</button>
					{/if}
				</div>
			{/if}
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
	.dochead {
		display: flex;
		align-items: center;
		gap: 4px;
	}
	.dochead .spacer {
		flex: 1;
	}
	.docpreview {
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		padding: 12px 14px;
		background: var(--surface-2);
		font: 13.5px/1.6 var(--font-ui);
		min-height: 120px;
		max-width: 720px;
	}
	.docpreview :global(h1), .docpreview :global(h2) {
		font-size: 16px;
		margin: 0.4em 0;
	}
	.docpreview :global(code) {
		font-family: var(--font-mono);
		background: var(--accent-soft);
		padding: 1px 5px;
		border-radius: 4px;
		font-size: 12px;
	}
	.docpreview :global(pre code) {
		display: block;
		padding: 10px;
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
	.runhist {
		display: flex;
		gap: 4px;
		overflow-x: auto;
		padding: 0 24px 8px;
		scrollbar-width: thin;
	}
	.rhitem {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 2px 7px;
		border: 1px solid var(--border);
		border-radius: 999px;
		background: var(--surface);
		color: var(--muted);
		font: 11px var(--font-ui);
		cursor: pointer;
		flex: none;
	}
	.rhitem.active {
		border-color: var(--accent);
		color: var(--text);
	}
	.rhstatus {
		font-weight: 700;
		color: var(--muted);
	}
	.rhstatus.ok {
		color: var(--ok, #1a7f37);
	}
	.rhstatus.warn {
		color: var(--warn, #b58900);
	}
	.rhstatus.err {
		color: var(--danger, var(--accent));
	}
	.rhtime {
		font: 10.5px var(--font-mono);
	}
	.examples {
		display: flex;
		flex-direction: column;
		gap: 6px;
		max-width: 720px;
	}
	.codegen {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.coderow {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.coderow select {
		height: 28px;
		padding: 0 8px;
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		background: var(--surface);
		color: var(--text);
	}
	.snippet {
		margin: 0;
		padding: 12px 14px;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		font: 12px/1.55 var(--font-mono);
		color: var(--text);
		overflow: auto;
		max-height: 420px;
		white-space: pre;
	}
	.codeerr {
		margin: 0;
		color: var(--danger);
		font-size: 12.5px;
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
	.scripts, .tests {
		display: flex;
		flex-direction: column;
		gap: 8px;
		max-width: 860px;
	}
	.scripts .code, textarea.code {
		font: 12px/1.5 var(--font-mono);
		resize: vertical;
	}
	.testrow {
		display: flex;
		gap: 6px;
		align-items: center;
	}
	.testrow .expr {
		flex: 2;
		font: 12.5px var(--font-mono);
	}
	.testrow select {
		flex: none;
		width: 130px;
		font-family: var(--font-ui);
	}
	.testrow .val {
		flex: 1;
		font: 12.5px var(--font-mono);
	}
	.proxyurl {
		width: 260px;
		font: 12.5px var(--font-mono);
	}
	.auth .wide {
		max-width: 480px;
	}
	.placeholder {
		padding: 24px;
		color: var(--muted);
	}
</style>
