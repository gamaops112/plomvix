import { useState } from 'react'
import {
  Box, Typography, Drawer, Tabs, Tab, IconButton, Button, Grid,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper,
} from '@mui/material'
import { X, ExternalLink } from 'lucide-react'
import { useTheme } from '@mui/material/styles'
import { useNavigate } from 'react-router-dom'
import type { Host } from '../mockData'
import { generateTimeSeries, mockProcesses } from '../mockData'
import MetricChart from './MetricChart'

interface HostDetailDrawerProps {
  open: boolean
  host: Host | null
  onClose: () => void
}

export default function HostDetailDrawer({ open, host, onClose }: HostDetailDrawerProps) {
  const theme = useTheme()
  const navigate = useNavigate()
  const [tab, setTab] = useState(0)

  if (!host) return null

  const cpuSeries = generateTimeSeries(host.cpu || 23, 8)
  const memSeries = generateTimeSeries(host.memUsage || 60, 10)

  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      slotProps={{
        paper: {
          sx: {
            width: 520,
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
              <Typography variant="h3">{host.name}</Typography>
              <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
                {host.os || 'Unknown'} &bull; {host.name} &bull; Last seen: just now
              </Typography>
            </Box>
            <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
              <Button
                size="small"
                startIcon={<ExternalLink size={14} />}
                sx={{ fontSize: 12 }}
                onClick={() => { onClose(); navigate(`/metrics/hosts/${host.id}`) }}
              >
                Open full page →
              </Button>
              <IconButton size="small" onClick={onClose} sx={{ color: theme.palette.text.secondary }}>
                <X size={18} />
              </IconButton>
            </Box>
          </Box>
        </Box>

        <Tabs value={tab} onChange={(_, v) => setTab(v)}>
          <Tab label="Overview" />
          <Tab label="Metrics" />
          <Tab label="Logs" />
          <Tab label="Processes" />
        </Tabs>

        <Box sx={{ flex: 1, overflow: 'auto' }}>
          {tab === 0 && (
            <Box sx={{ p: 2 }}>
              <Grid container spacing={2}>
                <Grid size={6}>
                  <Box sx={{ p: 1.5, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
                    <Typography variant="caption">CPU</Typography>
                    <Typography variant="metricSm">{host.cpu || '—'}%</Typography>
                    <MetricChart data={cpuSeries} color="#06b6d4" height={60} />
                  </Box>
                </Grid>
                <Grid size={6}>
                  <Box sx={{ p: 1.5, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
                    <Typography variant="caption">Memory</Typography>
                    <Typography variant="metricSm">{host.memUsage || '—'}%</Typography>
                    <MetricChart data={memSeries} color="#8b5cf6" height={60} />
                  </Box>
                </Grid>
                <Grid size={6}>
                  <Box sx={{ p: 1.5, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
                    <Typography variant="caption">Network RX</Typography>
                    <Typography variant="metricSm">{host.rx || '—'} Mbit/s</Typography>
                  </Box>
                </Grid>
                <Grid size={6}>
                  <Box sx={{ p: 1.5, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
                    <Typography variant="caption">Network TX</Typography>
                    <Typography variant="metricSm">{host.tx || '—'} Mbit/s</Typography>
                  </Box>
                </Grid>
              </Grid>

              <Box sx={{ mt: 3 }}>
                <Typography variant="h4" sx={{ mb: 1.5 }}>System Info</Typography>
                {[
                  ['Hostname', host.name],
                  ['OS', `${host.os || 'Unknown'} 22.04`],
                  ['Kernel', '5.15.0-1034-gke'],
                  ['IP Address', '192.168.1.42'],
                  ['Uptime', '12d 4h 22m'],
                  ['CPU Cores', '8'],
                  ['Total Memory', host.memTotal > 0 ? `${host.memTotal} GB` : '—'],
                  ['Cloud Provider', 'GCP'],
                  ['Region', 'us-central1-a'],
                ].map(([label, value]) => (
                  <Box key={label} sx={{ display: 'flex', py: 0.75, borderBottom: `1px solid ${theme.palette.divider}` }}>
                    <Typography variant="caption" sx={{ minWidth: 140, color: theme.palette.text.secondary, textTransform: 'none', fontWeight: 500 }}>
                      {label}
                    </Typography>
                    <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary }}>
                      {value}
                    </Typography>
                  </Box>
                ))}
              </Box>
            </Box>
          )}

          {tab === 1 && (
            <Box sx={{ p: 2 }}>
              <Typography variant="caption" sx={{ mb: 0.5, display: 'block' }}>CPU Usage %</Typography>
              <MetricChart data={cpuSeries} color="#06b6d4" />

              <Typography variant="caption" sx={{ mb: 0.5, mt: 2, display: 'block' }}>Memory Usage %</Typography>
              <MetricChart data={memSeries} color="#8b5cf6" />

              <Typography variant="caption" sx={{ mb: 0.5, mt: 2, display: 'block' }}>Network RX/TX Mbit/s</Typography>
              <MetricChart data={generateTimeSeries(host.rx || 50, 15)} color="#10b981" />

              <Typography variant="caption" sx={{ mb: 0.5, mt: 2, display: 'block' }}>Disk Latency ms</Typography>
              <MetricChart data={generateTimeSeries(host.diskLatency || 5, 3)} color="#f59e0b" />
            </Box>
          )}

          {tab === 2 && (
            <Box sx={{ p: 1 }}>
              <Typography variant="body2" sx={{ color: theme.palette.text.secondary, py: 2, textAlign: 'center' }}>
                Logs for {host.name} — connect to logs stream in Phase 7
              </Typography>
            </Box>
          )}

          {tab === 3 && (
            <Box sx={{ p: 2 }}>
              <TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ fontSize: 11, color: 'text.secondary', py: 1, borderColor: 'divider' }}>PID</TableCell>
                      <TableCell sx={{ fontSize: 11, color: 'text.secondary', py: 1, borderColor: 'divider' }}>Name</TableCell>
                      <TableCell sx={{ fontSize: 11, color: 'text.secondary', py: 1, borderColor: 'divider' }}>CPU</TableCell>
                      <TableCell sx={{ fontSize: 11, color: 'text.secondary', py: 1, borderColor: 'divider' }}>Memory</TableCell>
                      <TableCell sx={{ fontSize: 11, color: 'text.secondary', py: 1, borderColor: 'divider' }}>Status</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {mockProcesses.map((p) => (
                      <TableRow key={p.pid} hover>
                        <TableCell sx={{ fontSize: 13, color: theme.palette.text.primary, py: 0.75, borderColor: 'divider' }}>{p.pid}</TableCell>
                        <TableCell sx={{ fontSize: 13, color: theme.palette.text.primary, py: 0.75, borderColor: 'divider' }}>{p.name}</TableCell>
                        <TableCell sx={{ fontSize: 13, fontFamily: theme.typography.mono.fontFamily, color: '#10b981', py: 0.75, borderColor: 'divider' }}>{p.cpu}%</TableCell>
                        <TableCell sx={{ fontSize: 13, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary, py: 0.75, borderColor: 'divider' }}>{p.mem} GB</TableCell>
                        <TableCell sx={{ fontSize: 13, color: '#10b981', py: 0.75, borderColor: 'divider' }}>{p.status}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            </Box>
          )}
        </Box>
      </Box>
    </Drawer>
  )
}
