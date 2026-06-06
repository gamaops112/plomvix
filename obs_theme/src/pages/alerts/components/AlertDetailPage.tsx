import { useMemo } from 'react'
import { Box, Typography, Button, Grid, Chip } from '@mui/material'
import { useParams, useNavigate } from 'react-router-dom'
import { useTheme } from '@mui/material/styles'
import { ArrowLeft } from 'lucide-react'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { mockFiringAlerts, severityColors, generateTimeSeries } from '../mockData'
import type { FiringAlert } from '../mockData'

echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])

export default function AlertDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const theme = useTheme()

  const alert: FiringAlert | undefined = mockFiringAlerts.find((a: FiringAlert) => a.id === id)

  if (!alert) {
    return <Box sx={{ p: 3 }}><Typography>Alert not found</Typography></Box>
  }

  const thresholdValue = parseFloat(alert.condition.match(/([\d.]+)%?/)?.at(1) || '5')
  const currentValue = parseFloat(alert.value)

  const chartOpt = useMemo(() => ({
    tooltip: { trigger: 'axis' as const, backgroundColor: theme.palette.background.elevated, borderColor: theme.palette.divider, textStyle: { color: theme.palette.text.primary, fontSize: 12 } },
    grid: { top: 16, right: 16, bottom: 24, left: 48 },
    xAxis: { type: 'category' as const, data: Array.from({ length: 60 }, (_, i) => `${i}m ago`), axisLabel: { color: theme.palette.text.disabled, fontSize: 10 } },
    yAxis: { type: 'value' as const, splitLine: { lineStyle: { color: theme.palette.divider } }, axisLabel: { color: theme.palette.text.disabled, fontSize: 10 } },
    series: [
      { name: 'Value', type: 'line', data: generateTimeSeries(currentValue, currentValue * 0.3), smooth: true, symbol: 'none', lineStyle: { color: severityColors[alert.severity], width: 2 } },
      { name: 'Threshold', type: 'line', data: Array(60).fill(thresholdValue), symbol: 'none', lineStyle: { color: '#ef4444', width: 1, type: 'dashed' as const } },
    ],
  }), [currentValue, thresholdValue, alert.severity])

  return (
    <Box sx={{ p: 3 }}>
      <Button startIcon={<ArrowLeft size={14} />} sx={{ color: theme.palette.text.secondary, fontSize: 13, mb: 2 }}
        onClick={() => navigate('/alerts')}>Back to Alerts</Button>

      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
            <Chip label={alert.severity.toUpperCase()} size="small" sx={{ background: `${severityColors[alert.severity]}20`, color: severityColors[alert.severity], fontWeight: 600, fontSize: 10, height: 20 }} />
            <Typography variant="h2">{alert.name}</Typography>
          </Box>
          <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
            {alert.service} &bull; firing for {alert.duration}
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button variant="outlined" size="small" sx={{ fontSize: 13 }}>Silence</Button>
          <Button variant="outlined" size="small" sx={{ fontSize: 13 }}>Acknowledge</Button>
          <Button variant="outlined" size="small" sx={{ fontSize: 13, color: '#10b981', borderColor: '#10b981' }}>Resolve</Button>
        </Box>
      </Box>

      <Grid container spacing={2} sx={{ mb: 3 }}>
        {[
          { label: 'Status', value: alert.status.toUpperCase(), color: severityColors[alert.severity] },
          { label: 'Duration', value: alert.duration },
          { label: 'Value', value: alert.value },
          { label: 'Threshold', value: `${thresholdValue}%` },
        ].map((stat) => (
          <Grid key={stat.label} size={{ xs: 6, md: 3 }}>
            <Box sx={{ p: 1.5, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
              <Typography variant="caption" sx={{ color: theme.palette.text.secondary }}>{stat.label}</Typography>
              <Typography variant="metricSm" sx={{ color: stat.color || theme.palette.text.primary }}>{stat.value}</Typography>
            </Box>
          </Grid>
        ))}
      </Grid>

      <Box sx={{ p: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px', mb: 3 }}>
        <Typography variant="h4" sx={{ mb: 1 }}>Metric Chart</Typography>
        <ReactEChartsCore echarts={echarts} option={chartOpt} style={{ height: 320 }} notMerge />
      </Box>

      <Typography variant="h4" sx={{ mb: 1 }}>Details</Typography>
      {[
        ['Service', alert.service],
        ['Condition', alert.condition],
        ['Triggered by', alert.condition.split(' >')[0]],
        ['Duration', alert.duration],
        ['Started', alert.started],
        ['Assignee', alert.assignee || 'Unassigned'],
      ].map(([k, v]) => (
        <Box key={k} sx={{ display: 'flex', py: 0.5, borderBottom: `1px solid ${theme.palette.divider}` }}>
          <Typography variant="caption" sx={{ minWidth: 140, color: theme.palette.text.secondary, textTransform: 'none', fontWeight: 500 }}>{k}</Typography>
          <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary }}>{v}</Typography>
        </Box>
      ))}
    </Box>
  )
}
