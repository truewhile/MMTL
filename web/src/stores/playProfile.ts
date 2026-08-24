import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

interface PlayProfileState {
  activeProfileId: string | null
  activeProfilePinToken: string | null
  setActiveProfile: (id: string | null, pinToken?: string | null) => void
  setActiveProfileId: (id: string | null) => void
}

export const usePlayProfileStore = create<PlayProfileState>()(
  persist(
    (set) => ({
      activeProfileId: null,
      activeProfilePinToken: null,
      setActiveProfile: (id, pinToken = null) =>
        set({ activeProfileId: id || null, activeProfilePinToken: id ? pinToken || null : null }),
      setActiveProfileId: (id) =>
        set((state) => ({
          activeProfileId: id || null,
          activeProfilePinToken: id ? state.activeProfilePinToken : null,
        })),
    }),
    {
      name: 'mmtl-play-profile',
      storage: createJSONStorage(() => sessionStorage),
    },
  ),
)

export function getActivePlayProfileId(): string | null {
  return usePlayProfileStore.getState().activeProfileId
}

export function getActivePlayProfilePinToken(): string | null {
  return usePlayProfileStore.getState().activeProfilePinToken
}
