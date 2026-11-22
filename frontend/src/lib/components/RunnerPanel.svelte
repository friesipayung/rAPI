<script lang="ts">
	// Collection runner drawer (P0 task 4.2): picks scope + env, runs the plan
	// via RunnerService.Start and follows progress live through the
	// `runner-step` / `runner-done` Wails events. Completed steps expand into
	// the same ResponsePanel the request editor uses.
	import { onMount } from 'svelte';
	import { untrack } from 'svelte';
	import { Events } from '@wailsio/runtime';
	import { RunnerService } from '../../../bindings/github.com/friesipayung/rapi/index.js';
	import { app } from '#lib/state.svelte';
	import EnvPicker from './EnvPicker.svelte';
	import ResponsePanel from './ResponsePanel.svelte';
	import Icon from './Icon.svelte';
	import type { RunResult, TreeNode } from '#lib/types';

	// Event payload shapes (mirror internal/runner JSON — Step fields keep
	// their Go names because they carry no json tags).
	interface RunnerStep {
		RequestID?: string;
		Name?: string;
		result?: RunResult | null;
		skipped?: boolean;
		durationMs?: number;
	}
	interface RunnerStepEvent {
		runID: string;
		step: RunnerStep;
	}
	interface RunnerReport {
		totalMs: number;
		steps: RunnerStep[];
		failCount: number;
	}
	interface RunnerDoneEvent {
		runID: string;
		report: RunnerReport | null;
	}

	let { scope, onclose }: { scope: TreeNode; onclose: () => void } = $props();

	type StepStatus = 'pending' | 'ok' | 'fail' | 'skip';
	interface StepRow {
		requestID: string;
		name: string;
		status: StepStatus;
		result: RunResult | null;
		skipped: boolean;
		durationMs: number;
	}

	let steps = $state<StepRow[]>([]);
	let running = $state(false);
	let stopOnFailure = $state(false);
	let report = $state<RunnerReport | null>(null);
	let error = $state('');
	let openID = $state('');

	const collectionID = $derived(scope.id.split('/')[0]);
	// while running, the next uncompleted step is the one in flight
	const runningIdx = $derived(running ? steps.findIndex((s) => s.status === 'pending') : -1);
	const passed = $derived(steps.filter((s) => s.status === 'ok').length);
	const failed = $derived(steps.filter((s) => s.status === 'fail').length);
	const skipped = $derived(steps.filter((s) => s.status === 'skip').length);
	const totalSec = $derived(report ? (report.totalMs / 1000).toFixed(1) + 's' : '');

	// Client-side mirror of runner.PlanFromTree so the whole step list (with
	// per-step spinners) is visible before the first event arrives. The
	// server-side plan stays authoritative: events upsert by request id and
	// the final report replaces the rows.
	function planFromTree(nodes: TreeNode[], scopeID: string): { requestID: string; name: string }[] {
		const out: { requestID: string; name: string }[] = [];
		const walk = (n: TreeNode) => {
			if (n.type === 'http' && (n.id === scopeID || n.id.startsWith(scopeID + '/'))) {
				out.push({ requestID: n.id, name: n.name });
			}
			for (const c of n.children ?? []) walk(c);
		};
		for (const n of nodes) walk(n);
		return out;
	}

	function seedRows(scopeID: string): StepRow[] {
		return planFromTree(app.collections, scopeID).map((p) => ({
			requestID: p.requestID,
			name: p.name,
			status: 'pending' as StepStatus,
			result: null,
			skipped: false,
			durationMs: 0
		}));
	}

	// Re-seed when the drawer opens for a (different) node; never touch a
	// live run, and keep finished results when the same scope is re-shown.
	let openedFor = '';
	$effect(() => {
		const id = scope.id;
		if (running || id === openedFor) return;
		openedFor = id;
		untrack(() => {
			steps = seedRows(id);
			report = null;
			error = '';
			openID = '';
		});
	});

	// runID of the active run (plain var: events filter on it, no rendering)
	let activeRunID = '';
	// Events may arrive before the Start call resolves its runID; buffer them.
	let earlyEvents: { name: string; data: unknown }[] = [];

	function deliver(name: string, data: unknown) {
		const p = data as RunnerStepEvent & RunnerDoneEvent;
		if (!p || typeof p.runID !== 'string') return;
		if (activeRunID) {
			if (p.runID !== activeRunID) return; // stale run
			if (name === 'runner-step') applyStep(p.step);
			else finishRun((p as RunnerDoneEvent).report);
		} else if (running) {
			earlyEvents.push({ name, data });
		}
	}

	// stepFailed mirrors internal/runner's failure definition (status >= 400,
	// transport/script error, failed assertion; skipped never fails).
	function stepStatus(sr: RunnerStep): StepStatus {
		if (sr.skipped) return 'skip';
		const r = sr.result;
		if (!r) return 'fail';
		if (r.error || r.scriptError || r.status >= 400) return 'fail';
		if ((r.assertions ?? []).some((a) => !a.pass)) return 'fail';
		return 'ok';
	}

	function applyStep(sr: RunnerStep) {
		const id = sr.RequestID ?? '';
		const row = steps.find((s) => s.requestID === id);
		if (row) {
			row.result = sr.result ?? null;
			row.skipped = !!sr.skipped;
			row.durationMs = sr.durationMs ?? 0;
			row.status = stepStatus(sr);
		} else {
			// unplanned id (tree changed since seeding): append the row
			steps.push({
				requestID: id,
				name: sr.Name ?? id,
				status: stepStatus(sr),
				result: sr.result ?? null,
				skipped: !!sr.skipped,
				durationMs: sr.durationMs ?? 0
			});
		}
	}

	function finishRun(rep: RunnerReport | null) {
		running = false;
		report = rep;
		if (rep?.steps?.length) {
			// final sync from the authoritative report
			steps = rep.steps.map((sr) => ({
				requestID: sr.RequestID ?? '',
				name: sr.Name ?? '',
				status: stepStatus(sr),
				result: sr.result ?? null,
				skipped: !!sr.skipped,
				durationMs: sr.durationMs ?? 0
			}));
		}
	}

	async function startRun() {
		if (running) return;
		error = '';
		report = null;
		openID = '';
		activeRunID = '';
		earlyEvents = [];
		steps = seedRows(scope.id);
		running = true;
		try {
			// envName '' = each step uses the collection's active env — the
			// same state the EnvPicker in this header reads and writes.
			const runID = String(await RunnerService.Start(scope.id, '', stopOnFailure));
			activeRunID = runID;
			for (const q of earlyEvents.splice(0)) deliver(q.name, q.data);
		} catch (e) {
			running = false;
			earlyEvents = [];
			error = String(e);
		}
	}

	function toggleStep(st: StepRow) {
		if (!st.result) return;
		openID = openID === st.requestID ? '' : st.requestID;
	}

	onMount(() => {
		const offStep = Events.On('runner-step', (e) => deliver('runner-step', e.data));
		const offDone = Events.On('runner-done', (e) => deliver('runner-done', e.data));
		return () => {
			offStep();
			offDone();
		};
	});
