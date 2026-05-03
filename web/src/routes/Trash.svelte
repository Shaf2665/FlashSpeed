<script>
  import { onMount } from 'svelte'
  import { navigate } from 'svelte-routing'
  import { token, isLoggedIn } from '../lib/stores.js'

  onMount(() => {
    if (!$isLoggedIn) navigate('/login', { replace: true })
  })

  function logout() {
    token.set(null)
    navigate('/login', { replace: true })
  }
</script>

<style>
  :global(body) { margin: 0; background: #0f172a; color: #e2e8f0; font-family: monospace; }
  nav { background: #1e293b; border-bottom: 1px solid #334155; padding: 12px 20px;
        display: flex; align-items: center; gap: 16px; }
  nav h1 { color: #38bdf8; margin: 0; font-size: 16px; flex: 1; }
  button { background: #1e293b; color: #e2e8f0; border: 1px solid #334155; padding: 6px 12px;
           border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 12px; }
  button:hover { background: #334155; }
  .panel { padding: 24px 20px; max-width: 520px; }
  .panel p { color: #94a3b8; font-size: 13px; line-height: 1.5; margin: 0 0 16px; }
</style>

<nav>
  <h1>⚡ Trash</h1>
  <button on:click={() => navigate('/')}>← Files</button>
  <button on:click={logout}>Logout</button>
</nav>

<div class="panel">
  <p>
    Deleted files are removed from your library here soon. Backend list, restore,
    and permanent delete ship in P2-5 (<code>/api/trash</code>).
  </p>
  <p style="margin:0;color:#64748b;font-size:12px">
    For now, soft-deleted items stay in the database until P2-5 is implemented.
  </p>
</div>
