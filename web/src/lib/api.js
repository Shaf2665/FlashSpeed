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
  mkdir: (driveId, parentId, name) =>
    request('POST', '/files/mkdir', { drive_id: driveId, parent_id: parentId, name }),
  deleteFile: (id) => request('DELETE', `/files/${id}`),
  renameFile: (id, name) => request('PATCH', `/files/${id}`, { name }),
  downloadUrl: (id) => `${base}/files/${id}/download`,
  listDrives: () => request('GET', '/drives'),
  createShare: (fileId) => request('POST', '/shares', { file_id: fileId }),
  listShares: () => request('GET', '/shares'),
  deleteShare: (id) => request('DELETE', `/shares/${id}`),
}