</script>

<div class="drawer">
	<div class="drawerhead">
		<span class="title">Run: {scope.name}</span>
		<button class="iconbtn" onclick={onclose} title="Close"><Icon name="x" size={14} /></button>
	</div>

	<div class="ctrls">
		<EnvPicker collectionID={collectionID} />
		<label class="sof" title="Skip all remaining steps after the first failed one">
			<input type="checkbox" bind:checked={stopOnFailure} /> Stop on failure
		</label>
		<button class="btn primary" onclick={startRun} disabled={running || steps.length === 0}>
			<Icon name="send" size={13} /> {running ? 'Running…' : 'Run'}
		</button>
	</div>

	{#if error}<p class="error-text">{error}</p>{/if}

	<div class="steplist">
		{#each steps as st, i (st.requestID)}
			<div class="step">
				<button
					class="steprow"
					class:openrow={openID === st.requestID}
					class:idle={st.status === 'pending'}
					onclick={() => toggleStep(st)}
					disabled={!st.result}
					title={st.result ? 'Show response' : st.status === 'pending' ? 'Waiting…' : 'No response'}
				>
					<span class="sticon">
						{#if i === runningIdx}
							<span class="spin"></span>
						{:else if st.status === 'ok'}<span class="ok">✓</span>
						{:else if st.status === 'fail'}<span class="fail">✕</span>
						{:else if st.status === 'skip'}<span class="skip">→</span>
						{:else}<span class="pend"></span>{/if}
					</span>
					{#if st.result?.request?.method}
						<span class="method m-{st.result.request.method}">{st.result.request.method}</span>
					{/if}
					<span class="stname">{st.name}</span>
					{#if st.result?.error}
						<span class="stcode err">err</span>
					{:else if st.result}
						<span class="stcode {st.result.status >= 400 ? 'err' : 'good'}">{st.result.status}</span>
					{/if}
					{#if st.durationMs}<span class="stms">{st.durationMs}ms</span>{/if}
				</button>
				{#if openID === st.requestID && st.result}
					<div class="stepresp">
						<ResponsePanel result={st.result} />
					</div>
				{/if}
			</div>
		{:else}
			<p class="empty">No requests in this scope.</p>
		{/each}
	</div>

	<div class="runfoot">
		<span>{passed} passed · {failed} failed · {skipped} skipped</span>
		<span class="foottime">{running ? '…' : totalSec}</span>
	</div>
</div>

<style>
	/* same container as the git-log drawer in +page.svelte (scoped copy) */
	.drawer {
		position: fixed;
		right: 16px;
		top: 58px;
		width: min(460px, calc(100vw - 32px));
		max-height: 75vh;
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
	}
	.title {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.ctrls {
		display: flex;
		align-items: center;
		gap: 10px;
		flex: none;
		margin-bottom: 8px;
	}
	.ctrls :global(.picker) {
		flex: 1;
		min-width: 0;
	}
	.ctrls :global(.picker select) {
		max-width: none;
		flex: 1;
		min-width: 0;
	}
	.sof {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font-size: 11.5px;
		color: var(--muted);
		white-space: nowrap;
		flex: none;
		cursor: pointer;
	}
	.steplist {
		flex: 1;
		min-height: 0;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
	}
	.step {
		flex: none;
	}
	.steprow {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		min-width: 0;
		padding: 5px 6px;
		border: none;
		background: none;
		color: var(--text);
		font: inherit;
		text-align: left;
		border-radius: var(--radius-ctl);
		cursor: pointer;
	}
	.steprow:hover:not(:disabled) {
		background: var(--surface-2);
	}
	.steprow.openrow {
		background: var(--accent-soft);
	}
	.steprow:disabled {
		cursor: default;
		opacity: 1;
		color: var(--muted);
	}
	.sticon {
		flex: none;
		width: 14px;
		display: inline-flex;
		justify-content: center;
		font-weight: 700;
	}
	.sticon .ok {
		color: var(--success);
	}
	.sticon .fail {
		color: var(--danger);
	}
	.sticon .skip {
		color: var(--muted);
	}
	.pend {
		width: 9px;
		height: 9px;
		border-radius: 50%;
		border: 1.5px solid var(--border-strong);
	}
	.spin {
		width: 11px;
		height: 11px;
		border: 2px solid var(--border-strong);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	/* method chip — same family as the tree rows in Tree.svelte */
	.method {
		flex: none;
		font: 600 10px/1 var(--font-mono);
		padding: 3px 5px;
		border-radius: 4px;
		min-width: 38px;
		text-align: center;
	}
	.m-GET {
		color: var(--success);
		background: rgba(26, 127, 55, 0.12);
	}
	.m-POST {
		color: var(--warning);
		background: rgba(154, 103, 0, 0.12);
	}
	.m-PUT {
		color: #0b5bb5;
		background: rgba(11, 91, 181, 0.1);
	}
	.m-DELETE {
		color: var(--danger);
		background: rgba(209, 36, 47, 0.1);
	}
	.m-PATCH {
		color: #6929c4;
		background: rgba(105, 41, 196, 0.1);
	}
	.stname {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.steprow.idle .stname {
		color: var(--muted);
	}
	.stcode {
		flex: none;
		font: 700 11px var(--font-mono);
		padding: 2px 6px;
		border-radius: 5px;
	}
	.stcode.good {
		color: var(--success);
		background: rgba(26, 127, 55, 0.12);
	}
	.stcode.err {
		color: var(--danger);
		background: var(--danger-soft);
	}
	.stms {
		flex: none;
		font: 11px var(--font-mono);
		color: var(--muted);
	}
	.stepresp {
		margin: 0 0 6px 20px;
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		max-height: 320px;
		overflow: auto;
	}
	.empty {
		color: var(--muted);
		font-size: 12.5px;
		padding: 8px;
	}
	.runfoot {
		flex: none;
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 8px;
		border-top: 1px solid var(--border);
		margin-top: 8px;
		padding-top: 8px;
		font: 500 12px var(--font-mono);
		color: var(--text);
	}
	.foottime {
		color: var(--muted);
	}
</style>
