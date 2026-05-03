<script>
  import { token } from '../lib/stores.js'
  import { api } from '../lib/api.js'
  import { navigate } from 'svelte-routing'

  let username = ''
  let password = ''
  let error = ''
  let loading = false

  async function handleSubmit() {
    error = ''
    loading = true
    try {
      const res = await api.login(username, password)
      token.set(res.token)
      navigate('/', { replace: true })
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }
</script>

<style>
  :global(body) { margin: 0; background: #0f172a; color: #e2e8f0; font-family: monospace; }
  .login-wrap { display: flex; align-items: center; justify-content: center; min-height: 100vh; }
  .login-box { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 32px; width: 320px; }
  h1 { color: #38bdf8; margin: 0 0 24px; font-size: 20px; }
  label { display: block; margin-bottom: 4px; color: #94a3b8; font-size: 12px; }
  input { width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155;
          color: #e2e8f0; padding: 8px; border-radius: 4px; font-family: monospace; margin-bottom: 16px; }
  button { width: 100%; background: #38bdf8; color: #0f172a; border: none; padding: 10px;
           border-radius: 4px; font-weight: bold; cursor: pointer; font-family: monospace; }
  button:disabled { opacity: 0.5; cursor: default; }
  .error { color: #f87171; font-size: 12px; margin-bottom: 12px; }
</style>

<div class="login-wrap">
  <div class="login-box">
    <h1>⚡ FlashySpeed</h1>
    {#if error}<div class="error">{error}</div>{/if}
    <form on:submit|preventDefault={handleSubmit}>
      <label>Username</label>
      <input bind:value={username} autocomplete="username" />
      <label>Password</label>
      <input type="password" bind:value={password} autocomplete="current-password" />
      <button disabled={loading}>{loading ? 'Signing in...' : 'Sign In'}</button>
    </form>
  </div>
</div>
