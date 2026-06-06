import { Box, Typography, Grid } from '@mui/material'
import { alpha, useTheme } from '@mui/material/styles'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { ArrowUp, ArrowDown } from 'lucide-react'
import { colorCardData } from '../mockData'

echarts.use([LineChart, GridComponent, CanvasRenderer])

export default function ColorMetricCards() {
  const theme = useTheme()

  return (
    <Grid container spacing={2}>
      {colorCardData.map((card) => (
        <Grid size={{ xs: 12, sm: 6, md: 3 }} key={card.label}>
          <Box
            sx={{
              background: `linear-gradient(135deg, ${alpha(card.color, 0.08)} 0%, ${alpha(card.color, 0.03)} 100%)`,
              border: `1px solid ${alpha(card.color, 0.18)}`,
              borderRadius: '4px',
              p: 2,
              height: '100%',
            }}
          >
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
              <Typography variant="caption" sx={{ color: theme.palette.text.secondary }}>
                {card.label}
              </Typography>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                {card.changeType === 'positive' ? (
                  <ArrowUp size={12} color="#10b981" />
                ) : (
                  <ArrowDown size={12} color={card.changeType === 'negative' ? '#ef4444' : '#4d566b'} />
                )}
                <Typography
                  variant="caption2"
                  sx={{
                    color: card.changeType === 'positive' ? '#10b981' : card.changeType === 'negative' ? '#ef4444' : '#4d566b',
                  }}
                >
                  {card.change}
                </Typography>
              </Box>
            </Box>

            <Box sx={{ height: 48, my: 1 }}>
              <ReactEChartsCore
                echarts={echarts}
                option={{
                  grid: { top: 0, right: 0, bottom: 0, left: 0 },
                  xAxis: { show: false, data: card.sparkline.map((_, i) => i) },
                  yAxis: { show: false },
                  series: [{
                    type: 'line',
                    data: card.sparkline,
                    smooth: true,
                    symbol: 'none',
                    lineStyle: { color: card.color, width: 1.5 },
                    areaStyle: { color: alpha(card.color, 0.08) },
                  }],
                }}
                style={{ height: 48 }}
                notMerge
              />
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
