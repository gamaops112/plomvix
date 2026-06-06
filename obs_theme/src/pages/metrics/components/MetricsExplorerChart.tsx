import { useMemo } from 'react'
import { Card, CardContent } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { metricsExplorerData } from '../mockData'

echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

const chartColors = ['#06b6d4', '#8b5cf6', '#f59e0b', '#10b981', '#f97316', '#ec4899']

export default function MetricsExplorerChart() {
  const theme = useTheme()

  const option = useMemo(() => ({
    tooltip: {
      trigger: 'axis' as const,
      backgroundColor: theme.palette.background.elevated,
      borderColor: theme.palette.divider,
      textStyle: { color: theme.palette.text.primary, fontSize: 12 },
    },
    legend: { bottom: 0, textStyle: { color: theme.palette.text.secondary, fontSize: 12 }, type: 'scroll' as const },
    grid: { top: 16, right: 16, bottom: 40, left: 48 },
    xAxis: {
      type: 'category' as const, data: metricsExplorerData.timestamps,
      axisLabel: { color: theme.palette.text.disabled, fontSize: 11 },
      axisLine: { lineStyle: { color: theme.palette.divider } },
    },
    yAxis: {
      type: 'value' as const,
      splitLine: { lineStyle: { color: theme.palette.divider } },
      axisLabel: { color: theme.palette.text.disabled, fontSize: 11 },
    },
    series: Object.entries(metricsExplorerData.series).map(([name, data], i) => ({
      name, type: 'line', data, smooth: true, symbol: 'none',
      lineStyle: { color: chartColors[i % chartColors.length], width: 2 },
    })),
  }), [theme])

  return (
    <Card>
      <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
        <ReactEChartsCore echarts={echarts} option={option} style={{ height: 320 }} notMerge />
      </CardContent>
    </Card>
  )
}
