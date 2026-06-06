import { useState, useEffect, useCallback, useRef } from 'react'
import { Box, Typography, TextField, Divider, Modal, InputAdornment } from '@mui/material'
import { Search, LayoutDashboard, FileText, GitBranch, Activity } from 'lucide-react'
import { useTheme } from '@mui/material/styles'
import { useNavigate } from 'react-router-dom'

interface SearchResult {
  type: 'page' | 'service'
  label: string
  path?: string
  status?: string
  icon: typeof LayoutDashboard
}

const results: SearchResult[] = [
  { type: 'page', label: 'Dashboard', path: '/', icon: LayoutDashboard },
  { type: 'page', label: 'Logs Explorer', path: '/logs', icon: FileText },
  { type: 'page', label: 'Traces', path: '/traces', icon: GitBranch },
  { type: 'service', label: 'api-gateway', status: 'healthy', icon: Activity },
  { type: 'service', label: 'search-service', status: 'down', icon: Activity },
]

export default function GlobalSearch() {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const navigate = useNavigate()
  const theme = useTheme()

  const filtered = query ? results.filter((r) => r.label.toLowerCase().includes(query.toLowerCase())) : results

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') { e.preventDefault(); setOpen((prev) => !prev); setSelectedIndex(0) }
    if (e.key === 'Escape') setOpen(false)
  }, [])

  useEffect(() => { window.addEventListener('keydown', handleKeyDown); return () => window.removeEventListener('keydown', handleKeyDown) }, [handleKeyDown])
  useEffect(() => { if (open) setTimeout(() => inputRef.current?.focus(), 50) }, [open])

  const handleSelect = (item: SearchResult) => { if (item.path) navigate(item.path); setOpen(false); setQuery('') }

  return (
    <Modal open={open} onClose={() => setOpen(false)} disableAutoFocus>
      <Box sx={{ position: 'absolute', top: '20%', left: '50%', transform: 'translateX(-50%)', width: 560, maxWidth: '90vw', bgcolor: 'background.paper', border: 1, borderColor: 'divider', borderRadius: '6px', boxShadow: '0 20px 60px rgba(0,0,0,0.5)', outline: 'none' }}>
        <TextField
          inputRef={inputRef}
          value={query}
          onChange={(e) => { setQuery(e.target.value); setSelectedIndex(0) }}
          onKeyDown={(e) => {
            if (e.key === 'ArrowDown') { e.preventDefault(); setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1)) }
            if (e.key === 'ArrowUp') { e.preventDefault(); setSelectedIndex((i) => Math.max(i - 1, 0)) }
            if (e.key === 'Enter') { if (filtered[selectedIndex]) handleSelect(filtered[selectedIndex]) }
            if (e.key === 'Escape') setOpen(false)
          }}
          placeholder="Search services, logs, traces..."
          fullWidth variant="standard"
          sx={{ px: 2, py: 1.5, '& .MuiInput-root:before, & .MuiInput-root:after': { display: 'none' } }}
          slotProps={{ input: {
            startAdornment: <InputAdornment position="start"><Search size={18} color={theme.palette.text.disabled} /></InputAdornment>,
            endAdornment: <InputAdornment position="end"><Box sx={{ fontSize: 11, color: 'text.disabled', border: 1, borderColor: 'divider', borderRadius: '3px', px: 0.75, py: 0.25 }}>Esc</Box></InputAdornment>,
          }}}
        />
        {filtered.length > 0 && (
          <>
            <Divider />
            <Box sx={{ py: 1 }}>
              {filtered.map((item, i) => {
                const Icon = item.icon
                return (
                  <Box key={item.label} onClick={() => handleSelect(item)}
                    sx={{
                      display: 'flex', alignItems: 'center', gap: 1.5, px: 2, py: 1, cursor: 'pointer',
                      bgcolor: i === selectedIndex ? 'background.hover' : 'transparent',
                      '&:hover': { bgcolor: 'background.hover' },
                    }}>
                    <Icon size={16} color={theme.palette.text.secondary} />
                    <Typography variant="body2" sx={{ flex: 1 }}>{item.label}</Typography>
                    {item.type === 'page' && <Typography variant="caption2" sx={{ color: 'text.disabled' }}>{item.path}</Typography>}
                    {item.type === 'service' && (
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                        <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: item.status === 'down' ? '#ef4444' : '#10b981' }} />
                        <Typography variant="caption2" sx={{ color: item.status === 'down' ? '#ef4444' : '#10b981' }}>{item.status}</Typography>
                      </Box>
                    )}
                  </Box>
                )
              })}
            </Box>
          </>
        )}
      </Box>
    </Modal>
  )
}
