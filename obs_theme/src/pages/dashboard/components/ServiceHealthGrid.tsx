import { Box, Typography, Card, CardContent, CardHeader } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { serviceHealthData } from '../mockData'

const statusColors: Record<string, string> = {
  healthy: '#10b981',
  degraded: '#f59e0b',
  down: '#ef4444',
}

export default function ServiceHealthGrid() {
  const theme = useTheme()

  return (
    <Card>
      <CardHeader
        title="Service Health"
        sx={{ borderBottom: `1px solid ${theme.palette.divider}` }}
      />
      <CardContent sx={{ p: 0, '&:last-child': { pb: 0 } }}>
        {serviceHealthData.map((svc) => (
          <Box
            key={svc.name}
            sx={{
              display: 'flex',
              alignItems: 'center',
              px: 2,
              height: 32,
              borderBottom: `1px solid ${theme.palette.divider}`,
              '&:hover': { background: theme.palette.background.hover },
              '&:last-child': { borderBottom: 'none' },
            }}
          >
            <Box
              sx={{
                width: 6,
                height: 6,
                borderRadius: '50%',
                background: statusColors[svc.status],
                mr: 1.5,
                flexShrink: 0,
              }}
            />
            <Typography
              variant="body2"
              sx={{
                flex: 1,
                fontFamily: theme.typography.mono.fontFamily,
                color: theme.palette.text.primary,
              }}
            >
              {svc.name}
            </Typography>
            <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, mx: 2 }}>
              {svc.latency}
            </Typography>
            <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, minWidth: 48, textAlign: 'right' }}>
              {svc.uptime}
            </Typography>
          </Box>
        ))}
      </CardContent>
    </Card>
  )
}
