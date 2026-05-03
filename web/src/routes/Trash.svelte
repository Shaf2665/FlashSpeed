<script>
  import { onMount } from 'svelte'
  import { navigate } from 'svelte-routing'
  import { token, isLoggedIn } from '../lib/stores.js'
  import { api } from '../lib/api.js'

  let rows = []
  let loading = true
  let error = ''
  let busy = false

  onMount(async () => {
    if (!$isLoggedIn) {
      navigate('/login', { replace: true })
      return
    }
    await reload()
    loading = false
  })

  async function reload() {
    error = ''
    try {
      rows = await api.listTrash()
    } catch (e) {
      error = e.message || String(e)
    }
  }

  async function restore(id) {
    if (busy) return
    busy = true
    error = ''
    try {
      await api.restoreFile(id)
      await reload()
    } catch (e) {
      error = e.message || String(e)
    } finally {
      busy = false
    }
  }

  async function kill(id, name) {
    if (!confirm(`Permanently delete "${name}"? This cannot be undone.`)) return
    busy = true
    error = ''
    try {
      await api.permanentDelete(id)
      await reload()
    } catch (e) {
      error = e.message || String(e)
    } finally {
      busy = false
    }
  }

  async function emptyTrash() {
    if (rows.length === 0) return
    if (!confirm('Permanently delete everything in Trash?')) return
    busy = true
    error = ''
    try {
      await api.emptyTrash()
      await reload()
    } catch (e) {
      error = e.message || String(e)
    } finally {
      busy = false
    }
  }

  function logout() {
    token.set(null)
    navigate('/login', { replace: true })
  }

  function formatBytes(b) {
    if (!b) return '0 B'
    const units = ['B','KB','MB','GB','TB']
    let n = Number(b), i = 0
    while (n >= 1024 && i < units.length - 1) { n /= 1024; i++ }
    return n.toFixed(1) + ' ' + units[i]
  }

  function deletedLabel(e) {
    if (!e.deleted_at) return '—'
    return new Date(e.deleted_at).toLocaleString()
  }
</script>

<style>
  :global(body) { margin: 0; background: #0f172a; color: #e2e8f0; font-family: monospace; }
  nav { background: #1e293b; border-bottom: 1px solid #334155; padding: 12px 20px;
        display: flex; align-items: center; gap: 16px; flex-wrap: wrap; }
  nav h1 { color: #38bdf8; margin: 0; font-size: 16px; flex: 1; }
  button { background: #1e293b; color: #e2e8f0; border: 1px solid #334155; padding: 6px 12px;
           border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 12px; }
  button:hover:not(:disabled) { background: #334155; }
  button.danger { border-color: #7f1d1d; color: #fca5a5; }
  button.danger:hover:not(:disabled) { background: #450a0a; }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
  .toolbar { padding: 10px 20px; display: flex; gap: 8px; align-items: center; flex-wrap: wrap;
              border-bottom: 1px solid #1e293b; }
  table { width: 100%; border-collapse: collapse; }
  th { text-align: left; padding: 8px 20px; color: #64748b; font-size: 11px;
       border-bottom: 1px solid #1e293b; }
  td { padding: 8px 20px; border-bottom: 1px solid #0f172a; font-size: 13px; }
  tr:hover td { background: #1e293b; }
  .error { color: #f87171; padding: 8px 20px; font-size: 12px; }
  .actions { display: flex; gap: 6px; flex-wrap: wrap; }
</style>

<nav>
  <h1>⚡ Trash</h1>
  <button on:click={() => navigate('/')}>← Files</button>
  <button on:click={logout}>Logout</button>
</nav>

{#if error}<div class="error">{error}</div>{/if}

<div class="toolbar">
  <button on:click={emptyTrash} disabled={busy || rows.length === 0} class="danger">Empty trash</button>
</div>

{#if loading}
  <p style="padding:20px;color:#64748b">Loading…</p>
{:else}
  <table>
    <thead>
      <tr>
        <th>Name</th>
        <th>Size</th>
        <th>Deleted</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      {#each rows as e}
        <tr>
          <td>
            {#if e.is_dir}
              📁 <span>{e.name}</span>
            {:else}
              📄 {e.name}
            {/if}
          </td>
          <td style="color:#64748b">{e.is_dir ? '—' : formatBytes(e.size_bytes)}</td>
          <td style="color:#64748b">{deletedLabel(e)}</td>
          <td>
            <div class="actions">
              <button on:click={() => restore(e.id)} disabled={busy}>Restore</button>
              <button class="danger" on:click={() => kill(e.id, e.name)} disabled={busy}>Delete forever</button>
            </div>
          </td>
        </tr>
      {/each}
      {#if rows.length === 0}
        <tr><td colspan="4" style="color:#64748b;text-align:center;padding:40px">Trash is empty</td></tr>
      {/if}
    </tbody>
  </table>
{/if}
