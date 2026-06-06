import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { apiPost } from '../api/client'

export interface User {
  id: string
  username: string
  name: string
  email: string
  role: 'admin' | string
  avatar: string
}

interface AuthState {
  user: User | null
  token: string | null
  isAuthenticated: boolean
  login: (username: string, password: string) => Promise<{ success: boolean; error?: string }>
  loginDemo: () => void
  logout: () => void
}

function toUser(r: { id: string; username: string; role: string }): User {
  return {
    id: r.id,
    username: r.username,
    name: r.username,
    email: '',
    role: r.role,
    avatar: r.username.slice(0, 2).toUpperCase(),
  }
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      token: null,
      isAuthenticated: false,

      login: async (username: string, password: string) => {
        try {
          const data = await apiPost<{
            token: string
            expires_in: number
            user: { id: string; username: string; role: string }
          }>('/auth/login', { username, password })
          set({ user: toUser(data.user), token: data.token, isAuthenticated: true })
          return { success: true }
        } catch (err: unknown) {
          const msg = err instanceof Error ? err.message : 'Login failed'
          return { success: false, error: msg }
        }
      },

      loginDemo: () => {
        // No-op — kept so call sites don't crash during migration
      },

      logout: () => {
        apiPost('/auth/logout').catch(() => { /* ignore */ })
        set({ user: null, token: null, isAuthenticated: false })
      },
    }),
    {
      name: 'obsadmin-auth',
      partialize: (state) => ({
        user: state.user,
        token: state.token,
        isAuthenticated: state.isAuthenticated,
      }),
      onRehydrateStorage: () => (state) => {
        if (state && !state.token) {
          state.isAuthenticated = false
          state.user = null
        }
      },
    },
  ),
)
