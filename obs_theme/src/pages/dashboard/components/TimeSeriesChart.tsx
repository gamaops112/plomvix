import { useMemo } from 'react'
import { Card, CardContent, CardHeader, Typography } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { timeSeriesData } from '../mockData'

echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

export default function TimeSeriesChart() {
  const theme = useTheme()

  const option = useMemo(() => ({
    tooltip: {
      trigger: 'axis' as const,
      backgroundColor: theme.palette.background.elevated,
      borderColor: theme.palette.divider,
      textStyle: { color: theme.palette.text.primary, fontSize: 12 },
    },
    legend: {
      top: 0,
      textStyle: { color: theme.palette.text.secondary, fontSize: 12 },
    },
    grid: { top: 32, right: 16, bottom: 32, left: 48 },
    xAxis: {
      type: 'category' as const,
      data: timeSeriesData.timestamps,
      axisLine: { lineStyle: { color: theme.palette.divider } },
      axisLabel: { color: theme.palette.text.disabled, fontSize: 11 },
    },
    yAxis: {
      type: 'value' as const,
      splitLine: { lineStyle: { color: theme.palette.divider } },
      axisLabel: { color: theme.palette.text.disabled, fontSize: 11 },
      name: 'req/s',
      nameTextStyle: { color: theme.palette.text.disabled, fontSize: 11 },
    },
    series: [
      { name: 'Request Rate', type: 'line', data: timeSeriesData.requestRate, smooth: true, symbol: 'none', lineStyle: { color: '#06b6d4', width: 2 }, areaStyle: { color: 'rgba(6,182,212,0.08)' } },
      { name: 'Error Rate', type: 'line', data: timeSeriesData.errorRate, yAxisIndex: 0, smooth: true, symbol: 'none', lineStyle: { color: '#ef4444', width: 2 } },
    ],
  }), [theme])

  return (
    <Card>
      <CardHeader
        title="Request Rate & Error Rate"
        action={<Typography variant="caption2" sx={{ color: 'text.secondary' }}>last 1h</Typography>}
        sx={{ borderBottom: `1px solid ${theme.palette.divider}` }}
      />
      <CardContent>
        <ReactEChartsCore echarts={echarts} option={option} style={{ height: 280 }} notMerge />
      </CardContent>
    </Card>
  )
}
