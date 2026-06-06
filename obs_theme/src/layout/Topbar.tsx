import { useState } from 'react'
import {
  Box,
  TextField,
  InputAdornment,
  Select,
  MenuItem,
  Button,
  Avatar,
  Menu,
  Switch,
  Typography,
  Divider,
  useMediaQuery,
  useTheme,
} from '@mui/material'
import { Activity, Search, RefreshCw, Sun, Moon, User, Settings, Key, BookOpen, MessageCircle, LogOut } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import ThemeToggle from '../components/common/ThemeToggle'
import NotificationBell from '../components/common/NotificationBell'
import { useUIStore } from '../store/uiStore'
import { useAuthStore } from '../store/authStore'
import { notify } from '../lib/toast'

export default function Topbar() {
  const [timeRange, setTimeRange] = useState('15m')
  const [menuAnchor, setMenuAnchor] = useState<null | HTMLElement>(null)
  const theme = useTheme()
  const isMobile = useMediaQuery(theme.breakpoints.down('md'))
  const themeMode = useUIStore((s) => s.themeMode)
  const toggleThemeMode = useUIStore((s) => s.toggleThemeMode)
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()

  return (
    <Box
      sx={{
        height: 48,
        background: '#161b27',
        borderBottom: '1px solid #1f2535',
        px: 2,
        display: 'flex',
        alignItems: 'center',
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        zIndex: 1100,
        gap: 2,
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 'fit-content' }}>
        <Activity size={18} color="#06b6d4" />
        {!isMobile && (
          <Box sx={{ color: '#06b6d4', fontWeight: 600, fontSize: 15 }}>
            obsAdmin
          </Box>
        )}
      </Box>

      {!isMobile && (
        <Box sx={{ flex: 1, display: 'flex', justifyContent: 'center' }}>
          <TextField
            size="small"
            placeholder="Search services, logs, traces..."
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={14} color="#4d566b" />
                  </InputAdornment>
                ),
                endAdornment: (
                  <InputAdornment position="end">
                    <Box
                      sx={{
                        fontSize: 11,
                        color: '#4d566b',
                        border: '1px solid #2a3147',
                        borderRadius: '3px',
                        px: 0.75,
                        py: 0.25,
                        lineHeight: 1,
                      }}
                    >
                      ⌘K
                    </Box>
                  </InputAdornment>
                ),
              },
            }}
            sx={{
              maxWidth: 480,
              width: '100%',
              '& .MuiOutlinedInput-root': {
                height: 32,
                background: '#0f1117',
                fontSize: 13,
                '& fieldset': { borderColor: '#2a3147' },
                '&:hover fieldset': { borderColor: '#3d4663' },
                '&.Mui-focused fieldset': {
                  borderColor: '#06b6d4',
                  boxShadow: '0 0 0 2px rgba(6,182,212,0.12)',
                },
              },
              '& .MuiOutlinedInput-input': {
                height: 20,
                padding: '6px 8px',
                '&::placeholder': { color: '#4d566b', opacity: 1 },
              },
            }}
          />
        </Box>
      )}

      <Box sx={{ flex: isMobile ? 1 : 'none', display: 'flex', alignItems: 'center', gap: 1, justifyContent: 'flex-end' }}>
        {!isMobile && (
          <>
            <Select
              value={timeRange}
              onChange={(e) => setTimeRange(e.target.value)}
              size="small"
              sx={{
                fontSize: 13,
                height: 30,
                '& .MuiOutlinedInput-notchedOutline': { borderColor: '#2a3147' },
                '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: '#3d4663' },
                '& .MuiSelect-select': { py: 0.5 },
              }}
            >
              <MenuItem value="5m">Last 5 minutes</MenuItem>
              <MenuItem value="15m">Last 15 minutes</MenuItem>
              <MenuItem value="1h">Last 1 hour</MenuItem>
              <MenuItem value="6h">Last 6 hours</MenuItem>
              <MenuItem value="24h">Last 24 hours</MenuItem>
              <MenuItem value="7d">Last 7 days</MenuItem>
              <MenuItem value="custom">Custom</MenuItem>
            </Select>

            <Button
              variant="outlined"
              size="small"
              startIcon={<RefreshCw size={14} />}
              sx={{
                fontSize: 13,
                height: 30,
                color: '#06b6d4',
                borderColor: '#06b6d4',
                minWidth: 'fit-content',
                '&:hover': {
                  borderColor: '#22d3ee',
                  background: 'rgba(6,182,212,0.08)',
                },
              }}
            >
              Refresh
            </Button>
          </>
        )}

        <ThemeToggle />

        <NotificationBell />

        <Avatar
          onClick={(e) => setMenuAnchor(e.currentTarget)}
          sx={{ width: 28, height: 28, fontSize: 12, bgcolor: '#1a2540', color: '#06b6d4', cursor: 'pointer' }}
        >{user?.avatar || 'DU'}</Avatar>

        <Menu anchorEl={menuAnchor} open={Boolean(menuAnchor)} onClose={() => setMenuAnchor(null)}
          slotProps={{ paper: { sx: { mt: 0.5, minWidth: 220 } } }}>
          <Box sx={{ px: 2, py: 1.5 }}>
            <Typography variant="body2" sx={{ fontWeight: 600 }}>{user?.name || 'Demo User'}</Typography>
            <Typography variant="caption2" sx={{ color: 'text.secondary' }}>{user?.email || 'demo@obsadmin.io'}</Typography>
          </Box>
          <Divider />
          <MenuItem onClick={() => { toggleThemeMode(); }} sx={{ gap: 1.5 }}>
            {themeMode === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
            <Typography variant="body2" sx={{ flex: 1 }}>Dark mode</Typography>
            <Switch size="small" checked={themeMode === 'dark'} onChange={toggleThemeMode}
              sx={{ '& .MuiSwitch-switchBase.Mui-checked': { color: '#06b6d4' }, '& .MuiSwitch-switchBase.Mui-checked + .MuiSwitch-track': { background: '#06b6d4' } }} />
          </MenuItem>
          <Divider />
          <MenuItem onClick={() => { setMenuAnchor(null); navigate('/profile'); }} sx={{ gap: 1.5 }}><User size={16} /> Profile</MenuItem>
          <MenuItem onClick={() => { setMenuAnchor(null); navigate('/settings'); }} sx={{ gap: 1.5 }}><Settings size={16} /> Settings</MenuItem>
          <MenuItem onClick={() => { setMenuAnchor(null); navigate('/settings'); }} sx={{ gap: 1.5 }}><Key size={16} /> API Keys</MenuItem>
          <Divider />
          <MenuItem sx={{ gap: 1.5 }}><BookOpen size={16} /> Documentation</MenuItem>
          <MenuItem sx={{ gap: 1.5 }}><MessageCircle size={16} /> Community</MenuItem>
          <Divider />
          <MenuItem onClick={() => { setMenuAnchor(null); logout(); navigate('/login'); notify.success('Signed out'); }} sx={{ gap: 1.5, color: '#ef4444' }}><LogOut size={16} /> Sign out</MenuItem>
        </Menu>
      </Box>
    </Box>
  )
}
