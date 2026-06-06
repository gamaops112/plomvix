import { Box, Typography } from '@mui/material'
import { useTheme } from '@mui/material/styles'

export default function DemoCredentials() {
  const theme = useTheme()
  const isDark = theme.palette.mode === 'dark'

  return (
    <Box
      sx={{
        mt: 3,
        p: 1.5,
        background: isDark ? '#06b6d415' : '#eff6ff',
        border: '1px solid #06b6d440',
        borderRadius: '4px',
      }}
    >
      <Typography variant="caption" sx={{ color: '#06b6d4', textTransform: 'uppercase', letterSpacing: '0.04em', fontWeight: 600, display: 'block', mb: 1 }}>
        🔑 Demo Access
      </Typography>
      <Box sx={{ display: 'flex', gap: 2, mb: 0.5 }}>
        <Box>
          <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>Email</Typography>
          <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, fontSize: 13 }}>
            demo@obsadmin.io
          </Typography>
        </Box>
        <Box>
          <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>Password</Typography>
          <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, fontSize: 13 }}>
            ObsAdmin@demo
          </Typography>
        </Box>
      </Box>
      <Typography variant="caption2" sx={{ color: theme.palette.text.disabled }}>
        Pre-filled above. JWT expires in 24 hours.
      </Typography>
    </Box>
  )
}
