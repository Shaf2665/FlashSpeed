<script>
  import { onMount } from 'svelte'

  // token is passed as a prop from the route: <Share token={params.token} />
  export let token

  let loading = true
  let submitting = false
  let error = ''        // 404 / 410 / unexpected
  let share = null
  let file = null
  let needsPassword = false
  let password = ''
  let passwordError = ''
  let hasTriedPassword = false

  onMount(() => {
    resolveShare('')
  })

  async function resolveShare(pwd) {
    loading = true
    passwordError = ''
    try {
      const headers = {}
      if (pwd) headers['X-Share-Password'] = pwd

      const res = await fetch('/api/s/' + token, { headers })

      if (res.ok) {
        const data = await res.json()
        share = data.share
        file = data.file
        needsPassword = false
        error = ''
      } else if (res.status === 401) {
        needsPassword = true
        if (hasTriedPassword) {
          passwordError = 'Incorrect password.'
        }
      } else if (res.status === 410) {
        error = 'This share has expired or reached its download limit.'
      } else if (res.status === 404) {
        error = 'Share not found.'
      } else {
        error = 'Unexpected error (' + res.status + ').'
      }
    } catch (e) {
      error = 'Network error: ' + e.message
    } finally {
      loading = false
    }
  }

  async function handlePasswordSubmit() {
    hasTriedPassword = true
    submitting = true
    await resolveShare(password)
    submitting = false
    // clear password on success (when needsPassword becomes false)
    if (!needsPassword) password = ''
  }

  function formatBytes(b) {
    if (!b) return null
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    let i = 0
    while (b >= 1024 && i < units.length - 1) { b /= 1024; i++ }
    return b.toFixed(1) + ' ' + units[i]
  }

  function formatDate(d) {
    if (!d) return null
    return new Date(d).toLocaleString()
  }

  function mimeIcon(mime) {
    if (!mime) return '📄'
    if (mime.startsWith('image/')) return '🖼'
    if (mime.startsWith('video/')) return '🎬'
    if (mime.startsWith('audio/')) return '🎵'
    if (mime.includes('pdf')) return '📕'
    if (mime.includes('zip') || mime.includes('tar') || mime.includes('gz')) return '🗜'
    if (mime.includes('text/')) return '📝'
    return '📄'
  }
</script>

<style>
  :global(body) { margin: 0; background: #0f172a; color: #e2e8f0; font-family: monospace; }
  .wrap { display: flex; align-items: center; justify-content: center; min-height: 100vh; }
  .card { background: #1e293b; border: 1px solid #334155; border-radius: 8px;
          padding: 40px 32px; width: 360px; text-align: center; }
  .brand { color: #38bdf8; font-size: 20px; margin: 0 0 28px; display: flex; align-items: center; justify-content: center; gap: 10px; }
  .brand-logo { width: 40px; height: 40px; border-radius: 8px; }
  .file-icon { font-size: 48px; margin-bottom: 12px; }
  .filename { font-size: 16px; font-weight: bold; word-break: break-all; margin-bottom: 8px; }
  .meta { color: #64748b; font-size: 12px; margin-bottom: 6px; }
  .share-info { color: #94a3b8; font-size: 12px; margin-bottom: 24px; }
  .download-btn {
    display: inline-block; background: #38bdf8; color: #0f172a; border: none;
    padding: 10px 24px; border-radius: 4px; font-weight: bold; cursor: pointer;
    font-family: monospace; font-size: 14px; text-decoration: none;
  }
  .download-btn:hover { background: #7dd3fc; }
  label { display: block; text-align: left; margin-bottom: 4px; color: #94a3b8; font-size: 12px; }
  input { width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155;
          color: #e2e8f0; padding: 8px; border-radius: 4px; font-family: monospace; margin-bottom: 12px; }
  .submit-btn { width: 100%; background: #38bdf8; color: #0f172a; border: none; padding: 10px;
               border-radius: 4px; font-weight: bold; cursor: pointer; font-family: monospace; }
  .submit-btn:disabled { opacity: 0.5; cursor: default; }
  .error-msg { color: #f87171; font-size: 12px; margin-bottom: 12px; }
  .pwd-hint { color: #94a3b8; font-size: 13px; margin-bottom: 20px; }
  .spinner { color: #64748b; }
</style>

<div class="wrap">
  <div class="card">
    <h1 class="brand"><img src="/logo.svg" alt="FlashySpeed logo" class="brand-logo" /> FlashySpeed</h1>

    {#if loading && !needsPassword && !share && !error}
      <p class="spinner">Loading...</p>

    {:else if share}
      <div class="file-icon">{mimeIcon(file.mime_type)}</div>
      <div class="filename">{file.name}</div>

      {#if file.size_bytes}
        <div class="meta">{formatBytes(file.size_bytes)}</div>
      {/if}

      <div class="meta">{file.mime_type || 'Unknown type'}</div>

      <div class="share-info">
        {#if share && share.expires_at}
          Expires: {formatDate(share.expires_at)}
        {:else}
          No expiry
        {/if}
      </div>

      {#if !file.is_dir}
        <a class="download-btn" href="/api/s/{token}/download" download={file.name}>
          ⬇ Download
        </a>
      {:else}
        <div class="meta">Directory sharing — individual download not available.</div>
      {/if}

    {:else if needsPassword}
      <p class="pwd-hint">This share is password-protected.</p>
      {#if passwordError}<p class="error-msg" aria-live="assertive">{passwordError}</p>{/if}
      <form on:submit|preventDefault={handlePasswordSubmit}>
        <label>Password</label>
        <input type="password" bind:value={password} autocomplete="current-password" />
        <button class="submit-btn" disabled={submitting}>
          {submitting ? 'Checking...' : 'Unlock'}
        </button>
      </form>

    {:else if error}
      <p class="error-msg">{error}</p>

    {/if}
  </div>
</div>
