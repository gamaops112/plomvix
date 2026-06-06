import { useMemo } from 'react'
import { Card, CardContent, CardHeader } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import * as echarts from 'echarts/core'
import { GraphChart } from 'echarts/charts'
import { TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { serviceMapData } from '../mockData'

echarts.use([GraphChart, TooltipComponent, CanvasRenderer])

const statusNodeColors: Record<string, string> = {
  healthy: '#10b981',
  degraded: '#f59e0b',
  down: '#ef4444',
}

export default function ServiceMap() {
  const theme = useTheme()

  const option = useMemo(() => ({
    tooltip: {
      backgroundColor: '#1c2333',
      borderColor: '#2a3147',
      textStyle: { color: '#e8eaf0', fontSize: 12 },
      formatter: (params: { data?: { name?: string; status?: string } }) => {
        if (!params.data) return ''
        const { name, status } = params.data
        return `${name}<br/>Status: ${status}`
      },
    },
    series: [{
      type: 'graph',
      layout: 'none',
      roam: false,
      draggable: false,
      data: serviceMapData.nodes.map((node) => ({
        ...node,
        itemStyle: { color: statusNodeColors[node.status] },
        label: {
          show: true,
          position: 'bottom',
          color: '#8b93a8',
          fontSize: 11,
        },
      })),
      links: serviceMapData.edges.map((edge) => ({
        source: edge.source,
        target: edge.target,
        lineStyle: { color: '#3d4663', width: 1 },
      })),
    }],
  }), [])

  return (
    <Card sx={{ height: '100%' }}>
      <CardHeader
        title="Service Map"
        sx={{ borderBottom: `1px solid ${theme.palette.divider}` }}
      />
      <CardContent>
        <ReactEChartsCore
          echarts={echarts}
          option={option}
          style={{ height: 280 }}
          notMerge
        />
      </CardContent>
    </Card>
  )
}
