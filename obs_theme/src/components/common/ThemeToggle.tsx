import { IconButton, Tooltip } from '@mui/material'
import { Sun, Moon } from 'lucide-react'
import { useUIStore } from '../../store/uiStore'

export default function ThemeToggle() {
  const themeMode = useUIStore((s) => s.themeMode)
  const toggleThemeMode = useUIStore((s) => s.toggleThemeMode)

  return (
    <Tooltip title={`Switch to ${themeMode === 'dark' ? 'light' : 'dark'} mode`}>
      <IconButton size="small" onClick={toggleThemeMode} sx={{ color: '#8b93a8' }}>
        {themeMode === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
      </IconButton>
    </Tooltip>
  )
}
