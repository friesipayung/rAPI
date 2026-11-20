<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';

	let { children } = $props();

	// macOS uses a hidden-inset titlebar: reserve room for the traffic lights.
	onMount(() => {
		const mac = navigator.userAgent.includes('Macintosh');
		document.documentElement.dataset.platform = mac ? 'mac' : 'other';
	});
</script>

{@render children()}
