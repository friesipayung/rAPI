<script lang="ts">
	import { GreetService } from '../../bindings/github.com/friesipayung/rapi';
	import { Events } from '@wailsio/runtime';

	let name = $state('');
	let greeting = $state('');
</script>

<main>
	<h1>r<span>API</span></h1>
	<p>API client with git as the vault. Phase 1 scaffold — vault setup comes next.</p>

	<form onsubmit={(e) => e.preventDefault()}>
		<input bind:value={name} placeholder="Who should be greeted?" />
		<button type="submit" onclick={async () => { greeting = await GreetService.Greet(name); }}>
			Greet
		</button>
	</form>
	{#if greeting}
		<p class="greeting">{greeting}</p>
	{/if}
</main>

<style>
	main {
		max-width: 640px;
		margin: 12vh auto;
		padding: 24px;
	}
	h1 {
		font-size: 32px;
		letter-spacing: 0.5px;
	}
	h1 span {
		color: var(--accent);
	}
	form {
		display: flex;
		gap: 10px;
		margin: 24px 0;
	}
	input {
		flex: 1;
		padding: 10px 12px;
		border: 1px solid var(--border);
		border-radius: var(--radius-ctl);
		background: var(--surface);
		color: var(--text);
	}
	button {
		padding: 10px 18px;
		border: none;
		border-radius: var(--radius-ctl);
		background: var(--accent);
		color: var(--on-accent);
		font-weight: 600;
		cursor: pointer;
	}
	button:hover {
		background: var(--accent-hover);
	}
	.greeting {
		color: var(--success);
		font-family: var(--mono);
	}
</style>
