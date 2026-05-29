import { create } from 'zustand'

// The login/register responses share this shape.
export interface AuthUser {
  user_id: number
  email: string
  plan: string
  role: string
}

interface PersistedAuth {
  token: string
  user: AuthUser
}

const STORAGE_KEY = 'quantsaas.auth'

function loadPersisted(): PersistedAuth | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as PersistedAuth
    if (parsed?.token && parsed?.user) return parsed
    return null
  } catch {
    return null
  }
}

interface AuthState {
  token: string | null
  user: AuthUser | null
  login: (token: string, user: AuthUser) => void
  logout: () => void
}

const initial = loadPersisted()

export const useAuthStore = create<AuthState>((set) => ({
  token: initial?.token ?? null,
  user: initial?.user ?? null,
  login: (token, user) => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ token, user }))
    set({ token, user })
  },
  logout: () => {
    localStorage.removeItem(STORAGE_KEY)
    set({ token: null, user: null })
  },
}))

// Non-reactive accessor for the api layer.
export function getAuthToken(): string | null {
  return useAuthStore.getState().token
}
