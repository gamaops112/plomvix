import { useMemo, useState, useCallback } from 'react'
import { Box, Typography, Paper, IconButton } from '@mui/material'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import * as echarts from 'echarts/core'
import { GraphChart } from 'echarts/charts'
import { TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { X } from 'lucide-react'
import { useTheme } from '@mui/material/styles'
import type { ServiceData } from '../mockData'
import { mockServices } from '../mockData'

echarts.use([GraphChart, TooltipComponent, CanvasRenderer])

const statusColor: Record<string, string> = { healthy: '#10b981', degraded: '#f59e0b', down: '#ef4444' }

export default function ApmServiceMap() {
  const theme = useTheme()
  const [popover, setPopover] = useState<{ svc: ServiceData; x: number; y: number } | null>(null)

  const nodes = [
    { id: 'gateway', name: 'api-gateway', x: 300, y: 100, symbolSize: 40 },
    { id: 'auth', name: 'auth-service', x: 150, y: 210, symbolSize: 28 },
    { id: 'user', name: 'user-service', x: 450, y: 210, symbolSize: 28 },
    { id: 'payment', name: 'payment-service', x: 150, y: 330, symbolSize: 28 },
    { id: 'search', name: 'search-service', x: 450, y: 330, symbolSize: 22 },
    { id: 'cache', name: 'cache-service', x: 550, y: 100, symbolSize: 30 },
    { id: 'storage', name: 'storage-service', x: 300, y: 420, symbolSize: 30 },
    { id: 'queue', name: 'queue-service', x: 300, y: 260, symbolSize: 22 },
    { id: 'notification', name: 'notification-svc', x: 600, y: 260, symbolSize: 18 },
  ]

  const edges = [
    { source: 'gateway', target: 'auth' }, { source: 'gateway', target: 'user' },
    { source: 'gateway', target: 'payment' }, { source: 'gateway', target: 'search' },
    { source: 'gateway', target: 'queue' }, { source: 'search', target: 'cache' },
    { source: 'user', target: 'storage' }, { source: 'payment', target: 'storage' },
    { source: 'queue', target: 'notification' },
  ]

  const seriesNodes = nodes.map((n) => {
    const svc = mockServices.find((s) => s.name === n.name)
    return {
      ...n,
      itemStyle: { color: svc ? statusColor[svc.status] : '#4d566b' },
      symbolSize: Math.max(14, n.symbolSize * (svc ? svc.reqRate / 5000 : 1)),
      label: { show: true, position: 'bottom', color: '#8b93a8', fontSize: 11 },
    }
  })

  const handleClick = useCallback((event: { name: string }) => {
    const svc = mockServices.find((s) => s.name === event.name)
    if (svc) {
      setPopover({ svc, x: 300, y: 200 })
    }
  }, [])

  const option = useMemo(() => ({
    tooltip: {
      backgroundColor: theme.palette.background.elevated, borderColor: theme.palette.divider, textStyle: { color: theme.palette.text.primary, fontSize: 12 },
      formatter: (p: { data?: { name?: string } }) => p.data?.name || '',
    },
    series: [{
      type: 'graph', layout: 'none', roam: true, draggable: false,
      data: seriesNodes,
      links: edges.map((e) => ({ source: e.source, target: e.target, lineStyle: { color: theme.palette.divider, width: 1 } })),
    }],
  }), [theme, seriesNodes])

  return (
    <Box sx={{ position: 'relative', height: 360 }}>
      <ReactEChartsCore
        echarts={echarts}
        option={option}
        style={{ height: 360 }}
        onEvents={{ click: handleClick }}
        notMerge
      />

      {popover && (
        <Paper
          sx={{
            position: 'fixed',
            top: popover.y + 20,
            left: popover.x,
            transform: 'translateX(-50%)',
            zIndex: 1300,
            width: 220,
            p: 1.5,
            bgcolor: 'background.elevated',
            border: 1,
            borderColor: 'divider',
            borderRadius: '4px',
          }}
        >
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
            <Typography variant="h4">{popover.svc.name}</Typography>
            <IconButton size="small" onClick={() => setPopover(null)} sx={{ color: 'text.secondary', p: 0 }}>
              <X size={14} />
            </IconButton>
          </Box>
          <Box sx={{ '& > *': { py: 0.25 } }}>
            <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
              Req/s: <span style={{ color: theme.palette.text.primary }}>{popover.svc.reqRate.toLocaleString()}</span>
            </Typography>
            <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
              Error%: <span style={{ color: popover.svc.errorRate > 5 ? '#ef4444' : theme.palette.text.primary }}>{popover.svc.errorRate}%</span>
            </Typography>
            <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
              P99: <span style={{ color: popover.svc.p99 && popover.svc.p99 > 500 ? '#ef4444' : theme.palette.text.primary }}>
                {popover.svc.p99 !== null ? `${popover.svc.p99}ms` : '—'}
              </span>
            </Typography>
            <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
              Instances: <span style={{ color: theme.palette.text.primary }}>{popover.svc.instances}</span>
            </Typography>
          </Box>
        </Paper>
      )}
    </Box>
  )
}
