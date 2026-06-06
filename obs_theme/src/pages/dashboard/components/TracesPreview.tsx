import { Box, Typography, Card, CardContent, CardHeader, Chip } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { Link } from 'react-router-dom'
import { tracesPreviewData } from '../mockData'

const statusDotColors: Record<string, string> = {
  ok: '#10b981',
  error: '#ef4444',
  slow: '#f59e0b',
}

export default function TracesPreview() {
  const theme = useTheme()

  return (
    <Card sx={{ height: '100%' }}>
      <CardHeader
        title="Traces Preview"
        sx={{ borderBottom: `1px solid ${theme.palette.divider}` }}
      />
      <CardContent sx={{ p: 0, '&:last-child': { pb: 0 } }}>
        {tracesPreviewData.map((trace) => (
          <Box
            key={trace.traceId}
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
                background: statusDotColors[trace.status],
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
              {trace.operation}
            </Typography>
            <Typography
              variant="caption2"
              sx={{ color: theme.palette.text.secondary, mx: 1.5, minWidth: 90 }}
            >
              {trace.service}
            </Typography>
            <Typography
              variant="body2"
              sx={{
                fontFamily: theme.typography.mono.fontFamily,
                color: parseInt(trace.duration) > 1000 ? '#ef4444' : theme.palette.text.primary,
                mx: 1.5,
                minWidth: 56,
                textAlign: 'right',
              }}
            >
              {trace.duration}
            </Typography>
            <Typography variant="caption2" sx={{ color: theme.palette.text.disabled, mx: 1, minWidth: 56 }}>
              {trace.spans} spans
            </Typography>
            <Chip
              label={trace.status.toUpperCase()}
              color={
                trace.status === 'error' ? 'error' :
                trace.status === 'slow' ? 'warning' : 'success'
              }
              size="small"
            />
          </Box>
        ))}
        <Box sx={{ display: 'flex', justifyContent: 'flex-end', p: 1 }}>
          <Typography
            component={Link}
            to="/traces"
            variant="caption2"
            sx={{ color: theme.palette.primary.main, textDecoration: 'none', '&:hover': { textDecoration: 'underline' } }}
          >
            View all traces →
          </Typography>
        </Box>
      </CardContent>
    </Card>
  )
}
