import { Box, Typography, Divider } from '@mui/material'
import { Activity, Check } from 'lucide-react'
import { useTheme } from '@mui/material/styles'

export default function LoginLeftPanel() {
  const theme = useTheme()
  const isDark = theme.palette.mode === 'dark'

  return (
    <Box sx={{ flex: '0 0 45%', bgcolor: isDark ? '#0f1117' : '#1f2937', display: 'flex', flexDirection: 'column', justifyContent: 'center', p: 8, minHeight: '100vh' }}>
      <Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 2 }}>
          <Activity size={28} color="#06b6d4" />
          <Typography sx={{ color: '#06b6d4', fontWeight: 600, fontSize: 22 }}>obsAdmin</Typography>
        </Box>
        <Typography variant="body1" sx={{ color: isDark ? theme.palette.text.secondary : '#d1d5db', mb: 4, lineHeight: 1.6, maxWidth: 380 }}>
          Open-source observability platform for modern engineering teams.
        </Typography>
        <Divider sx={{ borderColor: isDark ? '#1f2535' : '#374151', mb: 3 }} />
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, mb: 4 }}>
          {[
            'Unified logs, metrics & traces',
            'Real-time alerting & incidents',
            'Distributed tracing & APM',
            'Infrastructure monitoring',
            'Synthetics & uptime checks',
          ].map((feat) => (
            <Box key={feat} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <Check size={16} color="#06b6d4" />
              <Typography variant="body2" sx={{ color: isDark ? theme.palette.text.secondary : '#d1d5db', fontSize: 14 }}>{feat}</Typography>
            </Box>
          ))}
        </Box>
        <Divider sx={{ borderColor: isDark ? '#1f2535' : '#374151', mb: 4 }} />
        <Box sx={{ p: 2, background: isDark ? '#161b27' : '#111827', border: `1px solid ${isDark ? '#1f2535' : '#374151'}`, borderRadius: '4px' }}>
          <Typography variant="caption" sx={{ color: isDark ? '#4d566b' : '#9ca3af', textTransform: 'uppercase', letterSpacing: '0.04em', display: 'block', mb: 1 }}>
            Dashboard Preview
          </Typography>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            {[100, 60, 80, 40, 90, 50].map((w, i) => (
              <Box key={i} sx={{ height: 4, background: i === 0 ? '#06b6d4' : i === 3 ? '#8b5cf6' : isDark ? '#2a3147' : '#374151', borderRadius: 2, width: `${w}%`, opacity: 0.6 }} />
            ))}
          </Box>
        </Box>
        <Box sx={{ mt: 6 }}>
          <Typography variant="caption2" sx={{ color: isDark ? theme.palette.text.disabled : '#9ca3af', fontSize: 11 }}>
            MIT License &bull; v0.1.0 &bull; github.com/obsadmin
          </Typography>
        </Box>
      </Box>
    </Box>
  )
}
