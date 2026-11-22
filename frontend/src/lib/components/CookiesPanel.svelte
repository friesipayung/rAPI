<script lang="ts">
	// Cookies drawer (P0 task 6.4): lists the cookies the app's request
	// engine has stored for the current vault (values arrive truncated from
	// the Go service — they are credentials) and offers a Clear-all. The
	// store lives outside the vault; see internal/cookies.
	import { CookieService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import Icon from './Icon.svelte';
	import type { CookieView } from '#lib/types';

	let { onclose }: { onclose: () => void } = $props();

	let cookies = $state<CookieView[]>([]);
	let error = $state('');
	let busy = $state(false);

	function fmtExpires(iso: string): string {
		if (!iso) return 'session';
		const t = new Date(iso);
		return isNaN(t.getTime()) ? iso : t.toLocaleString();
	}

	async function load() {
		error = '';
		try {
			cookies = ((await CookieService.List()) ?? []) as CookieView[];
		} catch (e) {
			error = String(e);
		}
	}

	async function clearAll() {
		if (!confirm('Delete all stored cookies?')) return;
		busy = true;
		error = '';
		try {
			await CookieService.Clear();
			await load();
		} catch (e) {
			error = String(e);
		} finally {
			busy = false;
		}
	}

	load();
</script>

<div class="drawer">
	<div class="drawerhead">
		<span class="title">Cookies ({cookies.length})</span>
		<span class="headbtns">
			<button class="btn danger sm" onclick={clearAll} disabled={busy || cookies.length === 0} title="Delete all stored cookies">
				<Icon name="trash" size={12} /> Clear all
			</button>
			<button class="iconbtn" onclick={onclose} title="Close"><Icon name="x" size={14} /></button>
		</span>
	</div>

	{#if error}<p class="error-text">{error}</p>{/if}

	{#if cookies.length === 0}
		<p class="empty">No cookies stored.</p>
	{:else}
		<div class="tablewrap">
			<table>
				<thead>
					<tr><th>Domain</th><th>Name</th><th>Value</th><th>Expires</th></tr>
				</thead>
				<tbody>
					{#each cookies as c (c.domain + '/' + c.name)}
						<tr>
							<td class="dom" title={c.domain}>{c.domain}</td>
							<td title={c.name}>{c.name}</td>
							<td class="val" title={c.value}>{c.value}</td>
							<td class="exp">{fmtExpires(c.expires)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<style>
	/* same container as the git-log drawer in +page.svelte (scoped copy) */
	.drawer {
		position: fixed;
		right: 16px;
		top: 58px;
		width: min(560px, calc(100vw - 32px));
		max-height: 60vh;
		display: flex;
		flex-direction: column;
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
		flex: none;
		gap: 8px;
	}
	.title {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.headbtns {
		display: flex;
		align-items: center;
		gap: 6px;
		flex: none;
	}
	.headbtns .iconbtn {
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
	.headbtns .iconbtn:hover {
		color: var(--accent);
		background: var(--accent-soft);
	}
	.btn.danger.sm {
		font-size: 11.5px;
		padding: 3px 8px;
	}
	.tablewrap {
		flex: 1;
		min-height: 0;
		overflow: auto;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 12px;
		color: var(--text);
	}
	th {
		text-align: left;
		font: 600 10.5px/1 var(--font-ui);
		text-transform: uppercase;
		letter-spacing: 0.6px;
		color: var(--muted);
		padding: 4px 6px;
		border-bottom: 1px solid var(--border);
		position: sticky;
		top: 0;
		background: var(--surface);
	}
	td {
		padding: 5px 6px;
		border-bottom: 1px solid var(--border);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		max-width: 150px;
	}
	td.dom {
		color: var(--accent);
	}
	/* cookie values are credentials: monospace + ellipsis, never wide */
	td.val {
		font: 11.5px var(--font-mono);
		color: var(--muted);
	}
	td.exp {
		color: var(--muted);
		font-size: 11px;
	}
	.empty {
		color: var(--muted);
		font-size: 12.5px;
		padding: 8px;
	}
</style>
