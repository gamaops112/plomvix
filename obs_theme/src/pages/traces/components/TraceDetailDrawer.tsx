import { useState } from 'react'
import {
  Box, Typography, Drawer, Tabs, Tab, IconButton, Button, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper,
} from '@mui/material'
import { X, ExternalLink } from 'lucide-react'
import { useTheme } from '@mui/material/styles'
import { useNavigate } from 'react-router-dom'
import type { Trace } from '../mockData'
import { mockSpanTree } from '../mockData'
import SpanWaterfall from './SpanWaterfall'

interface TraceDetailDrawerProps {
  open: boolean
  trace: Trace | null
  onClose: () => void
}

export default function TraceDetailDrawer({ open, trace, onClose }: TraceDetailDrawerProps) {
  const theme = useTheme()
  const navigate = useNavigate()
  const [tab, setTab] = useState(0)

  if (!trace) return null

  const spanTree = mockSpanTree

  const allSpans = (() => {
    const result: { id: string; service: string; operation: string; duration: number; startOffset: number; status: string }[] = []
    const walk = (span: typeof spanTree.rootSpan) => {
      result.push({ id: span.id, service: span.service, operation: span.operation, duration: span.duration, startOffset: span.startOffset, status: span.status })
      span.children.forEach(walk)
    }
    walk(spanTree.rootSpan)
    return result
  })()

  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      slotProps={{
        paper: {
          sx: {
            width: 640,
            background: theme.palette.background.paper,
            borderLeft: `1px solid ${theme.palette.divider}`,
          },
        },
      }}
    >
      <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        <Box sx={{ px: 2, py: 1.5, borderBottom: `1px solid ${theme.palette.divider}` }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <Box>
              <Typography variant="h3">Trace: {trace.id.substring(0, 8)}</Typography>
              <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
                {trace.rootOp} &bull; {trace.duration > 1000 ? `${(trace.duration / 1000).toFixed(1)}s` : `${trace.duration}ms`} &bull; {trace.spans} spans &bull; {trace.errors} errors
              </Typography>
            </Box>
            <Box sx={{ display: 'flex', gap: 1 }}>
              <Button size="small" startIcon={<ExternalLink size={14} />} sx={{ fontSize: 12 }}
                onClick={() => { onClose(); navigate(`/traces/${trace.id}`) }}>
                Open full page →
              </Button>
              <IconButton size="small" onClick={onClose} sx={{ color: theme.palette.text.secondary }}>
                <X size={18} />
              </IconButton>
            </Box>
          </Box>
        </Box>

        <Tabs value={tab} onChange={(_, v) => setTab(v)}>
          <Tab label="Waterfall" />
          <Tab label="Spans" />
          <Tab label="Logs" />
          <Tab label="Summary" />
        </Tabs>

        <Box sx={{ flex: 1, overflow: 'auto' }}>
          {tab === 0 && (
            <Box sx={{ p: 2 }}>
              <SpanWaterfall
                totalDuration={spanTree.totalDuration}
                rootSpan={spanTree.rootSpan}
                traceId={spanTree.traceId}
                rootOperation={spanTree.rootOperation}
              />
            </Box>
          )}

          {tab === 1 && (
            <TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell sx={{ fontSize: 11, color: '#8b93a8', py: 1, borderColor: 'divider' }}>Span ID</TableCell>
                    <TableCell sx={{ fontSize: 11, color: '#8b93a8', py: 1, borderColor: 'divider' }}>Service</TableCell>
                    <TableCell sx={{ fontSize: 11, color: '#8b93a8', py: 1, borderColor: 'divider' }}>Operation</TableCell>
                    <TableCell sx={{ fontSize: 11, color: '#8b93a8', py: 1, borderColor: 'divider' }}>Duration</TableCell>
                    <TableCell sx={{ fontSize: 11, color: '#8b93a8', py: 1, borderColor: 'divider' }}>Start</TableCell>
                    <TableCell sx={{ fontSize: 11, color: '#8b93a8', py: 1, borderColor: 'divider' }}>Status</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {allSpans.map((s) => (
                    <TableRow key={s.id} hover>
                      <TableCell sx={{ fontSize: 12, fontFamily: theme.typography.mono.fontFamily, color: '#06b6d4', py: 0.5, borderColor: 'divider' }}>{s.id}</TableCell>
                      <TableCell sx={{ fontSize: 12, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary, py: 0.5, borderColor: 'divider' }}>{s.service}</TableCell>
                      <TableCell sx={{ fontSize: 12, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary, py: 0.5, borderColor: 'divider' }}>{s.operation}</TableCell>
                      <TableCell sx={{ fontSize: 12, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary, py: 0.5, borderColor: 'divider' }}>{s.duration}ms</TableCell>
                      <TableCell sx={{ fontSize: 12, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary, py: 0.5, borderColor: 'divider' }}>{s.startOffset}ms</TableCell>
                      <TableCell sx={{ fontSize: 12, color: s.status === 'error' ? '#ef4444' : '#10b981', py: 0.5, borderColor: 'divider' }}>{s.status}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}

          {tab === 2 && (
            <Box sx={{ p: 2 }}>
              <Typography variant="body2" sx={{ color: theme.palette.text.secondary, py: 2, textAlign: 'center' }}>
                Logs correlated to trace {trace.id} — connect in Phase 7
              </Typography>
            </Box>
          )}

          {tab === 3 && (
            <Box sx={{ p: 2 }}>
              <Box sx={{ mb: 2 }}>
                <Typography variant="h4" sx={{ mb: 1 }}>Trace Info</Typography>
                {[
                  ['Trace ID', trace.id],
                  ['Root Service', trace.rootService],
                  ['Root Operation', trace.rootOp],
                  ['Start Time', `2024-01-15 ${trace.time}`],
                  ['Duration', `${trace.duration}ms`],
                  ['Total Spans', String(trace.spans)],
                  ['Error Spans', String(trace.errors)],
                  ['Status', trace.status],
                ].map(([k, v]) => (
                  <Box key={k} sx={{ display: 'flex', py: 0.5, borderBottom: `1px solid ${theme.palette.divider}` }}>
                    <Typography variant="caption" sx={{ minWidth: 140, color: theme.palette.text.secondary, textTransform: 'none', fontWeight: 500 }}>{k}</Typography>
                    <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary }}>{v}</Typography>
                  </Box>
                ))}
              </Box>

              <Typography variant="h4" sx={{ mb: 1, mt: 3 }}>Span Breakdown</Typography>
              {(() => {
                const groups: Record<string, { count: number; totalDuration: number }> = {}
                allSpans.forEach((s) => {
                  if (!groups[s.service]) groups[s.service] = { count: 0, totalDuration: 0 }
                  groups[s.service].count++
                  groups[s.service].totalDuration += s.duration
                })
                return Object.entries(groups).map(([svc, data]) => (
                  <Box key={svc} sx={{ display: 'flex', py: 0.5, borderBottom: `1px solid ${theme.palette.divider}` }}>
                    <Typography variant="body2" sx={{ flex: 1, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary }}>{svc}</Typography>
                    <Typography variant="caption2" sx={{ mx: 1, color: theme.palette.text.secondary }}>{data.count} span{data.count > 1 ? 's' : ''}</Typography>
                    <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary }}>{data.totalDuration}ms</Typography>
                  </Box>
                ))
              })()}
            </Box>
          )}
        </Box>
      </Box>
    </Drawer>
  )
}
