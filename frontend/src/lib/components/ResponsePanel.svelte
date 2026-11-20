<script lang="ts">
	import type { RunResult } from '#lib/types';
	import Icon from './Icon.svelte';

	let { result = null as RunResult | null, running = false, onsaveexample = () => {} } = $props();

	let tab = $state<'body' | 'headers'>('body');

	function statusClass(s: number) {
		if (s >= 200 && s < 300) return 'ok';
		if (s >= 300 && s < 400) return 'redir';
		if (s >= 400) return 'err';
		return 'info';
	}
	function fmtSize(b: number) {
		if (b < 1024) return `${b} B`;
		if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
		return `${(b / 1024 / 1024).toFixed(2)} MB`;
	}
</script>

<div class="panel">
	<div class="head">
		<span class="title">Response</span>
		{#if running}
			<span class="running">sending…</span>
		{:else if result?.error}
			<span class="stat err">Error</span>
		{:else if result}
			<span class="stat {statusClass(result.status)}">{result.status} {result.statusText?.replace(/^\d+\s*/, '')}</span>
			<span class="meta">{result.timeMs} ms · {fmtSize(result.sizeBytes)}{result.envUsed ? ` · env: ${result.envUsed}` : ''}</span>
		{/if}
		<div class="spacer"></div>
		{#if result && !result.error && !running}
			<button class="iconbtn saveex" title="Save this response as an example" onclick={() => onsaveexample()}>
				<Icon name="save" size={13} /> Save as example
			</button>
		{/if}
		{#if result}
			<button class:active={tab === 'body'} onclick={() => (tab = 'body')}>Body</button>
			<button class:active={tab === 'headers'} onclick={() => (tab = 'headers')}>Headers ({result.headers?.length ?? 0})</button>
		{/if}
	</div>

	{#if running}
		<div class="placeholder">Waiting for response…</div>
	{:else if !result}
		<div class="placeholder">Send the request to see the response.</div>
	{:else if result.error}
		<pre class="error">{result.error}</pre>
	{:else if tab === 'headers'}
		<div class="headers">
			{#each result.headers ?? [] as h}
				<div class="hrow">
					<span class="hname">{h.name}</span>
					<span class="hval">{h.value}</span>
				</div>
			{/each}
		</div>
	{:else}
		<pre class="body">{result.body?.data || '(empty body)'}</pre>
	{/if}
</div>

<style>
	.panel {
		display: flex;
		flex-direction: column;
		min-height: 0;
		height: 100%;
		border-top: 1px solid var(--border);
		background: var(--surface);
	}
	.head {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 8px 12px;
		border-bottom: 1px solid var(--border);
	}
	.title {
		font: 600 11px/1 var(--font-ui);
		text-transform: uppercase;
		letter-spacing: 0.8px;
		color: var(--muted);
	}
	.spacer {
		flex: 1;
	}
	.head button {
		border: none;
		background: none;
		color: var(--muted);
		font: 500 12px var(--font-ui);
		cursor: pointer;
		padding: 4px 8px;
		border-radius: 6px;
	}
	.head button.active {
		color: var(--accent);
		background: var(--accent-soft);
	}
	.saveex {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font: 500 12px var(--font-ui);
		color: var(--muted);
		padding: 4px 8px;
	}
	.saveex:hover {
		color: var(--accent);
	}
	.stat {
		font: 700 12.5px var(--font-mono);
		padding: 3px 8px;
		border-radius: 6px;
	}
	.stat.ok {
		color: var(--ok, #1a7f37);
		background: rgba(26, 127, 55, 0.12);
	}
	.stat.redir {
		color: #9a6700;
		background: rgba(154, 103, 0, 0.12);
	}
	.stat.err {
		color: var(--danger);
		background: var(--accent-soft);
	}
	.stat.info {
		color: var(--muted);
		background: var(--surface-2);
	}
	.meta {
		color: var(--muted);
		font-size: 12px;
	}
	.running {
		color: var(--muted);
		font-size: 12.5px;
	}
	.placeholder {
		padding: 20px;
		color: var(--muted);
		font-size: 12.5px;
	}
	pre.body,
	pre.error {
		margin: 0;
		padding: 14px;
		overflow: auto;
		flex: 1;
		font: 12px/1.55 var(--font-mono);
		color: var(--text);
		white-space: pre-wrap;
		word-break: break-word;
	}
	pre.error {
		color: var(--danger);
	}
	.headers {
		padding: 10px 14px;
		overflow: auto;
	}
	.hrow {
		display: flex;
		gap: 12px;
		padding: 3px 0;
		font: 12px var(--font-mono);
	}
	.hname {
		color: var(--accent);
		min-width: 200px;
	}
	.hval {
		color: var(--text);
		word-break: break-all;
	}
</style>
