<script lang="ts">
	import { onMount } from 'svelte';

	const appName = 'PulseAlpha';
	
	let isAuthenticated = false;
	let username = '';
	let password = '';
	let error = '';

	onMount(() => {
		if (typeof window !== 'undefined') {
			const storedAuth = localStorage.getItem('pulse_alpha_auth');
			if (storedAuth === 'true') {
				isAuthenticated = true;
			}
		}
	});

	function handleLogin() {
		if (username === 'admin' && password === '123456789') {
			isAuthenticated = true;
			error = '';
			if (typeof window !== 'undefined') {
				localStorage.setItem('pulse_alpha_auth', 'true');
			}
		} else {
			error = 'Geçersiz kullanıcı adı veya şifre';
		}
	}
</script>

<svelte:head>
	<title>{appName}</title>
	<meta
		name="description"
		content="PulseAlpha is a real-time market decision-support interface for live assets and AI-backed probability signals."
	/>
</svelte:head>

{#if isAuthenticated}
	<slot />
{:else}
	<div class="login-container">
		<div class="login-box">
			<h2>{appName} Giriş</h2>
			{#if error}
				<div class="error">{error}</div>
			{/if}
			<form on:submit|preventDefault={handleLogin}>
				<div class="form-group">
					<label for="username">Kullanıcı Adı</label>
					<input type="text" id="username" bind:value={username} required />
				</div>
				<div class="form-group">
					<label for="password">Şifre</label>
					<input type="password" id="password" bind:value={password} required />
				</div>
				<button type="submit">Giriş Yap</button>
			</form>
		</div>
	</div>
{/if}

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

	.login-container {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
	}

	.login-box {
		background: white;
		padding: 2.5rem;
		border-radius: 12px;
		box-shadow: 0 10px 25px rgba(0, 0, 0, 0.05);
		width: 100%;
		max-width: 400px;
		text-align: center;
	}

	.login-box h2 {
		margin-top: 0;
		margin-bottom: 1.5rem;
		color: #1f2a37;
	}

	.form-group {
		margin-bottom: 1.25rem;
		text-align: left;
	}

	.form-group label {
		display: block;
		margin-bottom: 0.5rem;
		font-weight: 500;
		color: #4b5563;
	}

	.form-group input {
		width: 100%;
		padding: 0.75rem;
		border: 1px solid #d1d5db;
		border-radius: 6px;
		font-size: 1rem;
		outline: none;
		transition: border-color 0.2s;
	}

	.form-group input:focus {
		border-color: #3b82f6;
	}

	button {
		width: 100%;
		padding: 0.875rem;
		background: #3b82f6;
		color: white;
		border: none;
		border-radius: 6px;
		font-size: 1rem;
		font-weight: 600;
		cursor: pointer;
		transition: background 0.2s;
	}

	button:hover {
		background: #2563eb;
	}

	.error {
		color: #ef4444;
		background: #fee2e2;
		padding: 0.75rem;
		border-radius: 6px;
		margin-bottom: 1.25rem;
		font-size: 0.875rem;
	}
</style>
