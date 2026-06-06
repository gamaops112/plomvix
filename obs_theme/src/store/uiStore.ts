import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface UIState {
  sidebarCollapsed: boolean
  toggleSidebar: () => void
  themeMode: 'dark' | 'light'
  toggleThemeMode: () => void
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      toggleSidebar: () =>
        set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),
      themeMode: 'dark',
      toggleThemeMode: () =>
        set((state) => ({ themeMode: state.themeMode === 'dark' ? 'light' : 'dark' })),
    }),
    { name: 'obsadmin-ui-v1' }
  )
)
