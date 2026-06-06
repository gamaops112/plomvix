import { useMemo } from 'react'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { useTheme } from '@mui/material/styles'
import { generateTimeSeries } from '../mockData'

echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

export default function LatencyChart() {
  const theme = useTheme()

  const option = useMemo(() => ({
    tooltip: { trigger: 'axis' as const, backgroundColor: theme.palette.background.elevated, borderColor: theme.palette.divider, textStyle: { color: theme.palette.text.primary, fontSize: 12 } },
    legend: { bottom: 0, textStyle: { color: theme.palette.text.secondary, fontSize: 12 } },
    grid: { top: 16, right: 16, bottom: 36, left: 48 },
    xAxis: { type: 'category' as const, data: Array.from({ length: 60 }, (_, i) => `${i}m`), axisLabel: { color: theme.palette.text.disabled, fontSize: 11 }, axisLine: { lineStyle: { color: theme.palette.divider } } },
    yAxis: { type: 'value' as const, name: 'ms', splitLine: { lineStyle: { color: theme.palette.divider } }, axisLabel: { color: theme.palette.text.disabled, fontSize: 11 }, nameTextStyle: { color: theme.palette.text.disabled } },
    series: [
      { name: 'api-gateway P50', type: 'line', data: generateTimeSeries(45, 10), smooth: true, symbol: 'none', lineStyle: { color: '#06b6d4', width: 2 } },
      { name: 'api-gateway P95', type: 'line', data: generateTimeSeries(124, 30), smooth: true, symbol: 'none', lineStyle: { color: '#06b6d4', width: 1, type: 'dashed' as const } },
      { name: 'user-service P50', type: 'line', data: generateTimeSeries(312, 80), smooth: true, symbol: 'none', lineStyle: { color: '#10b981', width: 2 } },
      { name: 'user-service P95', type: 'line', data: generateTimeSeries(891, 200), smooth: true, symbol: 'none', lineStyle: { color: '#10b981', width: 1, type: 'dashed' as const } },
    ],
  }), [theme])

  return <ReactEChartsCore echarts={echarts} option={option} style={{ height: 280 }} notMerge />
}
