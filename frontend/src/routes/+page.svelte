<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { VaultService } from '../../bindings/github.com/friesipayung/rapi/index.js';
	import { app, listenSyncStatus } from '#lib/state.svelte';
	import type { VaultStatus } from '#lib/types';

	listenSyncStatus();

	onMount(async () => {
		let vault: VaultStatus | null = null;
		try {
			vault = ((await VaultService.Reopen()) ?? null) as unknown as VaultStatus | null;
		} catch {
			vault = null;
		}
		if (vault?.valid) {
			app.vault = vault;
			app.ready = true;
			goto('/workspace', { replaceState: true });
		} else {
			goto('/setup', { replaceState: true });
		}
	});
</script>

<main>
	<p class="muted">Starting rAPI…</p>
</main>

<style>
	main {
		display: grid;
		place-items: center;
		min-height: 80vh;
	}
</style>
