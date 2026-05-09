import { get } from 'svelte/store'
import { token } from './stores.js'

const base = '/api'

async function request(method, path, body) {
  const headers = { 'Content-Type': 'application/json' }
  const tok = get(token)
  if (tok) headers['Authorization'] = `Bearer ${tok}`

  const res = await fetch(base + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  if (res.status === 204) return null
  return res.json()
}

export const api = {
  login: (username, password) => request('POST', '/auth/login', { username, password }),
  me: () => request('GET', '/auth/me'),
  logout: () => request('POST', '/auth/logout'),
  listFiles: (driveId, parentId) =>
    request('GET', `/files?drive_id=${driveId}&parent_id=${parentId || 0}`),
  searchFiles: (q) => request('GET', `/files/search?q=${encodeURIComponent(q)}`),
  mkdir: (driveId, parentId, name) =>
    request('POST', '/files/mkdir', { drive_id: driveId, parent_id: parentId, name }),
  deleteFile: (id) => request('DELETE', `/files/${id}`),
  bulkDelete: (ids) => request('DELETE', '/files', { ids }),
  zipDownload: (ids) => `${base}/files/zip`, // POST with body {ids}
  renameFile: (id, name) => request('PATCH', `/files/${id}`, { name }),
  downloadUrl: (id) => `${base}/files/${id}/download`,
  listDrives: () => request('GET', '/drives'),
  createShare: (fileId) => request('POST', '/shares', { file_id: fileId }),
  listShares: () => request('GET', '/shares'),
  deleteShare: (id) => request('DELETE', `/shares/${id}`),
  listTrash: () => request('GET', '/trash'),
  restoreFile: (id) => request('POST', `/trash/${id}/restore`),
  permanentDelete: (id) => request('DELETE', `/trash/${id}`),
  emptyTrash: () => request('DELETE', '/trash'),
  // Admin
  adminListUsers: () => request('GET', '/admin/users'),
  adminCreateUser: (data) => request('POST', '/admin/users', data),
  adminUpdateUser: (id, data) => request('PATCH', `/admin/users/${id}`, data),
  adminDeleteUser: (id) => request('DELETE', `/admin/users/${id}`),
  adminStorage: () => request('GET', '/admin/storage'),
  adminTailscaleStatus: () => request('GET', '/admin/tailscale/status'),
  adminTailscaleInstall: () => request('POST', '/admin/tailscale/install'),
  adminTailscaleUp: (authKey) => request('POST', '/admin/tailscale/up', { auth_key: authKey }),
}
