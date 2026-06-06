import { Box, Typography } from '@mui/material'
import { severityColors } from '../mockData'

export default function AlertsSummaryBar() {
  const counts = { critical: 2, high: 3, warning: 5, info: 4, resolved: 12 }

  return (
    <Box sx={{ display: 'flex', gap: 1.5, mb: 2, flexWrap: 'wrap' }}>
      {(Object.entries(counts) as Array<[string, number]>).map(([key, count]) => {
        const color = severityColors[key] || '#4d566b'
        return (
          <Box
            key={key}
            sx={{
              display: 'flex', alignItems: 'center', gap: 1,
              px: 1.5, py: 0.5,
              background: `${color}15`,
              border: `1px solid ${color}40`,
              borderRadius: '4px',
            }}
          >
            <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: color }} />
            <Typography variant="caption2" sx={{ color: '#8b93a8', textTransform: 'capitalize' }}>
              {key}
            </Typography>
            <Typography variant="body2" sx={{ fontWeight: 600, color }}>
              {count}
            </Typography>
          </Box>
        )
      })}
    </Box>
  )
}
