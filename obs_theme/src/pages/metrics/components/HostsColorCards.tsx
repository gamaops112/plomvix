import { Box, Typography, Grid } from '@mui/material'
import { alpha, useTheme } from '@mui/material/styles'
import MetricChart from './MetricChart'
import { hostCardsData } from '../mockData'

export default function HostsColorCards() {
  const theme = useTheme()

  return (
    <Grid container spacing={2}>
      {hostCardsData.map((card) => (
        <Grid key={card.label} size={{ xs: 12, sm: 6, md: 12 / 5 }}>
          <Box
            sx={{
              background: `linear-gradient(135deg, ${alpha(card.color, 0.08)} 0%, ${alpha(card.color, 0.03)} 100%)`,
              border: `1px solid ${alpha(card.color, 0.18)}`,
              borderRadius: '4px',
              p: 2,
              height: 100,
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'space-between',
            }}
          >
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
              <Typography variant="caption" sx={{ color: theme.palette.text.secondary }}>
                {card.label}
              </Typography>
              <Box sx={{ width: 60, height: 30 }}>
                <MetricChart data={card.sparkline} color={card.color} height={30} showArea={false} />
              </Box>
            </Box>
            <Typography variant="metricSm" sx={{ color: theme.palette.text.primary }}>
              {card.value}
              <Typography component="span" variant="caption2" sx={{ ml: 0.5 }}>
                {card.unit}
              </Typography>
            </Typography>
          </Box>
        </Grid>
      ))}
    </Grid>
  )
}
