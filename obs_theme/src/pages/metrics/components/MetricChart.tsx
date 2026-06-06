import { useMemo } from 'react'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([LineChart, GridComponent, CanvasRenderer])

interface MetricChartProps {
  data: number[]
  color: string
  height?: number
  showArea?: boolean
}

export default function MetricChart({ data, color, height = 140, showArea = true }: MetricChartProps) {
  const option = useMemo(() => ({
    grid: { top: 4, right: 4, bottom: 16, left: 40 },
    xAxis: { show: false, data: data.map((_, i) => i) },
    yAxis: {
      show: false,
      min: Math.min(...data) * 0.9,
      max: Math.max(...data) * 1.1,
    },
    series: [{
      type: 'line',
      data,
      smooth: true,
      symbol: 'none',
      lineStyle: { color, width: 1.5 },
      areaStyle: showArea ? { color: `${color}20` } : undefined,
    }],
  }), [data, color, showArea])

  return (
    <ReactEChartsCore
      echarts={echarts}
      option={option}
      style={{ height }}
      notMerge
    />
  )
}
