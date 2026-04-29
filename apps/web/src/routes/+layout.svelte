<script lang="ts">
	const appName = 'PulseAlpha';
</script>

<svelte:head>
	<title>{appName}</title>
	<meta
		name="description"
		content="PulseAlpha is a real-time market decision-support interface for live assets and AI-backed probability signals."
	/>
</svelte:head>

<slot />

<style>
	:global(html) {
		color: #1f2a37;
		font-family: "Aptos", "Segoe UI Variable Display", "Trebuchet MS", sans-serif;
		background: #f4f6fb;
	}

	:global(body) {
		margin: 0;
		min-height: 100vh;
		background:
			radial-gradient(circle at top left, rgba(115, 178, 255, 0.14), transparent 22%),
			radial-gradient(circle at right top, rgba(55, 211, 153, 0.1), transparent 20%),
			linear-gradient(180deg, #f8fbff 0%, #f2f5fb 38%, #edf2f8 100%);
	}

	:global(*) {
		box-sizing: border-box;
	}

	:global(a) {
		color: inherit;
		text-decoration: none;
	}
</style>
