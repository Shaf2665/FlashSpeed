<script>
  import { onMount, onDestroy } from 'svelte'
  import { navigate } from 'svelte-routing'
  import { token, currentDriveId, currentParentId, isLoggedIn } from '../lib/stores.js'
  import { api } from '../lib/api.js'

  let drives = []
  let entries = []
  let loading = true
  let error = ''
  let newFolderName = ''
  let showNewFolder = false

  // --- Search state ---
  let searchQuery = ''
  let searchResults = null   // null = not searching, [] = no results, [...] = results
  let searchLoading = false

  // --- Bulk selection state ---
  let selected = new Set()   // Set of entry IDs
  let bulkLoading = false

  // --- Rename state ---
  let renameEntry = null   // entry being renamed
  let renameName = ''
  let renameLoading = false

  // --- Breadcrumb state ---
  // Each element: { id: number|0, name: string }
  let breadcrumbs = [{ id: 0, name: '/' }]

  // --- Share dialog state ---
  let shareEntry = null   // entry being shared
  let shareUrl = ''       // generated share URL after create
  let shareError = ''
  let shareLoading = false
  let shareCopied = false

  // --- Media preview state ---
  let previewEntry = null  // entry being previewed
  let previewBlobUrl = null

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

  onDestroy(() => {
    revokePreviewBlob()
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

  // ---- Search ----

  async function doSearch() {
    if (!searchQuery.trim()) { searchResults = null; return }
    searchLoading = true
    try {
      searchResults = await api.searchFiles(searchQuery.trim())
    } catch (e) { error = e.message }
    searchLoading = false
  }

  function clearSearch() {
    searchQuery = ''
    searchResults = null
  }

  // ---- Rename ----

  function startRename(entry) {
    renameEntry = entry
    renameName = entry.name
  }

  async function commitRename() {
    if (!renameEntry || !renameName.trim() || renameName === renameEntry.name) {
      renameEntry = null; return
    }
    renameLoading = true
    try {
      await api.renameFile(renameEntry.id, renameName.trim())
      renameEntry = null
      await loadFiles()
    } catch (e) { error = e.message; renameEntry = null }
    renameLoading = false
  }

  function cancelRename() { renameEntry = null }

  // ---- Breadcrumbs ----

  function navigateFolder(entry) {
    breadcrumbs = [...breadcrumbs, { id: entry.id, name: entry.name }]
    currentParentId.set(entry.id)
    loadFiles()
    clearSearch()
  }

  function navigateBreadcrumb(index) {
    const crumb = breadcrumbs[index]
    breadcrumbs = breadcrumbs.slice(0, index + 1)
    currentParentId.set(crumb.id)
    loadFiles()
    clearSearch()
  }

  // ---- Bulk selection ----

  function toggleSelect(id) {
    const s = new Set(selected)
    if (s.has(id)) s.delete(id); else s.add(id)
    selected = s
  }

  function toggleSelectAll() {
    const list = searchResults ?? entries
    if (selected.size === list.filter(e => !e.is_dir).length) {
      selected = new Set()
    } else {
      selected = new Set(list.filter(e => !e.is_dir).map(e => e.id))
    }
  }

  async function bulkDeleteSelected() {
    if (!selected.size) return
    if (!confirm(`Move ${selected.size} file(s) to trash?`)) return
    bulkLoading = true
    try {
      await api.bulkDelete([...selected])
      selected = new Set()
      await loadFiles()
    } catch (e) { error = e.message }
    bulkLoading = false
  }

  async function zipDownloadSelected() {
    if (!selected.size) return
    const ids = [...selected]
    const res = await fetch('/api/files/zip', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${$token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ ids }),
    })
    if (!res.ok) { error = 'ZIP download failed'; return }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = 'files.zip'; a.click()
    URL.revokeObjectURL(url)
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

  // ---- Share dialog ----

  function openShareDialog(entry) {
    shareEntry = entry
    shareUrl = ''
    shareError = ''
    shareCopied = false
    shareLoading = false
  }

  function closeShareDialog() {
    shareEntry = null
    shareUrl = ''
    shareError = ''
    shareCopied = false
  }

  function handleShareBackdropClick(e) {
    if (e.target === e.currentTarget) closeShareDialog()
  }

  async function createShare() {
    if (!shareEntry) return
    shareLoading = true
    shareError = ''
    try {
      const share = await api.createShare(shareEntry.id)
      shareUrl = `${window.location.protocol}//${window.location.host}/s/${share.id}`
    } catch (e) {
      shareError = e.message
    } finally {
      shareLoading = false
    }
  }

  async function copyShareUrl() {
    try {
      await navigator.clipboard.writeText(shareUrl)
      shareCopied = true
      setTimeout(() => { shareCopied = false }, 2000)
    } catch (e) {
      shareError = 'Could not copy to clipboard'
    }
  }

  // ---- Media preview ----

  function revokePreviewBlob() {
    if (previewBlobUrl) {
      URL.revokeObjectURL(previewBlobUrl)
      previewBlobUrl = null
    }
  }

  async function openPreview(entry) {
    revokePreviewBlob()
    previewEntry = entry
    const mime = entry.mime_type || ''
    const endpoint = mime.startsWith('image/')
      ? `/api/files/${entry.id}/download`
      : `/api/files/${entry.id}/stream`
    try {
      // NOTE: for large video files this buffers the entire file into memory.
      // A streaming approach via MediaSource/Service Worker would be needed for production use.
      const res = await fetch(endpoint, {
        headers: { 'Authorization': `Bearer ${$token}` }
      })
      if (!res.ok) throw new Error(res.statusText)
      const blob = await res.blob()
      previewBlobUrl = URL.createObjectURL(blob)
    } catch (e) {
      error = `Preview failed: ${e.message}`
      previewEntry = null
    }
  }

  function closePreview() {
    revokePreviewBlob()
    previewEntry = null
  }

  function handlePreviewBackdropClick(e) {
    if (e.target === e.currentTarget) closePreview()
  }

  function isPreviewable(entry) {
    if (entry.is_dir) return false
    const mime = entry.mime_type || ''
    return mime.startsWith('image/') || mime.startsWith('video/') || mime.startsWith('audio/')
  }
</script>

<style>
  * { box-sizing: border-box; }
  :global(body) { margin: 0; background: #0f172a; color: #e2e8f0; font-family: monospace; }
  nav { background: #1e293b; border-bottom: 1px solid #334155; padding: 12px 20px;
        display: flex; align-items: center; gap: 16px; }
  nav h1 { color: #38bdf8; margin: 0; font-size: 16px; flex: 1; display: flex; align-items: center; gap: 8px; }
  .nav-logo { width: 32px; height: 32px; border-radius: 6px; }
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
  .search-bar { display: flex; gap: 8px; padding: 8px 20px; align-items: center;
                border-bottom: 1px solid #1e293b; }
  .search-bar input { flex: 1; background: #0f172a; border: 1px solid #334155; color: #e2e8f0;
                      padding: 5px 8px; border-radius: 4px; font-family: monospace; }
  .bulk-bar { padding: 6px 20px; display: flex; gap: 8px; align-items: center;
              background: #1e293b; border-bottom: 1px solid #334155; font-size: 12px; color: #94a3b8; }
  input[type="checkbox"] { accent-color: #38bdf8; cursor: pointer; }
  .rename-input { background: #0f172a; border: 1px solid #38bdf8; color: #e2e8f0;
                  padding: 2px 6px; border-radius: 3px; font-family: monospace; font-size: 12px;
                  width: 180px; }
  .breadcrumbs { padding: 6px 20px; display: flex; align-items: center; gap: 4px;
                 font-size: 12px; color: #64748b; border-bottom: 1px solid #1e293b;
                 flex-wrap: wrap; }
  .breadcrumbs .crumb { cursor: pointer; color: #38bdf8; }
  .breadcrumbs .crumb:hover { text-decoration: underline; }
  .breadcrumbs .sep { color: #334155; }

  /* Modal shared styles */
  .modal-backdrop {
    position: fixed; inset: 0; background: rgba(0,0,0,0.7);
    display: flex; align-items: center; justify-content: center;
    z-index: 100;
  }
  .modal-card {
    background: #1e293b; border: 1px solid #334155; border-radius: 8px;
    padding: 24px; min-width: 360px; max-width: 520px; width: 90%;
    position: relative;
  }
  .modal-card h2 { margin: 0 0 16px; font-size: 14px; color: #38bdf8; }
  .modal-close {
    position: absolute; top: 12px; right: 12px;
    background: none; border: none; color: #64748b; font-size: 18px;
    cursor: pointer; padding: 0 4px;
  }
  .modal-close:hover { color: #e2e8f0; background: none; }

  /* Share dialog */
  .share-url-row { display: flex; gap: 8px; margin-top: 12px; align-items: center; }
  .share-url-input {
    flex: 1; background: #0f172a; border: 1px solid #334155; color: #e2e8f0;
    padding: 6px 8px; border-radius: 4px; font-family: monospace; font-size: 12px;
  }
  .share-error { color: #f87171; font-size: 12px; margin-top: 8px; }

  /* Preview modal */
  .preview-media { max-width: 100%; max-height: 60vh; display: block; margin: 0 auto; border-radius: 4px; }
  .preview-filename { color: #64748b; font-size: 12px; margin-bottom: 12px; word-break: break-all; }
</style>

<nav>
  <h1><img src="/logo.svg" alt="FlashySpeed logo" class="nav-logo" /> FlashySpeed</h1>
  <select bind:value={$currentDriveId} on:change={() => currentParentId.set(0)}>
    {#each drives as d}
      <option value={d.id}>{d.name}</option>
    {/each}
  </select>
  <button on:click={() => navigate('/trash')}>🗑 Trash</button>
  <button on:click={() => navigate('/admin')}>⚙ Admin</button>
  <button on:click={logout}>Logout</button>
</nav>

{#if error}<div class="error">{error}</div>{/if}

<!-- Search bar -->
<div class="search-bar">
  <input bind:value={searchQuery} placeholder="🔍 Search files…"
         on:keydown={e => e.key === 'Enter' && doSearch()} />
  <button on:click={doSearch} disabled={searchLoading}>{searchLoading ? '…' : 'Search'}</button>
  {#if searchResults !== null}
    <button on:click={clearSearch}>✕ Clear</button>
  {/if}
</div>

<!-- Breadcrumb trail -->
{#if searchResults === null}
  <div class="breadcrumbs">
    {#each breadcrumbs as crumb, i}
      {#if i > 0}<span class="sep">›</span>{/if}
      {#if i < breadcrumbs.length - 1}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <span class="crumb" on:click={() => navigateBreadcrumb(i)}>
          {crumb.id === 0 ? '🏠 Home' : '📁 ' + crumb.name}
        </span>
      {:else}
        <span style="color:#e2e8f0">{crumb.id === 0 ? '🏠 Home' : '📁 ' + crumb.name}</span>
      {/if}
    {/each}
  </div>
{/if}

<div class="toolbar">
  {#if searchResults === null}
    <label>
      <button>⬆ Upload</button>
      <input type="file" style="display:none" on:change={handleUpload} />
    </label>
    <button on:click={() => showNewFolder = !showNewFolder}>📁 New Folder</button>
  {/if}
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

  <!-- Bulk action bar (shown when items selected) -->
  {#if selected.size > 0}
    <div class="bulk-bar">
      <span>{selected.size} selected</span>
      <button on:click={bulkDeleteSelected} disabled={bulkLoading}>🗑 Delete Selected</button>
      <button on:click={zipDownloadSelected} disabled={bulkLoading}>⬇ ZIP Download</button>
      <button on:click={() => selected = new Set()}>✕ Clear</button>
    </div>
  {/if}

  {#if searchResults !== null}
    <!-- Search results -->
    <div style="padding:8px 20px;font-size:12px;color:#64748b">
      {searchResults.length} result(s) for "{searchQuery}"
    </div>
  {/if}

  <table>
    <thead>
      <tr>
        <th style="width:32px">
          <!-- svelte-ignore a11y-click-events-have-key-events -->
          <!-- svelte-ignore a11y-no-static-element-interactions -->
          <input type="checkbox" on:click={toggleSelectAll}
                 checked={selected.size > 0 && selected.size === (searchResults ?? entries).filter(e => !e.is_dir).length} />
        </th>
        <th>Name</th><th>Size</th><th>Modified</th><th></th>
      </tr>
    </thead>
    <tbody>
      {#each (searchResults ?? entries) as e}
        <tr>
          <td>
            {#if !e.is_dir}
              <input type="checkbox" checked={selected.has(e.id)}
                     on:change={() => toggleSelect(e.id)} />
            {/if}
          </td>
          <td>
            {#if e.is_dir}
              <span class="icon">📁</span>
              <!-- svelte-ignore a11y-click-events-have-key-events -->
              <!-- svelte-ignore a11y-no-static-element-interactions -->
              <span style="cursor:pointer;color:#38bdf8" on:click={() => navigateFolder(e)}>
                {e.name}
              </span>
            {:else}
              <span class="icon">📄</span>
              {#if renameEntry && renameEntry.id === e.id}
                <input class="rename-input" bind:value={renameName}
                       on:keydown={ev => ev.key === 'Enter' && commitRename()}
                       on:blur={cancelRename} />
              {:else}
                {e.name}
              {/if}
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
                {#if isPreviewable(e)}
                  <button on:click={() => openPreview(e)}>▶ Preview</button>
                {/if}
                <button on:click={() => openShareDialog(e)}>🔗 Share</button>
                {#if renameEntry && renameEntry.id === e.id}
                  <button on:click={commitRename} disabled={renameLoading}>✓</button>
                  <button on:click={cancelRename}>✕</button>
                {:else}
                  <button on:click={() => startRename(e)}>✏</button>
                {/if}
              {/if}
              <button on:click={() => deleteEntry(e.id)}>🗑</button>
            </div>
          </td>
        </tr>
      {/each}
      {#if (searchResults ?? entries).length === 0}
        <tr>
          <td colspan="5" style="color:#64748b;text-align:center;padding:40px">
            {searchResults !== null ? 'No results found' : 'Empty folder'}
          </td>
        </tr>
      {/if}
    </tbody>
  </table>
{/if}

<!-- ====== Share Dialog ====== -->
{#if shareEntry}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="modal-backdrop" on:click={handleShareBackdropClick}>
    <div class="modal-card">
      <button class="modal-close" on:click={closeShareDialog}>✕</button>
      <h2>🔗 Share "{shareEntry.name}"</h2>

      {#if !shareUrl}
        <p style="color:#94a3b8;font-size:13px;margin:0 0 16px">
          Create a public link for this file. Anyone with the link can download it.
        </p>
        <button on:click={createShare} disabled={shareLoading}>
          {shareLoading ? 'Creating…' : 'Create Share Link'}
        </button>
      {:else}
        <p style="color:#94a3b8;font-size:12px;margin:0 0 4px">Share link created:</p>
        <div class="share-url-row">
          <input class="share-url-input" readonly value={shareUrl} />
          <button on:click={copyShareUrl}>{shareCopied ? '✓ Copied' : 'Copy'}</button>
        </div>
      {/if}

      {#if shareError}
        <div class="share-error">{shareError}</div>
      {/if}
    </div>
  </div>
{/if}

<!-- ====== Media Preview Modal ====== -->
{#if previewEntry}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="modal-backdrop" on:click={handlePreviewBackdropClick}>
    <div class="modal-card" style="max-width:720px">
      <button class="modal-close" on:click={closePreview}>✕</button>
      <h2>▶ Preview</h2>
      <div class="preview-filename">{previewEntry.name}</div>

      {#if previewBlobUrl}
        {#if (previewEntry.mime_type || '').startsWith('image/')}
          <img class="preview-media" src={previewBlobUrl} alt={previewEntry.name} />
        {:else if (previewEntry.mime_type || '').startsWith('video/')}
          <!-- svelte-ignore a11y-media-has-caption -->
          <video class="preview-media" src={previewBlobUrl} controls></video>
        {:else if (previewEntry.mime_type || '').startsWith('audio/')}
          <audio src={previewBlobUrl} controls style="width:100%;margin-top:8px"></audio>
        {/if}
      {:else}
        <p style="color:#64748b;text-align:center;padding:20px">Loading preview…</p>
      {/if}
    </div>
  </div>
{/if}
