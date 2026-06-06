import { useState, useMemo } from 'react'
import { Box, Typography, Grid, Tabs, Tab } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import TracesToolbar from './components/TracesToolbar'
import TracesTable from './components/TracesTable'
import TraceDetailDrawer from './components/TraceDetailDrawer'
import { mockTraces } from './mockData'
import type { Trace } from './mockData'
import { staticErrorData } from '../apm/mockData'
import { generateTimeSeries } from '../apm/mockData'

echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

export default function Traces() {
  const theme = useTheme()
  const [tab, setTab] = useState(0)
  const [search, setSearch] = useState('')
  const [serviceFilter, setServiceFilter] = useState('All')
  const [statusFilter, setStatusFilter] = useState('All')
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [selectedTrace, setSelectedTrace] = useState<Trace | null>(null)

  const filteredTraces = useMemo(() => {
    let result = mockTraces
    if (serviceFilter !== 'All') result = result.filter((t) => t.rootService === serviceFilter)
    if (statusFilter !== 'All') result = result.filter((t) => t.status === statusFilter)
    if (search) result = result.filter((t) => t.rootOp.toLowerCase().includes(search.toLowerCase()) || t.rootService.toLowerCase().includes(search.toLowerCase()))
    return result
  }, [serviceFilter, statusFilter, search])

  const openTrace = (trace: Trace) => {
    setSelectedTrace(trace)
    setDrawerOpen(true)
  }

  const latOption = useMemo(() => ({
    tooltip: { trigger: 'axis' as const, backgroundColor: theme.palette.background.elevated, borderColor: theme.palette.divider, textStyle: { color: theme.palette.text.primary, fontSize: 12 } },
    legend: { top: 0, textStyle: { color: theme.palette.text.secondary, fontSize: 12 } },
    grid: { top: 32, right: 16, bottom: 32, left: 48 },
    xAxis: { type: 'category' as const, data: Array.from({ length: 60 }, (_, i) => `${i}m`), axisLabel: { color: theme.palette.text.disabled, fontSize: 11 }, axisLine: { lineStyle: { color: theme.palette.divider } } },
    yAxis: { type: 'value' as const, name: 'ms', splitLine: { lineStyle: { color: theme.palette.divider } }, axisLabel: { color: theme.palette.text.disabled, fontSize: 11 }, nameTextStyle: { color: theme.palette.text.disabled } },
    series: [
      { name: 'P50', type: 'line', data: generateTimeSeries(80, 20), smooth: true, symbol: 'none', lineStyle: { color: '#10b981', width: 2 } },
      { name: 'P95', type: 'line', data: generateTimeSeries(250, 80), smooth: true, symbol: 'none', lineStyle: { color: '#f59e0b', width: 2 } },
      { name: 'P99', type: 'line', data: generateTimeSeries(600, 200), smooth: true, symbol: 'none', lineStyle: { color: '#ef4444', width: 2 } },
    ],
  }), [])

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h2" sx={{ mb: 2 }}>Traces</Typography>

      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 3 }}>
        <Tab label="Overview" />
        <Tab label="Traces" />
        <Tab label="Service Map" />
      </Tabs>

      {tab === 0 && (
        <Box>
          <Grid container spacing={2} sx={{ mb: 3 }}>
            {[
              { label: 'Total Traces', value: '24,821', color: '#06b6d4' },
              { label: 'Error Traces', value: '1,842', color: '#ef4444' },
              { label: 'Avg Duration', value: '234ms', color: '#8b5cf6' },
              { label: 'Throughput', value: '412/min', color: '#10b981' },
            ].map((card) => (
              <Grid key={card.label} size={{ xs: 6, md: 3 }}>
                <Box sx={{ p: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px', borderLeft: `3px solid ${card.color}` }}>
                  <Typography variant="caption" sx={{ color: theme.palette.text.secondary }}>{card.label}</Typography>
                  <Typography variant="metricSm">{card.value}</Typography>
                </Box>
              </Grid>
            ))}
          </Grid>

          <Grid container spacing={2}>
            <Grid size={{ xs: 12, md: 8 }}>
              <Box sx={{ p: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
                <Typography variant="h4" sx={{ mb: 1 }}>Latency Percentiles</Typography>
                <ReactEChartsCore echarts={echarts} option={latOption} style={{ height: 280 }} notMerge />
              </Box>
            </Grid>
            <Grid size={{ xs: 12, md: 4 }}>
              <Box sx={{ p: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
                <Typography variant="h4" sx={{ mb: 1.5 }}>Top Error Services</Typography>
                <Box component="table" sx={{ width: '100%' }}>
                  <Box component="thead">
                    <Box component="tr" sx={{ '& td': { fontSize: 11, color: theme.palette.text.secondary, fontWeight: 500, pb: 1 } }}>
                      <Box component="td">SERVICE</Box>
                      <Box component="td" sx={{ textAlign: 'right' }}>ERRORS</Box>
                      <Box component="td" sx={{ textAlign: 'right' }}>ERROR%</Box>
                    </Box>
                  </Box>
                  <Box component="tbody">
                    {staticErrorData.map((s) => (
                      <Box key={s.name} component="tr" sx={{ '& td': { py: 0.5, borderBottom: `1px solid ${theme.palette.divider}`, fontSize: 13 } }}>
                        <Box component="td" sx={{ fontFamily: theme.typography.mono.fontFamily }}>{s.name}</Box>
                        <Box component="td" sx={{ fontFamily: theme.typography.mono.fontFamily, textAlign: 'right' }}>{s.errors.toLocaleString()}</Box>
                        <Box component="td" sx={{
                          fontFamily: theme.typography.mono.fontFamily, textAlign: 'right',
                          color: s.errorPct > 5 ? '#ef4444' : s.errorPct > 1 ? '#f59e0b' : theme.palette.text.primary,
                        }}>
                          {s.errorPct}%
                        </Box>
                      </Box>
                    ))}
                  </Box>
                </Box>
              </Box>
            </Grid>
          </Grid>
        </Box>
      )}

      {tab === 1 && (
        <Box>
          <TracesToolbar
            search={search} onSearchChange={setSearch}
            serviceFilter={serviceFilter} onServiceChange={setServiceFilter}
            statusFilter={statusFilter} onStatusChange={setStatusFilter}
          />
          <TracesTable traces={filteredTraces} onSelectTrace={openTrace} />
        </Box>
      )}

      {tab === 2 && (
        <Box sx={{ p: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
          <Typography variant="body2" sx={{ color: theme.palette.text.secondary, textAlign: 'center', py: 4 }}>
            Enhanced service map — coming in Phase 7 polish
          </Typography>
        </Box>
      )}

      <TraceDetailDrawer open={drawerOpen} trace={selectedTrace} onClose={() => setDrawerOpen(false)} />
    </Box>
  )
}
