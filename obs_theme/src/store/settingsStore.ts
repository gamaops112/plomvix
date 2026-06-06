import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface SettingsState {
  timezone: string
  dateFormat: string
  defaultTimeRange: string
  defaultEnvironment: string
  rowsPerPage: number
  sendTelemetry: boolean
  density: 'compact' | 'default' | 'comfortable'
  showSectionLabels: boolean
  showIcons: boolean
  updateSettings: (settings: Partial<SettingsState>) => void
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      timezone: 'UTC',
      dateFormat: 'YYYY-MM-DD',
      defaultTimeRange: 'Last 15 minutes',
      defaultEnvironment: 'production',
      rowsPerPage: 25,
      sendTelemetry: false,
      density: 'default' as const,
      showSectionLabels: true,
      showIcons: true,
      updateSettings: (newSettings) => set((state) => ({ ...state, ...newSettings })),
    }),
    { name: 'obsadmin-settings' }
  )
)
