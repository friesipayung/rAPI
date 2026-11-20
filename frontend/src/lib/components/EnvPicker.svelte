<script lang="ts">
	import { onMount } from 'svelte';
	import { EnvService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import Icon from './Icon.svelte';
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
		const vars = editVars.filter((v) => v.name.trim()).map((v) => (v.secret ? { ...v, value: '' } : v));
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

	async function setSecret(v: Variable) {
		if (!editName.trim() || !v.name.trim()) {
			error = 'Set the env + variable name first';
			return;
		}
		const val = window.prompt(`Value for secret "${v.name}" (stored in the OS keyring, never in git)`);
		if (val === null) return;
		try {
			await EnvService.SetSecret(collectionID, editName.trim(), v.name.trim(), val);
			v.secret = true;
			v.value = '';
		} catch (e) {
			error = String(e);
		}
	}

	async function deleteSecret(v: Variable) {
		if (!editName.trim() || !v.name.trim()) return;
		if (!window.confirm(`Remove secret "${v.name}" from the OS keyring?`)) return;
		try {
			await EnvService.DeleteSecret(collectionID, editName.trim(), v.name.trim());
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
	<button class="iconbtn" onclick={openManage} title="Manage environments"><Icon name="sliders" size={14} /></button>
</div>
{#if error}<span class="err" title={error}><Icon name="alert" size={13} /></span>{/if}

{#if managing}
	<div class="overlay" role="presentation" onclick={(e) => e.target === e.currentTarget && (managing = false)}>
		<div class="modal">
			<div class="modal-head">
				<span>Environments — {collectionID}</span>
				<button class="iconbtn" onclick={() => (managing = false)} title="Close"><Icon name="x" size={14} /></button>
			</div>
			<div class="mtabs">
				<button class="chip" class:active={editOriginal === ''} onclick={() => startEdit('')}>New</button>
				{#each envs as env (env.name)}
					<button class="chip" class:active={editOriginal === env.name} onclick={() => startEdit(env.name)}>{env.name}</button>
				{/each}
			</div>
			<label class="field-label" for="envname">Name</label>
			<input id="envname" bind:value={editName} placeholder="local / staging / production" />
			<label class="field-label" for="envvars">Variables (used as {'{{name}}'} in URLs, headers, body, auth)</label>
			<div class="vartable">
				{#each editVars as v, i}
					<div class="vrow">
						<input placeholder="name" bind:value={v.name} />
						{#if v.secret}
							<input type="password" placeholder="••• stored in OS keyring" disabled />
							<button class="iconbtn" title="Remove from keyring" onclick={() => deleteSecret(v)}><Icon name="x" size={13} /></button>
						{:else}
							<input placeholder="value" bind:value={v.value} />
						{/if}
						<label class="secretflag" title="Store the value in the OS keyring (never in git)">
							<input type="checkbox" bind:checked={v.secret} onchange={(e) => (e.target as HTMLInputElement).checked && setSecret(v)} />
							secret
						</label>
						<button class="iconbtn" onclick={() => editVars.splice(i, 1)} title="Remove variable"><Icon name="x" size={13} /></button>
					</div>
				{/each}
				<button class="btn dashed" onclick={() => editVars.push({ name: '', value: '' })}><Icon name="plus" size={12} /> Variable</button>
			</div>
			{#if error}<p class="error-text">{error}</p>{/if}
			<div class="mfoot">
				{#if editOriginal}
					<button class="btn danger" onclick={deleteEnv}>Delete env</button>
				{/if}
				<div class="spacer"></div>
				<button class="btn primary" onclick={saveEnv}>Save environment</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.picker {
		display: flex;
		gap: 4px;
		align-items: center;
	}
	select {
		max-width: 170px;
		font-size: 12.5px;
		font-weight: 500;
	}
	.err {
		display: inline-flex;
		color: var(--danger);
	}
	.mtabs {
		display: flex;
		gap: 6px;
		flex-wrap: wrap;
	}
	.vartable {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.vrow {
		display: grid;
		grid-template-columns: 1fr 1.4fr auto 30px;
		gap: 6px;
		align-items: center;
	}
	.secretflag {
		display: flex;
		align-items: center;
		gap: 4px;
		font-size: 11.5px;
		color: var(--muted);
		white-space: nowrap;
	}
	.vrow .iconbtn {
		color: var(--muted);
	}
	.vrow .iconbtn:hover:not(:disabled) {
		color: var(--danger);
		background: var(--danger-soft);
	}
	.mfoot {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 6px;
	}
	.spacer {
		flex: 1;
	}
</style>
