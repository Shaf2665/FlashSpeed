<script>
  import { onMount } from 'svelte'
  import { navigate } from 'svelte-routing'
  import { isLoggedIn, token } from '../lib/stores.js'
  import { api } from '../lib/api.js'

  let loading = true
  let error = ''

  // Users
  let users = []
  let showNewUser = false
  let newUser = { username: '', email: '', password: '', role: 'user', quota_bytes: 0 }
  let newUserError = ''
  let newUserLoading = false

  // Storage
  let storage = null

  // Tailscale
  let tsStatus = null
  let tsAuthKey = ''
  let tsLoading = false
  let tsError = ''
  let tsMsg = ''

  // Edit user modal
  let editUser = null
  let editFields = {}
  let editError = ''
  let editLoading = false

  onMount(async () => {
    if (!$isLoggedIn) { navigate('/login', { replace: true }); return }
    try {
      await Promise.all([loadUsers(), loadStorage(), loadTailscale()])
    } catch (e) { error = e.message }
    loading = false
  })

  async function loadUsers() {
    users = await api.adminListUsers()
  }

  async function loadStorage() {
    storage = await api.adminStorage()
  }

  async function loadTailscale() {
    tsStatus = await api.adminTailscaleStatus()
  }

  async function createUser() {
    if (!newUser.username || !newUser.email || !newUser.password) {
      newUserError = 'Username, email and password are required'
      return
    }
    newUserLoading = true
    newUserError = ''
    try {
      await api.adminCreateUser({ ...newUser, quota_bytes: Number(newUser.quota_bytes) })
      showNewUser = false
      newUser = { username: '', email: '', password: '', role: 'user', quota_bytes: 0 }
      await loadUsers()
    } catch (e) { newUserError = e.message }
    newUserLoading = false
  }

  async function deleteUser(id, username) {
    if (!confirm(`Delete user "${username}"? Their files will remain but the account will be removed.`)) return
    try {
      await api.adminDeleteUser(id)
      await loadUsers()
    } catch (e) { error = e.message }
  }

  function openEditUser(u) {
    editUser = u
    editFields = { role: u.role, quota_bytes: u.quota_bytes, password: '' }
    editError = ''
  }

  async function saveEditUser() {
    editLoading = true
    editError = ''
    try {
      const patch = { role: editFields.role, quota_bytes: Number(editFields.quota_bytes) }
      if (editFields.password) patch.password = editFields.password
      await api.adminUpdateUser(editUser.id, patch)
      editUser = null
      await loadUsers()
    } catch (e) { editError = e.message }
    editLoading = false
  }

  async function installTailscale() {
    tsLoading = true; tsError = ''; tsMsg = ''
    try {
      await api.adminTailscaleInstall()
      tsMsg = 'Tailscale installed. Reload status to confirm.'
      await loadTailscale()
    } catch (e) { tsError = e.message }
    tsLoading = false
  }

  async function connectTailscale() {
    if (!tsAuthKey.trim()) { tsError = 'Auth key required'; return }
    tsLoading = true; tsError = ''; tsMsg = ''
    try {
      await api.adminTailscaleUp(tsAuthKey)
      tsMsg = 'Tailscale connected!'
      tsAuthKey = ''
      await loadTailscale()
    } catch (e) { tsError = e.message }
    tsLoading = false
  }

  function formatBytes(b) {
    if (!b) return '0 B'
    const units = ['B','KB','MB','GB','TB']
    let i = 0
    while (b >= 1024 && i < units.length - 1) { b /= 1024; i++ }
    return b.toFixed(1) + ' ' + units[i]
  }

  function quotaPct(used, quota) {
    if (!quota) return 0
    return Math.min(100, Math.round((used / quota) * 100))
  }
</script>

