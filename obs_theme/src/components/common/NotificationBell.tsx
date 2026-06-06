import { useState } from 'react'
import { Box, Typography, Popover, IconButton, Badge, Divider } from '@mui/material'
import { Bell } from 'lucide-react'
import { useTheme } from '@mui/material/styles'
import { useNavigate } from 'react-router-dom'

interface Notification {
  id: number
  title: string
  body: string
  time: string
  read: boolean
  severity: 'critical' | 'warning' | 'info'
}

const mockNotifications: Notification[] = [
  { id: 1, title: 'search-service is down', body: 'Error rate exceeded 5% threshold', time: '2m ago', read: false, severity: 'critical' },
  { id: 2, title: 'High latency on user-service', body: 'P95 latency: 891ms', time: '8m ago', read: false, severity: 'warning' },
  { id: 3, title: 'Deployment completed', body: 'auth-service v2.4.1 deployed', time: '22m ago', read: true, severity: 'info' },
  { id: 4, title: 'Auto-scaling triggered', body: 'api-gateway scaled to 5 instances', time: '41m ago', read: true, severity: 'info' },
]

const severityDot: Record<string, string> = { critical: '#ef4444', warning: '#f59e0b', info: '#06b6d4' }

export default function NotificationBell() {
  const theme = useTheme()
  const navigate = useNavigate()
  const [anchor, setAnchor] = useState<HTMLElement | null>(null)
  const [notifs, setNotifs] = useState(mockNotifications)

  const unread = notifs.filter((n) => !n.read).length

  const markAllRead = () => {
    setNotifs((prev) => prev.map((n) => ({ ...n, read: true })))
  }

  return (
    <>
      <IconButton size="small" onClick={(e) => setAnchor(e.currentTarget)} sx={{ color: '#8b93a8' }}>
        <Badge badgeContent={unread} color="error" sx={{ '& .MuiBadge-badge': { fontSize: 10, height: 16, minWidth: 16 } }}>
          <Bell size={18} />
        </Badge>
      </IconButton>

      <Popover
        open={Boolean(anchor)}
        anchorEl={anchor}
        onClose={() => setAnchor(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        slotProps={{ paper: { sx: { width: 360, mt: 0.5 } } }}
      >
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', px: 2, py: 1.5 }}>
          <Typography variant="h4">Notifications</Typography>
          <Typography
            variant="caption2"
            onClick={markAllRead}
            sx={{ color: '#06b6d4', cursor: 'pointer', '&:hover': { textDecoration: 'underline' } }}
          >
            Mark all read
          </Typography>
        </Box>
        <Divider />

        {notifs.map((n) => (
          <Box key={n.id}>
            <Box
              sx={{
                px: 2, py: 1.5, cursor: 'pointer',
                background: n.read ? 'transparent' : '#06b6d408',
                '&:hover': { background: theme.palette.background.hover },
              }}
              onClick={() => {
                setNotifs((prev) => prev.map((x) => x.id === n.id ? { ...x, read: true } : x))
                setAnchor(null)
                navigate('/alerts')
              }}
            >
              <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5 }}>
                <Box sx={{ width: 8, height: 8, borderRadius: '50%', background: severityDot[n.severity], mt: 0.5, flexShrink: 0 }} />
                <Box sx={{ flex: 1 }}>
                  <Typography variant="body2" sx={{ fontWeight: n.read ? 400 : 600 }}>
                    {n.title}
                  </Typography>
                  <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, display: 'block' }}>
                    {n.body}
                  </Typography>
                </Box>
                <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 0.5 }}>
                  <Typography variant="caption2" sx={{ color: theme.palette.text.disabled, fontSize: 10, whiteSpace: 'nowrap' }}>
                    {n.time}
                  </Typography>
                  {!n.read && <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: '#06b6d4' }} />}
                </Box>
              </Box>
            </Box>
            <Divider />
          </Box>
        ))}

        <Box sx={{ px: 2, py: 1.5, textAlign: 'center' }}>
          <Typography
            variant="caption2"
            onClick={() => { setAnchor(null); navigate('/alerts') }}
            sx={{ color: '#06b6d4', cursor: 'pointer', '&:hover': { textDecoration: 'underline' } }}
          >
            View all alerts →
          </Typography>
        </Box>
      </Popover>
    </>
  )
}
