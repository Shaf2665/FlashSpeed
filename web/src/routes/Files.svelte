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
  <h1>⚡ FlashySpeed</h1>
  <select bind:value={$currentDriveId} on:change={() => currentParentId.set(0)}>
    {#each drives as d}
      <option value={d.id}>{d.name}</option>
    {/each}
  </select>
  <button on:click={() => navigate('/trash')}>🗑 Trash</button>
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
                {#if isPreviewable(e)}
                  <button on:click={() => openPreview(e)}>▶ Preview</button>
                {/if}
                <button on:click={() => openShareDialog(e)}>🔗 Share</button>
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
