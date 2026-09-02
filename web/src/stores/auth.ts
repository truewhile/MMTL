import { create } from 'zustand'
import { persist } from 'zustand/middleware'

import type { User } from '../types'
import { refreshToken } from '../api/refresh'

// Single source of truth for the authenticated user + JWT.
// Persisted to localStorage so a page reload does not drop the session.
interface AuthState {
  token: string | null
  refreshToken: string | null
  user: User | null
  tier: string
  setSession: (token: string, refreshToken: string, user: User) => void
  setUser: (user: User) => void
  setToken: (token: string) => void
  setRefreshToken: (refreshToken: string) => void
  logout: () => void
  tokenRefresh: () => Promise<boolean>
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      refreshToken: null,
      user: null,
      tier: 'free',
      setSession: (token, refreshToken, user) => set({ 
        token, 
        refreshToken, 
        user,
        tier: user.tier || 'free'
      }),
      setUser: (user) => set({ user, tier: user.tier || 'free' }),
      setToken: (token) => set({ token }),
      setRefreshToken: (refreshToken) => set({ refreshToken }),
      logout: () => set({ token: null, refreshToken: null, user: null, tier: 'free' }),
      tokenRefresh: async () => {
        const rt = get().refreshToken
        if (!rt) {
          return false
        }
        try {
          const resp = await refreshToken(rt)
          set({ 
            token: resp.token, 
            refreshToken: resp.refresh_token 
          })
          return true
        } catch {
          // Refresh failed, need to logout
          set({ token: null, refreshToken: null, user: null, tier: 'free' })
          return false
        }
      },
    }),
    { 
      name: 'mebox-auth',
      partialize: (state) => ({ 
        token: state.token, 
        refreshToken: state.refreshToken, 
        user: state.user,
        tier: state.tier
      }),
    },
  ),
)

// Helper function to check if user is authenticated
export function isAuthenticated(): boolean {
  return useAuthStore.getState().token !== null
}

// Helper function to check if user is admin
export function isAdmin(): boolean {
  const user = useAuthStore.getState().user
  return user?.role === 'admin'
}

// Helper function to check if user is plus
export function isPlus(): boolean {
  const state = useAuthStore.getState()
  return state.tier === 'plus' || state.user?.role === 'admin'
}

// Helper function to check if user is super user (admin or plus)
export function isSuperUser(): boolean {
  return isAdmin() || isPlus()
}
