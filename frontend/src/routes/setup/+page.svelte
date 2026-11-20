<script lang="ts">
	import { goto } from '$app/navigation';
	import { VaultService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import { app, listenSyncStatus } from '#lib/state.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import type { VaultStatus } from '#lib/types';

	listenSyncStatus();

	let path = $state('');
	let name = $state('');
	let remote = $state('');
	let step = $state(1);
	let busy = $state(false);
	let error = $state('');
	let done: VaultStatus | null = $state(null);

	async function next() {
		error = '';
		if (step === 3) {
			finish();
			return;
		}
		if (step === 1) {
			if (!path.startsWith('/')) {
				error = 'Enter an absolute path (e.g. /home/you/api-vault).';
				return;
			}
			step = 2;
			return;
		}
		// step 2 → create/adopt the vault
		busy = true;
		try {
			const st = await VaultService.Init(path, name, remote.trim());
			done = st;
			app.vault = st;
			app.ready = true;
			step = 3;
		} catch (e) {
			error = String(e);
		} finally {
			busy = false;
		}
	}

	async function testRemote() {
		error = '';
		busy = true;
		try {
			await VaultService.TestRemote();
			error = '';
		} catch (e) {
			error = String(e);
		} finally {
			busy = false;
		}
	}

	function finish() {
		goto('/workspace', { replaceState: true });
	}
</script>

<div class="dragstrip" aria-hidden="true"></div>
<main>
	<div class="card">
		<h1><Icon name="box" size={24} strokeWidth={2} /> <span><span class="r">r</span>API</span></h1>
		<p class="sub">Set up your vault — a git repo that stores your collections. No account, no cloud; your git, your rules.</p>

		<ol class="steps">
			<li class:active={step === 1} class:done={step > 1}>Vault location</li>
			<li class:active={step === 2} class:done={step > 2}>Remote (optional)</li>
			<li class:active={step === 3}>Ready</li>
		</ol>

		{#if step === 1}
			<label>
				Vault path (local git repo)
				<input bind:value={path} placeholder="/home/you/api-vault" spellcheck="false" />
			</label>
			<label>
				Vault name (optional)
				<input bind:value={name} placeholder="My API Vault" />
			</label>
			<p class="hint">The folder is created if missing. An existing repo is adopted as-is — nothing is overwritten.</p>
		{:else if step === 2}
			<label>
				Git remote URL (optional)
				<input bind:value={remote} placeholder="git@github.com:you/api-vault.git" spellcheck="false" />
			</label>
			<p class="hint">Any git host works (GitHub, GitLab, Gitea, self-hosted). Authentication uses the SSH keys / credential helpers already configured on this machine — rAPI never stores tokens. Leave empty for a local-only vault; you can add a remote later.</p>
		{:else}
			<p class="ok"><Icon name="commit" size={14} /> Vault ready at <code>{done?.path}</code></p>
			{#if done?.hasRemote}
				<p class="hint">Remote: <code>{done?.remoteUrl}</code></p>
			{:else}
				<p class="hint">No remote yet — commits stay local until you add one.</p>
			{/if}
		{/if}

		{#if error}
			<p class="err"><Icon name="alert" size={13} /> {error}</p>
		{/if}

		<div class="actions">
			{#if step > 1 && step < 3}
				<button class="btn ghost" onclick={() => (step -= 1)} disabled={busy}>Back</button>
			{/if}
			<div class="spacer"></div>
			{#if step === 2 && remote.trim()}
				<button class="btn ghost" onclick={testRemote} disabled={busy}>Test connection</button>
			{/if}
			<button class="btn primary lg" onclick={next} disabled={busy}>
				{busy ? 'Working…' : step === 1 ? 'Next' : step === 2 ? 'Create vault' : 'Open workspace'}
			</button>
		</div>
	</div>
</main>

<style>
	/* invisible strip that makes the window draggable on macOS, where the
	   native titlebar is hidden; other platforms keep their native titlebar */
	.dragstrip {
		display: none;
	}
	:global(html[data-platform='mac']) .dragstrip {
		display: block;
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		height: 44px;
		--wails-draggable: drag;
	}
	:global(html[data-platform='mac']) main {
		padding-top: 56px;
	}
	main {
		display: grid;
		place-items: center;
		min-height: 100vh;
		padding: 24px;
	}
	.card {
		width: min(560px, 100%);
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow-1);
		padding: 32px;
	}
	h1 {
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 0 0 6px;
		font-size: 26px;
	}
	h1 span {
		font-weight: 750;
		letter-spacing: 0.4px;
	}
	h1 .r {
		color: var(--accent);
	}
	.sub {
		color: var(--muted);
		margin: 0 0 24px;
	}
	.steps {
		display: flex;
		gap: 8px;
		list-style: none;
		padding: 0;
		margin: 0 0 24px;
	}
	.steps li {
		flex: 1;
		text-align: center;
		font-size: 12px;
		color: var(--muted);
		padding: 6px;
		border-bottom: 2px solid var(--border);
	}
	.steps li.active {
		color: var(--accent);
		border-color: var(--accent);
		font-weight: 600;
	}
	.steps li.done {
		color: var(--success);
		border-color: var(--success);
	}
	label {
		display: block;
		font-weight: 600;
		font-size: 13px;
		margin: 16px 0;
	}
	input {
		display: block;
		width: 100%;
		margin-top: 6px;
		font: 13px var(--font-mono);
	}
	.hint,
	.err,
	.ok {
		font-size: 12.5px;
	}
	.hint {
		color: var(--muted);
	}
	.err {
		display: flex;
		align-items: center;
		gap: 6px;
		color: var(--danger);
		background: var(--danger-soft);
		border-radius: var(--radius-ctl);
		padding: 8px 12px;
	}
	.ok {
		display: flex;
		align-items: center;
		gap: 6px;
		color: var(--success);
		font-weight: 600;
	}
	.ok code {
		font: 12px var(--font-mono);
	}
	.actions {
		display: flex;
		gap: 10px;
		margin-top: 24px;
	}
	.spacer {
		flex: 1;
	}
</style>
