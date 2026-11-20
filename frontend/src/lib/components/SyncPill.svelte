<script lang="ts">
	import type { SyncStatus } from '../types';

	let { sync, clickable = false }: { sync: SyncStatus; clickable?: boolean } = $props();

	const LABELS: Record<string, string> = {
		idle: 'in sync',
		pending: 'unsaved changes',
		syncing: 'syncing…',
		synced: 'synced',
		error: 'sync error',
		conflict: 'conflict',
	};

	const detail = $derived(sync.detail && ['error', 'conflict'].includes(sync.state) ? '' : sync.detail);
</script>

<span class="pill {sync.state}" title={sync.detail ?? ''} role="status">
	<span class="dot"></span>
	{LABELS[sync.state] ?? sync.state}{detail ? ` — ${detail}` : ''}
	{#if clickable}
		<button onclick={(e) => e.stopPropagation()}>details</button>
	{/if}
</span>

<style>
	.pill {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 12px;
		border-radius: 999px;
		font-size: 12px;
		font-weight: 500;
		background: var(--surface-2);
		color: var(--muted);
		border: 1px solid var(--border);
		max-width: 420px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--muted);
		flex: none;
	}
	.pending .dot {
		background: var(--warning);
	}
	.pending {
		color: var(--warning);
	}
	.syncing .dot {
		background: var(--accent);
		animation: pulse 1s infinite;
	}
	.syncing {
		color: var(--accent);
	}
	.synced .dot {
		background: var(--success);
	}
	.synced,
	.idle {
		color: var(--success);
	}
	.error,
	.conflict {
		color: var(--danger);
		background: var(--accent-soft);
	}
	.error .dot,
	.conflict .dot {
		background: var(--danger);
	}
	button {
		border: none;
		background: none;
		color: inherit;
		font: inherit;
		text-decoration: underline;
		cursor: pointer;
	}
	@keyframes pulse {
		50% {
			opacity: 0.4;
		}
	}
</style>
