import { Box, Typography, Card, CardContent, CardHeader, Chip } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { recentAlertsData } from '../mockData'

const severityDotColors: Record<string, string> = {
  critical: '#ef4444',
  warning: '#f59e0b',
  info: '#06b6d4',
}

export default function RecentAlerts() {
  const theme = useTheme()

  return (
    <Card sx={{ height: '100%' }}>
      <CardHeader
        title="Recent Alerts"
        sx={{ borderBottom: `1px solid ${theme.palette.divider}` }}
      />
      <CardContent sx={{ p: 0, '&:last-child': { pb: 0 } }}>
        {recentAlertsData.map((alert) => (
          <Box
            key={alert.id}
            sx={{
              display: 'flex',
              alignItems: 'center',
              px: 2,
              height: 40,
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
                background: severityDotColors[alert.severity],
                mr: 1.5,
                flexShrink: 0,
              }}
            />
            <Typography variant="body2" sx={{ flex: 1, color: theme.palette.text.primary }}>
              {alert.title}
            </Typography>
            <Typography
              variant="caption2"
              sx={{
                fontFamily: theme.typography.mono.fontFamily,
                color: theme.palette.text.secondary,
                mx: 1.5,
                minWidth: 100,
              }}
            >
              {alert.service}
            </Typography>
            <Typography variant="caption2" sx={{ color: theme.palette.text.disabled, mx: 1, minWidth: 56 }}>
              {alert.time}
            </Typography>
            <Chip
              label={alert.status === 'firing' ? 'FIRING' : 'RESOLVED'}
              color={alert.status === 'firing' ? 'error' : 'default'}
              size="small"
            />
          </Box>
        ))}
      </CardContent>
    </Card>
  )
}
