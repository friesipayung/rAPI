// Global app state (Svelte 5 runes).
import { Events } from '@wailsio/runtime';
import type { SyncStatus, TreeNode, VaultStatus } from './types';

export const app = $state({
	ready: false,
	vault: null as VaultStatus | null,
	collections: [] as TreeNode[],
	sync: { state: 'idle', detail: '' } as SyncStatus,
});

/** Listen once for sync-status events; call from the root layout. */
let listening = false;
export function listenSyncStatus() {
	if (listening) return;
	listening = true;
	Events.On('sync-status', (e) => {
		app.sync = e.data as SyncStatus;
	});
}
