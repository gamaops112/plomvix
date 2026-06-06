import { useState, useMemo } from 'react'
import {
  Box, Typography, Grid, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper,
} from '@mui/material'
import { useTheme } from '@mui/material/styles'
import MetricsQueryBar from './MetricsQueryBar'
import MetricsExplorerChart from './MetricsExplorerChart'
import MetricChart from './MetricChart'
import { presetMetrics, serviceMetrics, generateTimeSeries } from '../mockData'

export default function MetricsExplorerTab() {
  const theme = useTheme()
  const [metric, setMetric] = useState('system.cpu.usage')
  const [aggregation, setAggregation] = useState('avg')
  const [groupBy, setGroupBy] = useState('host')
  const [selectedPreset, setSelectedPreset] = useState('system.cpu.usage')

  const presetDataMap = useMemo(() => {
    const map: Record<string, number[]> = {}
    presetMetrics.forEach((p) => {
      map[p.metric] = generateTimeSeries(30 + Math.random() * 30, 10, 60)
    })
    return map
  }, [])

  const errorColor = (pct: number) => {
    if (pct > 5) return '#ef4444'
    if (pct > 1) return '#f59e0b'
    return theme.palette.text.primary
  }

  const p99Color = (val: number) => {
    if (val > 1000) return '#ef4444'
    if (val > 500) return '#f59e0b'
    return theme.palette.text.primary
  }

  return (
    <Box>
      <MetricsQueryBar
        metric={metric}
        onMetricChange={setMetric}
        aggregation={aggregation}
        onAggChange={setAggregation}
        groupBy={groupBy}
        onGroupByChange={setGroupBy}
      />

      <MetricsExplorerChart />

      <Grid container spacing={2} sx={{ mt: 2 }}>
        {presetMetrics.map((p) => {
          const isActive = selectedPreset === p.metric
          const data = presetDataMap[p.metric]
          const avg = data ? (data.reduce((a: number, b: number) => a + b, 0) / data.length).toFixed(1) : '0'
          return (
            <Grid key={p.metric} size={{ xs: 6, md: 3 }}>
              <Box
                onClick={() => { setSelectedPreset(p.metric); setMetric(p.metric) }}
                sx={{
                  p: 1.5,
                  border: `1px solid ${isActive ? '#06b6d440' : theme.palette.divider}`,
                  borderRadius: '4px',
                  background: isActive ? '#06b6d408' : 'transparent',
                  cursor: 'pointer',
                  transition: 'border-color 0.2s',
                }}
              >
                <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
                  <Typography variant="caption" sx={{ color: theme.palette.text.secondary }}>
                    {p.label}
                  </Typography>
                  <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
                    avg {avg}{p.unit}
                  </Typography>
                </Box>
                <MetricChart data={data} color={p.color} height={80} showArea />
              </Box>
            </Grid>
          )
        })}
      </Grid>

      <Box sx={{ mt: 4 }}>
        <Typography variant="h4" sx={{ mb: 2 }}>Service Metrics</Typography>
        <TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell sx={{ fontSize: 11, color: '#8b93a8', fontWeight: 500, letterSpacing: '0.04em', textTransform: 'uppercase', py: 1, borderColor: 'divider' }}>
                  Service
                </TableCell>
                <TableCell sx={{ fontSize: 11, color: '#8b93a8', fontWeight: 500, letterSpacing: '0.04em', textTransform: 'uppercase', py: 1, borderColor: 'divider' }}>
                  Req/s
                </TableCell>
                <TableCell sx={{ fontSize: 11, color: '#8b93a8', fontWeight: 500, letterSpacing: '0.04em', textTransform: 'uppercase', py: 1, borderColor: 'divider' }}>
                  Error%
                </TableCell>
                <TableCell sx={{ fontSize: 11, color: '#8b93a8', fontWeight: 500, letterSpacing: '0.04em', textTransform: 'uppercase', py: 1, borderColor: 'divider' }}>
                  P50
                </TableCell>
                <TableCell sx={{ fontSize: 11, color: '#8b93a8', fontWeight: 500, letterSpacing: '0.04em', textTransform: 'uppercase', py: 1, borderColor: 'divider' }}>
                  P95
                </TableCell>
                <TableCell sx={{ fontSize: 11, color: '#8b93a8', fontWeight: 500, letterSpacing: '0.04em', textTransform: 'uppercase', py: 1, borderColor: 'divider' }}>
                  P99
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {serviceMetrics.map((s) => (
                <TableRow key={s.service} hover>
                  <TableCell sx={{ fontSize: 13, color: theme.palette.text.primary, py: 0.75, borderColor: 'divider' }}>
                    {s.service}
                  </TableCell>
                  <TableCell sx={{ fontSize: 13, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary, py: 0.75, borderColor: 'divider' }}>
                    {s.reqs.toLocaleString()}
                  </TableCell>
                  <TableCell sx={{ fontSize: 13, fontFamily: theme.typography.mono.fontFamily, color: errorColor(s.errorPct), py: 0.75, borderColor: 'divider' }}>
                    {s.errorPct}%
                  </TableCell>
                  <TableCell sx={{ fontSize: 13, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary, py: 0.75, borderColor: 'divider' }}>
                    {s.p50 > 1000 ? `${(s.p50 / 1000).toFixed(1)}s` : `${s.p50}ms`}
                  </TableCell>
                  <TableCell sx={{ fontSize: 13, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary, py: 0.75, borderColor: 'divider' }}>
                    {s.p95 > 0 ? (s.p95 > 1000 ? `${(s.p95 / 1000).toFixed(1)}s` : `${s.p95}ms`) : '—'}
                  </TableCell>
                  <TableCell sx={{ fontSize: 13, fontFamily: theme.typography.mono.fontFamily, color: p99Color(s.p99), py: 0.75, borderColor: 'divider' }}>
                    {s.p99 > 0 ? (s.p99 > 1000 ? `${(s.p99 / 1000).toFixed(1)}s` : `${s.p99}ms`) : '—'}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>
    </Box>
  )
}
