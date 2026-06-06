import { useState, useMemo } from 'react'
import { Box, Typography, Drawer, Tabs, Tab, IconButton, Button } from '@mui/material'
import { X, Pencil } from 'lucide-react'
import { useTheme } from '@mui/material/styles'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])

interface Monitor {
  id: string; name: string; url: string; type: string; status: string
  availability: number; avgDuration: number | null; lastCheck: string; frequency: string
}

interface MonitorDetailDrawerProps { open: boolean; monitor: Monitor | null; onClose: () => void }

const statusDot: Record<string, string> = { up: '#10b981', down: '#ef4444', degraded: '#f59e0b' }

export default function MonitorDetailDrawer({ open, monitor, onClose }: MonitorDetailDrawerProps) {
  const theme = useTheme()
  const [tab, setTab] = useState(0)

  const generateSeries = (base: number, variance: number) =>
    Array.from({ length: 24 }, () => +(base + (Math.random() - 0.5) * variance * 2).toFixed(1))

  const chartOpt = useMemo(() => ({
    tooltip: { trigger: 'axis' as const, backgroundColor: theme.palette.background.elevated, borderColor: theme.palette.divider, textStyle: { color: theme.palette.text.primary, fontSize: 12 } },
    grid: { top: 8, right: 8, bottom: 24, left: 40 },
    xAxis: { type: 'category' as const, data: Array.from({ length: 24 }, (_, i) => `${i}h`), axisLabel: { color: theme.palette.text.disabled, fontSize: 10 } },
    yAxis: { type: 'value' as const, name: 'ms', splitLine: { lineStyle: { color: theme.palette.divider } }, axisLabel: { color: theme.palette.text.disabled, fontSize: 10 } },
    series: [
      { name: 'P50', type: 'line', data: generateSeries(45, 10), smooth: true, symbol: 'none', lineStyle: { color: '#10b981', width: 2 } },
      { name: 'P75', type: 'line', data: generateSeries(70, 15), smooth: true, symbol: 'none', lineStyle: { color: '#f59e0b', width: 2 } },
      { name: 'P95', type: 'line', data: generateSeries(120, 30), smooth: true, symbol: 'none', lineStyle: { color: '#ef4444', width: 2 } },
      { name: 'Max', type: 'line', data: generateSeries(180, 50), smooth: true, symbol: 'none', lineStyle: { color: '#8b5cf6', width: 1, type: 'dashed' as const } },
    ],
  }), [theme])

  const historyRows = Array.from({ length: 10 }, (_, i) => ({
    time: new Date(Date.now() - i * 60000).toLocaleTimeString(),
    status: Math.random() > 0.9 ? 'down' : Math.random() > 0.8 ? 'degraded' : 'up',
    duration: Math.floor(30 + Math.random() * 60),
    location: Math.random() > 0.5 ? 'US East' : 'EU West',
  }))

  const statusSquares = Array.from({ length: 288 }, () => {
    const r = Math.random()
    return r > 0.97 ? 'down' : r > 0.93 ? 'degraded' : 'up'
  })

  if (!monitor) return null

  return (
    <Drawer anchor="right" open={open} onClose={onClose}
      slotProps={{ paper: { sx: { width: 480, background: theme.palette.background.paper, borderLeft: `1px solid ${theme.palette.divider}` } } }}>
      <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        <Box sx={{ px: 2, py: 1.5, borderBottom: `1px solid ${theme.palette.divider}` }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: statusDot[monitor.status] }} />
                <Typography variant="h3">{monitor.name}</Typography>
              </Box>
              <Typography variant="caption2" sx={{ color: 'text.secondary' }}>
                {monitor.url} &bull; {monitor.type} &bull; Last checked: {monitor.lastCheck}
              </Typography>
            </Box>
            <Box sx={{ display: 'flex', gap: 1 }}>
              <Button size="small" startIcon={<Pencil size={14} />} sx={{ fontSize: 12 }}>Edit</Button>
              <IconButton size="small" onClick={onClose} sx={{ color: 'text.secondary' }}><X size={18} /></IconButton>
            </Box>
          </Box>
        </Box>

        <Tabs value={tab} onChange={(_, v) => setTab(v)}>
          <Tab label="Overview" /><Tab label="History" /><Tab label="Errors" />
        </Tabs>

        <Box sx={{ flex: 1, overflow: 'auto' }}>
          {tab === 0 && (
            <Box sx={{ p: 2 }}>
              <Box sx={{ display: 'flex', gap: 2, mb: 2 }}>
                {[
                  { label: 'Availability', value: `${monitor.availability}%` },
                  { label: 'Avg Duration', value: monitor.avgDuration ? `${monitor.avgDuration}ms` : '—' },
                  { label: 'Checks/day', value: '1,440' },
                ].map((s) => (
                  <Box key={s.label} sx={{ flex: 1, p: 1.5, border: 1, borderColor: 'divider', borderRadius: '4px', textAlign: 'center' }}>
                    <Typography variant="caption2" sx={{ color: 'text.secondary' }}>{s.label}</Typography>
                    <Typography variant="metricSm">{s.value}</Typography>
                  </Box>
                ))}
              </Box>
              <Typography variant="caption" sx={{ color: 'text.secondary', mb: 1, display: 'block' }}>Duration Trend (last 24h)</Typography>
              <ReactEChartsCore echarts={echarts} option={chartOpt} style={{ height: 160 }} notMerge />
              <Typography variant="caption" sx={{ color: 'text.secondary', mt: 2, mb: 1, display: 'block' }}>Status History (last 24h)</Typography>
              <Box sx={{ display: 'flex', gap: 0.25, flexWrap: 'wrap' }}>
                {statusSquares.map((s, i) => (
                  <Box key={i} sx={{ width: 10, height: 10, borderRadius: '1px', background: statusDot[s] }} />
                ))}
              </Box>
              <Typography variant="caption" sx={{ color: 'text.disabled', mt: 2, display: 'block' }}>Monitor Details</Typography>
              {[['Type', monitor.type], ['URL', monitor.url], ['Frequency', `Every ${monitor.frequency}`], ['Created', '2 days ago']].map(([k, v]) => (
                <Box key={k} sx={{ display: 'flex', py: 0.5 }}>
                  <Typography variant="caption2" sx={{ minWidth: 120, color: 'text.secondary' }}>{k}</Typography>
                  <Typography variant="caption2" sx={{ fontFamily: theme.typography.mono.fontFamily }}>{v}</Typography>
                </Box>
              ))}
            </Box>
          )}
          {tab === 1 && (
            <Box sx={{ p: 2 }}>
              {historyRows.map((h, i) => (
                <Box key={i} sx={{ display: 'flex', alignItems: 'center', py: 0.5, borderBottom: `1px solid ${theme.palette.divider}`, height: 28 }}>
                  <Typography sx={{ fontFamily: theme.typography.mono.fontFamily, fontSize: 12, color: 'text.secondary', width: 80 }}>{h.time}</Typography>
                  <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: statusDot[h.status], mr: 1 }} />
                  <Typography variant="caption2" sx={{ width: 80 }}>{h.status}</Typography>
                  <Typography sx={{ fontFamily: theme.typography.mono.fontFamily, fontSize: 12, width: 80 }}>{h.duration}ms</Typography>
                  <Typography variant="caption2" sx={{ color: 'text.secondary' }}>{h.location}</Typography>
                </Box>
              ))}
            </Box>
          )}
          {tab === 2 && (
            <Box sx={{ p: 2, textAlign: 'center', py: 6 }}>
              <Typography variant="h4" sx={{ color: 'text.secondary', mb: 1 }}>No errors</Typography>
              <Typography variant="caption2" sx={{ color: 'text.disabled' }}>This monitor has been running without errors</Typography>
            </Box>
          )}
        </Box>
      </Box>
    </Drawer>
  )
}
