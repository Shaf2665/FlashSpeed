import { writable, derived } from 'svelte/store'

export const token = writable(localStorage.getItem('fs_token') || null)
export const isLoggedIn = derived(token, $t => !!$t)

token.subscribe(val => {
  if (val) localStorage.setItem('fs_token', val)
  else localStorage.removeItem('fs_token')
})

export const currentDriveId = writable(null)
export const currentParentId = writable(0)
