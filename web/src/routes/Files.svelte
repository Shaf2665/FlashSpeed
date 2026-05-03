<script>
  import { onMount } from 'svelte'
  import { token, currentDriveId, currentParentId, isLoggedIn } from '../lib/stores.js'
  import { api } from '../lib/api.js'
  import { navigate } from 'svelte-routing'

  let drives = []
  let entries = []
  let loading = true
  let error = ''
  let newFolderName = ''
  let showNewFolder = false

  $: if ($currentDriveId) loadFiles()

  onMount(async () => {
    if (!$isLoggedIn) { navigate('/login', { replace: true }); return }
    try {
      drives = await api.listDrives()
      if (drives.length > 0 && !$currentDriveId) {
        currentDriveId.set(drives[0].id)
      }
    } catch (e) { error = e.message }
    loading = false
  })

  async function loadFiles() {
    if (!$currentDriveId) return
    try {
      entries = await api.listFiles($currentDriveId, $currentParentId)
    } catch (e) { error = e.message }
  }

  async function createFolder() {
    if (!newFolderName.trim()) return
    try {
      await api.mkdir($currentDriveId, $currentParentId, newFolderName.trim())
      newFolderName = ''
      showNewFolder = false
      await loadFiles()
    } catch (e) { error = e.message }
  }

  async function deleteEntry(id) {
    if (!confirm('Move to trash?')) return
    try {
      await api.deleteFile(id)
      await loadFiles()
    } catch (e) { error = e.message }
  }

  function handleUpload(e) {
    const file = e.target.files[0]
    if (!file) return
    uploadTUS(file)
  }

  async function uploadTUS(file) {
    const createRes = await fetch('/api/tus/', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${$token}`,
        'Upload-Length': String(file.size),
        'Upload-Metadata': `filename ${btoa(file.name)},drive_id ${btoa(String($currentDriveId))}`,
        'Tus-Resumable': '1.0.0',
      }
    })
    if (!createRes.ok) { error = 'Upload create failed'; return }
    const location = createRes.headers.get('Location')

    const patchRes = await fetch(location, {
      method: 'PATCH',
      headers: {
        'Authorization': `Bearer ${$token}`,
        'Content-Type': 'application/offset+octet-stream',
        'Upload-Offset': '0',
        'Tus-Resumable': '1.0.0',
      },
      body: file,
    })
    if (!patchRes.ok) { error = 'Upload failed'; return }
    await loadFiles()
  }

  function logout() {
    token.set(null)
    navigate('/login', { replace: true })
  }

  function formatBytes(b) {
    if (!b) return '0 B'
    const units = ['B','KB','MB','GB','TB']
    let i = 0
    while (b >= 1024 && i < units.length - 1) { b /= 1024; i++ }
    return b.toFixed(1) + ' ' + units[i]
  }

  function formatDate(d) {
    if (!d) return ''
    return new Date(d).toLocaleDateString()
  }
</script>

<style>
  * { box-sizing: border-box; }
  :global(body) { margin: 0; background: #0f172a; color: #e2e8f0; font-family: monospace; }
  nav { background: #1e293b; border-bottom: 1px solid #334155; padding: 12px 20px;
        display: flex; align-items: center; gap: 16px; }
  nav h1 { color: #38bdf8; margin: 0; font-size: 16px; flex: 1; }
  select { background: #0f172a; color: #e2e8f0; border: 1px solid #334155;
           padding: 4px 8px; border-radius: 4px; font-family: monospace; }
  .toolbar { padding: 10px 20px; display: flex; gap: 8px; border-bottom: 1px solid #1e293b; }
  button { background: #1e293b; color: #e2e8f0; border: 1px solid #334155; padding: 6px 12px;
           border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 12px; }
  button:hover { background: #334155; }
  table { width: 100%; border-collapse: collapse; }
  th { text-align: left; padding: 8px 20px; color: #64748b; font-size: 11px;
       border-bottom: 1px solid #1e293b; }
  td { padding: 8px 20px; border-bottom: 1px solid #0f172a; font-size: 13px; }
  tr:hover td { background: #1e293b; }
  .icon { margin-right: 6px; }
  .error { color: #f87171; padding: 8px 20px; font-size: 12px; }
  .actions { display: flex; gap: 6px; }
  .new-folder { display: flex; gap: 8px; padding: 8px 20px; align-items: center; }
  .new-folder input { background: #0f172a; border: 1px solid #334155; color: #e2e8f0;
                      padding: 5px 8px; border-radius: 4px; font-family: monospace; }
</style>

<nav>
  <h1>⚡ FlashySpeed</h1>
  <select bind:value={$currentDriveId} on:change={() => currentParentId.set(0)}>
    {#each drives as d}
      <option value={d.id}>{d.name}</option>
    {/each}
  </select>
  <button on:click={logout}>Logout</button>
</nav>

{#if error}<div class="error">{error}</div>{/if}

<div class="toolbar">
  <label>
    <button>⬆ Upload</button>
    <input type="file" style="display:none" on:change={handleUpload} />
  </label>
  <button on:click={() => showNewFolder = !showNewFolder}>📁 New Folder</button>
</div>

{#if showNewFolder}
<div class="new-folder">
  <input bind:value={newFolderName} placeholder="Folder name" on:keydown={e => e.key==='Enter' && createFolder()} />
  <button on:click={createFolder}>Create</button>
  <button on:click={() => showNewFolder = false}>Cancel</button>
</div>
{/if}

{#if loading}
  <p style="padding:20px;color:#64748b">Loading...</p>
{:else}
  <table>
    <thead>
      <tr><th>Name</th><th>Size</th><th>Modified</th><th></th></tr>
    </thead>
    <tbody>
      {#each entries as e}
        <tr>
          <td>
            {#if e.is_dir}
              <span class="icon">📁</span>
              <span style="cursor:pointer;color:#38bdf8"
                    on:click={() => { currentParentId.set(e.id); loadFiles() }}>
                {e.name}
              </span>
            {:else}
              <span class="icon">📄</span>{e.name}
            {/if}
          </td>
          <td style="color:#64748b">{e.is_dir ? '—' : formatBytes(e.size_bytes)}</td>
          <td style="color:#64748b">{formatDate(e.updated_at)}</td>
          <td>
            <div class="actions">
              {#if !e.is_dir}
                <a href={api.downloadUrl(e.id)} download={e.name}>
                  <button>⬇</button>
                </a>
              {/if}
              <button on:click={() => deleteEntry(e.id)}>🗑</button>
            </div>
          </td>
        </tr>
      {/each}
      {#if entries.length === 0}
        <tr><td colspan="4" style="color:#64748b;text-align:center;padding:40px">Empty folder</td></tr>
      {/if}
    </tbody>
  </table>
{/if}