<style>
  * { box-sizing: border-box; }
  :global(body) { margin: 0; background: #0f172a; color: #e2e8f0; font-family: monospace; }
  nav { background: #1e293b; border-bottom: 1px solid #334155; padding: 12px 20px;
        display: flex; align-items: center; gap: 16px; }
  nav h1 { color: #38bdf8; margin: 0; font-size: 16px; flex: 1; display: flex; align-items: center; gap: 8px; }
  .nav-logo { width: 32px; height: 32px; border-radius: 6px; }
  button { background: #1e293b; color: #e2e8f0; border: 1px solid #334155; padding: 6px 12px;
           border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 12px; }
  button:hover { background: #334155; }
  button.danger { border-color: #f87171; color: #f87171; }
  button.danger:hover { background: #7f1d1d; }
  button.primary { background: #0369a1; border-color: #0369a1; color: #fff; }
  button.primary:hover { background: #0284c7; }
  section { padding: 20px; border-bottom: 1px solid #1e293b; }
  h2 { color: #38bdf8; font-size: 14px; margin: 0 0 14px; }
  table { width: 100%; border-collapse: collapse; }
  th { text-align: left; padding: 6px 12px; color: #64748b; font-size: 11px;
       border-bottom: 1px solid #1e293b; }
  td { padding: 6px 12px; font-size: 12px; border-bottom: 1px solid #0f172a; }
  tr:hover td { background: #1e293b; }
  .error { color: #f87171; font-size: 12px; margin: 6px 0; }
  .success { color: #4ade80; font-size: 12px; margin: 6px 0; }
  input, select { background: #0f172a; border: 1px solid #334155; color: #e2e8f0;
                  padding: 5px 8px; border-radius: 4px; font-family: monospace; font-size: 12px; }
  .form-row { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; margin-top: 10px; }
  .form-row input, .form-row select { flex: 1; min-width: 120px; }
  .actions { display: flex; gap: 6px; }
  .bar-bg { background: #334155; border-radius: 3px; height: 8px; flex: 1; overflow: hidden; }
  .bar-fill { height: 100%; border-radius: 3px; background: #38bdf8; transition: width 0.3s; }
  .bar-fill.warn { background: #f59e0b; }
  .bar-fill.danger { background: #f87171; }
  .bar-row { display: flex; align-items: center; gap: 8px; font-size: 11px; color: #64748b; }
  .ts-status { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: #334155; }
  .dot.on { background: #4ade80; }
  /* Edit modal */
  .modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,0.7);
    display: flex; align-items: center; justify-content: center; z-index: 100; }
  .modal-card { background: #1e293b; border: 1px solid #334155; border-radius: 8px;
    padding: 24px; min-width: 340px; max-width: 480px; width: 90%; position: relative; }
  .modal-card h2 { margin: 0 0 16px; }
  .modal-close { position: absolute; top: 12px; right: 12px; background: none; border: none;
    color: #64748b; font-size: 18px; cursor: pointer; padding: 0 4px; }
  .modal-close:hover { color: #e2e8f0; background: none; }
</style>

<nav>
  <h1><img src="/logo.svg" alt="FlashySpeed logo" class="nav-logo" /> FlashySpeed — Admin</h1>
  <button on:click={() => navigate('/')}>← Files</button>
</nav>

{#if error}<div style="padding:8px 20px" class="error">{error}</div>{/if}
{#if loading}
  <p style="padding:20px;color:#64748b">Loading...</p>
{:else}

<!-- ====== Tailscale Section ====== -->
<section>
  <h2>🔗 Tailscale</h2>
  {#if tsStatus}
    <div class="ts-status">
      <div class="dot" class:on={tsStatus.running}></div>
      <span style="color:#e2e8f0">{tsStatus.running ? 'Connected' : 'Not connected'}</span>
      {#if tsStatus.ip}<span style="color:#64748b">· {tsStatus.ip}</span>{/if}
      {#if tsStatus.version}<span style="color:#64748b">v{tsStatus.version}</span>{/if}
    </div>
  {/if}
  {#if !tsStatus?.running}
    <div style="display:flex;gap:8px;flex-wrap:wrap;margin-bottom:10px">
      <button on:click={installTailscale} disabled={tsLoading}>
        {tsLoading ? '…' : '⬇ Install Tailscale'}
      </button>
    </div>
    <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
      <input bind:value={tsAuthKey} placeholder="tskey-auth-..." style="flex:1;min-width:200px" />
      <button class="primary" on:click={connectTailscale} disabled={tsLoading || !tsAuthKey}>
        {tsLoading ? '…' : 'Connect'}
      </button>
    </div>
  {:else}
    <button on:click={loadTailscale}>↺ Refresh Status</button>
  {/if}
  {#if tsError}<div class="error">{tsError}</div>{/if}
  {#if tsMsg}<div class="success">{tsMsg}</div>{/if}
</section>

<!-- ====== Storage Dashboard ====== -->
{#if storage}
<section>
  <h2>💾 Storage</h2>
  {#if storage.drives.length > 0}
    <table style="margin-bottom:16px">
      <thead><tr><th>Drive</th><th>Mount</th><th>Files</th><th>Used</th></tr></thead>
      <tbody>
        {#each storage.drives as d}
          <tr>
            <td>{d.drive_name}</td>
            <td style="color:#64748b;font-size:11px">{d.mount_path}</td>
            <td>{d.total_files}</td>
            <td>{formatBytes(d.total_bytes)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}

  <h2 style="font-size:13px;margin-bottom:10px">Per-User Quota</h2>
  {#each storage.users as u}
    <div style="margin-bottom:10px">
      <div style="display:flex;justify-content:space-between;font-size:12px;margin-bottom:4px">
        <span>{u.username}</span>
        <span style="color:#64748b">
          {formatBytes(u.used_bytes)}{u.quota_bytes ? ' / ' + formatBytes(u.quota_bytes) : ' (unlimited)'}
        </span>
      </div>
      {#if u.quota_bytes}
        <div class="bar-row">
          <div class="bar-bg">
            <div class="bar-fill"
                 class:warn={quotaPct(u.used_bytes, u.quota_bytes) >= 75}
                 class:danger={quotaPct(u.used_bytes, u.quota_bytes) >= 90}
                 style="width:{quotaPct(u.used_bytes, u.quota_bytes)}%">
            </div>
          </div>
          <span>{quotaPct(u.used_bytes, u.quota_bytes)}%</span>
        </div>
      {/if}
    </div>
  {/each}
</section>
{/if}

<!-- ====== User Management ====== -->
<section>
  <h2>👥 Users</h2>
  <table>
    <thead>
      <tr><th>ID</th><th>Username</th><th>Email</th><th>Role</th><th>Quota</th><th></th></tr>
    </thead>
    <tbody>
      {#each users as u}
        <tr>
          <td style="color:#64748b">{u.id}</td>
          <td>{u.username}</td>
          <td style="color:#64748b">{u.email}</td>
          <td>
            <span style="color:{u.role==='admin'?'#f59e0b':'#94a3b8'}">{u.role}</span>
          </td>
          <td style="color:#64748b">{u.quota_bytes ? formatBytes(u.quota_bytes) : 'unlimited'}</td>
          <td>
            <div class="actions">
              <button on:click={() => openEditUser(u)}>✏ Edit</button>
              <button class="danger" on:click={() => deleteUser(u.id, u.username)}>🗑</button>
            </div>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>

  {#if !showNewUser}
    <button style="margin-top:12px" on:click={() => showNewUser = true}>+ New User</button>
  {:else}
    <div style="margin-top:14px;background:#0f172a;border:1px solid #334155;border-radius:6px;padding:16px">
      <div style="font-size:13px;color:#38bdf8;margin-bottom:10px">New User</div>
      <div class="form-row">
        <input bind:value={newUser.username} placeholder="username" />
        <input bind:value={newUser.email} placeholder="email" type="email" />
        <input bind:value={newUser.password} placeholder="password" type="password" />
        <select bind:value={newUser.role}>
          <option value="user">user</option>
          <option value="admin">admin</option>
        </select>
        <input bind:value={newUser.quota_bytes} placeholder="quota bytes (0=unlimited)" type="number" min="0" />
      </div>
      {#if newUserError}<div class="error">{newUserError}</div>{/if}
      <div style="display:flex;gap:8px;margin-top:10px">
        <button class="primary" on:click={createUser} disabled={newUserLoading}>
          {newUserLoading ? 'Creating…' : 'Create'}
        </button>
        <button on:click={() => { showNewUser = false; newUserError = '' }}>Cancel</button>
      </div>
    </div>
  {/if}
</section>

{/if}

<!-- ====== Edit User Modal ====== -->
{#if editUser}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="modal-backdrop" on:click|self={() => editUser = null}>
    <div class="modal-card">
      <button class="modal-close" on:click={() => editUser = null}>✕</button>
      <h2>✏ Edit "{editUser.username}"</h2>
      <div class="form-row" style="flex-direction:column;gap:10px">
        <div>
          <div style="font-size:11px;color:#64748b;margin-bottom:4px">Role</div>
          <select bind:value={editFields.role} style="width:100%">
            <option value="user">user</option>
            <option value="admin">admin</option>
          </select>
        </div>
        <div>
          <div style="font-size:11px;color:#64748b;margin-bottom:4px">Quota (bytes, 0 = unlimited)</div>
          <input bind:value={editFields.quota_bytes} type="number" min="0" style="width:100%" />
        </div>
        <div>
          <div style="font-size:11px;color:#64748b;margin-bottom:4px">New Password (leave blank to keep)</div>
          <input bind:value={editFields.password} type="password" placeholder="new password…" style="width:100%" />
        </div>
      </div>
      {#if editError}<div class="error">{editError}</div>{/if}
      <div style="display:flex;gap:8px;margin-top:14px">
        <button class="primary" on:click={saveEditUser} disabled={editLoading}>
          {editLoading ? 'Saving…' : 'Save'}
        </button>
        <button on:click={() => editUser = null}>Cancel</button>
      </div>
    </div>
  </div>
{/if}
