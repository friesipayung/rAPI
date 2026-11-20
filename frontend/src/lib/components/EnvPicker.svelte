<script lang="ts">
	import { onMount } from 'svelte';
	import { EnvService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import type { EnvironmentDoc, Variable } from '#lib/types';

	let { collectionID } = $props();

	let envs = $state<EnvironmentDoc[]>([]);
	let active = $state('');
	let error = $state('');
	let managing = $state(false);

	// manage-modal state
	let editName = $state('');
	let editVars = $state<Variable[]>([]);
	let editOriginal = $state('');

	$effect(() => {
		collectionID;
		load();
	});

	async function load() {
		if (!collectionID) return;
		error = '';
		try {
			envs = ((await EnvService.List(collectionID)) as EnvironmentDoc[]) ?? [];
			const st = await EnvService.ActiveState();
			const want = (st as { activeEnvs?: Record<string, string> })?.activeEnvs?.[collectionID] ?? '';
			active = envs.some((e) => e.name === want) ? want : '';
		} catch (e) {
			error = String(e);
		}
	}

	async function activate() {
		if (!collectionID) return;
		try {
			await EnvService.SetActive(collectionID, active);
		} catch (e) {
			error = String(e);
		}
	}

	function openManage() {
		const env = envs.find((e) => e.name === active);
		startEdit(env?.name ?? '');
		managing = true;
	}

	function startEdit(name: string) {
		const env = envs.find((e) => e.name === name);
		editOriginal = name;
		editName = name;
		editVars = env?.variables ? env.variables.map((v) => ({ ...v })) : [];
	}

	async function saveEnv() {
		error = '';
		if (!editName.trim()) {
			error = 'Environment name required';
			return;
		}
		const vars = editVars.filter((v) => v.name.trim());
		try {
			await EnvService.Save(collectionID, {
				name: editName.trim(),
				variables: vars,
				description: '',
				color: '',
				extends: ''
			} as any);
			managing = false;
			await load();
			active = editName.trim();
			await activate();
		} catch (e) {
			error = String(e);
		}
	}

	async function deleteEnv() {
		if (!editOriginal) return;
		try {
			await EnvService.Delete(collectionID, editOriginal);
			if (active === editOriginal) active = '';
			await activate();
			managing = false;
			await load();
		} catch (e) {
			error = String(e);
		}
	}
</script>

<div class="picker">
	<select bind:value={active} onchange={activate} title="Active environment">
		<option value="">No environment</option>
		{#each envs as env (env.name)}
			<option value={env.name}>{env.name}</option>
		{/each}
	</select>
	<button onclick={openManage} title="Manage environments">⚙</button>
</div>
{#if error}<span class="err" title={error}>⚠</span>{/if}

{#if managing}
	<div class="overlay" role="presentation" onclick={(e) => e.target === e.currentTarget && (managing = false)}>
		<div class="modal">
			<div class="mhead">
				<span>Environments — {collectionID}</span>
				<button onclick={() => (managing = false)}>×</button>
			</div>
			<div class="mtabs">
				<button class:active={editOriginal === ''} onclick={() => startEdit('')}>+ New</button>
				{#each envs as env (env.name)}
					<button class:active={editOriginal === env.name} onclick={() => startEdit(env.name)}>{env.name}</button>
				{/each}
			</div>
			<label class="lbl">Name</label>
			<input class="nameinput" bind:value={editName} placeholder="local / staging / production" />
			<label class="lbl">Variables (used as {'{{name}}'} in URLs, headers, body, auth)</label>
			<div class="vartable">
				{#each editVars as v, i}
					<div class="vrow">
						<input placeholder="name" bind:value={v.name} />
						<input placeholder="value" bind:value={v.value} />
						<button onclick={() => editVars.splice(i, 1)}>×</button>
					</div>
				{/each}
				<button class="addvar" onclick={() => editVars.push({ name: '', value: '' })}>+ Variable</button>
			</div>
			{#if error}<p class="merr">{error}</p>{/if}
			<div class="mfoot">
				{#if editOriginal}
					<button class="danger" onclick={deleteEnv}>Delete env</button>
				{/if}
				<div class="spacer"></div>
				<button class="primary" onclick={saveEnv}>Save environment</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.picker {
		display: flex;
		gap: 6px;
		align-items: center;
	}
	select {
		background: var(--surface-2);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-ctl);
		padding: 5px 8px;
		font: 500 12px Inter, system-ui, sans-serif;
		max-width: 160px;
	}
	.picker button {
		border: 1px solid var(--border);
		background: var(--surface-2);
		color: var(--muted);
		border-radius: var(--radius-ctl);
		padding: 4px 8px;
		cursor: pointer;
	}
	.picker button:hover {
		color: var(--accent);
	}
	.err {
		color: var(--danger);
	}
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.45);
		display: grid;
		place-items: center;
		z-index: 50;
	}
	.modal {
		width: min(520px, calc(100vw - 40px));
		max-height: 80vh;
		overflow: auto;
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow);
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.mhead {
		display: flex;
		justify-content: space-between;
		font-weight: 700;
		font-size: 14px;
	}
	.mhead button,
	.mtabs button,
	.vrow button {
		border: none;
		background: none;
		color: var(--muted);
		cursor: pointer;
		font-size: 15px;
	}
	.mtabs {
		display: flex;
		gap: 6px;
		flex-wrap: wrap;
	}
	.mtabs button {
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 4px 12px;
		font: 500 12px Inter, system-ui, sans-serif;
	}
	.mtabs button.active {
		color: var(--accent);
		border-color: var(--accent);
		background: var(--accent-soft);
	}
	.lbl {
		font: 600 11px/1 Inter, system-ui, sans-serif;
		text-transform: uppercase;
		letter-spacing: 0.6px;
		color: var(--muted);
		margin-top: 6px;
	}
	input {
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		color: var(--text);
		padding: 7px 10px;
		font: 12.5px var(--mono);
	}
	.vartable {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.vrow {
		display: grid;
		grid-template-columns: 1fr 1.4fr 26px;
		gap: 6px;
		align-items: center;
	}
	.vrow button {
		font-size: 15px;
	}
	.addvar {
		border: 1px dashed var(--border);
		background: none;
		color: var(--muted);
		border-radius: var(--radius-ctl);
		padding: 5px 12px;
		cursor: pointer;
		align-self: flex-start;
		font-size: 12px;
	}
	.addvar:hover {
		color: var(--accent);
	}
	.merr {
		color: var(--danger);
		font-size: 12.5px;
		margin: 0;
	}
	.mfoot {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 10px;
	}
	.spacer {
		flex: 1;
	}
	.danger {
		border: 1px solid var(--danger);
		color: var(--danger);
		background: none;
		border-radius: var(--radius-ctl);
		padding: 7px 12px;
		cursor: pointer;
		font: 500 12.5px Inter, system-ui, sans-serif;
	}
	.primary {
		border: none;
		background: var(--accent);
		color: #fff;
		border-radius: var(--radius-ctl);
		padding: 8px 16px;
		cursor: pointer;
		font: 600 12.5px Inter, system-ui, sans-serif;
	}
	.primary:hover {
		background: var(--accent-hover);
	}
</style>
