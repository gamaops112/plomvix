import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ThemeProvider } from '@mui/material/styles'
import CssBaseline from '@mui/material/CssBaseline'
import { darkTheme, lightTheme } from './theme'
import { useUIStore } from './store/uiStore'
import App from './App'

function ThemedApp() {
  const themeMode = useUIStore((s) => s.themeMode)
  const theme = themeMode === 'dark' ? darkTheme : lightTheme
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <App />
    </ThemeProvider>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemedApp />
  </StrictMode>,
)
