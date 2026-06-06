import { Box, Typography, Grid } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { ArrowUp, ArrowDown, Minus } from 'lucide-react'
import { statTileData } from '../mockData'

export default function StatTiles() {
  const theme = useTheme()

  return (
    <Grid container spacing={2}>
      {statTileData.map((tile) => (
        <Grid key={tile.label} size={{ xs: 6, sm: 'grow' }}>
          <Box
            sx={{
              background: theme.palette.background.paper,
              border: 1,
              borderColor: 'divider',
              borderRadius: '4px',
              px: 2,
              py: 1.5,
              minWidth: 120,
            }}
          >
            <Typography variant="caption" sx={{ color: theme.palette.text.secondary }}>
              {tile.label}
            </Typography>
            <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 0.5, mt: 0.25 }}>
              <Typography variant="metricSm" sx={{ color: theme.palette.text.primary }}>
                {tile.value}
              </Typography>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
                {tile.deltaType === 'positive' && <ArrowUp size={10} color="#10b981" />}
                {tile.deltaType === 'negative' && <ArrowDown size={10} color="#ef4444" />}
                {tile.deltaType === 'neutral' && <Minus size={10} color="#4d566b" />}
                <Typography
                  variant="caption2"
                  sx={{
                    color:
                      tile.deltaType === 'positive' ? '#10b981' :
                      tile.deltaType === 'negative' ? '#ef4444' : '#4d566b',
                  }}
                >
                  {tile.delta}
                </Typography>
              </Box>
            </Box>
          </Box>
        </Grid>
      ))}
    </Grid>
  )
}
