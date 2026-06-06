import { Box, Typography, Button, Grid } from '@mui/material'
import { useParams, useNavigate } from 'react-router-dom'
import { useTheme } from '@mui/material/styles'
import { ArrowLeft, RefreshCw } from 'lucide-react'
import { mockHosts, generateTimeSeries } from './mockData'
import type { Host } from './mockData'
import MetricChart from './components/MetricChart'

export default function HostDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const theme = useTheme()
  const host: Host | undefined = mockHosts.find((h: Host) => h.id === id)

  if (!host) {
    return (
      <Box sx={{ p: 3 }}>
        <Typography>Host not found</Typography>
      </Box>
    )
  }

  const cpuSeries = generateTimeSeries(host.cpu || 23, 8)
  const memSeries = generateTimeSeries(host.memUsage || 60, 10)
  const rxSeries = generateTimeSeries(host.rx || 50, 15)
  const txSeries = generateTimeSeries(host.tx || 40, 12)
  const diskSeries = generateTimeSeries(host.diskLatency || 5, 3)

  return (
    <Box sx={{ p: 3 }}>
      <Button
        startIcon={<ArrowLeft size={14} />}
        sx={{ color: theme.palette.text.secondary, fontSize: 13, mb: 2 }}
        onClick={() => navigate('/metrics')}
      >
        Back to Infrastructure
      </Button>

      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Typography variant="h2">{host.name}</Typography>
          <Box sx={{ px: 1, py: 0.25, borderRadius: '3px', background: '#1e2438', fontSize: 12, color: '#8b93a8' }}>
            {host.os || 'Unknown'}
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: '#10b981' }} />
            <Typography variant="caption2" sx={{ color: '#10b981' }}>Healthy</Typography>
          </Box>
        </Box>
        <Button
          variant="outlined"
          size="small"
          startIcon={<RefreshCw size={14} />}
          sx={{ fontSize: 13 }}
        >
          Refresh
        </Button>
      </Box>

      <Grid container spacing={2} sx={{ mb: 3 }}>
        {[
          { label: 'CPU', value: `${host.cpu || '—'}%`, color: '#06b6d4', data: cpuSeries },
          { label: 'Memory', value: host.memUsage > 0 ? `${host.memUsage}%` : '—', color: '#8b5cf6', data: memSeries },
          { label: 'RX', value: host.rx > 0 ? `${host.rx} Mbit/s` : '—', color: '#10b981', data: rxSeries },
          { label: 'TX', value: host.tx > 0 ? `${host.tx} Mbit/s` : '—', color: '#f59e0b', data: txSeries },
        ].map((stat) => (
          <Grid key={stat.label} size={{ xs: 6, md: 3 }}>
            <Box sx={{ p: 1.5, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
              <Typography variant="caption" sx={{ color: theme.palette.text.secondary }}>
                {stat.label}
              </Typography>
              <Typography variant="metricSm" sx={{ mt: 0.5 }}>{stat.value}</Typography>
              <MetricChart data={stat.data} color={stat.color} height={60} />
            </Box>
          </Grid>
        ))}
      </Grid>

      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 8 }}>
          <Typography variant="h4" sx={{ mb: 1.5 }}>CPU Usage</Typography>
          <MetricChart data={cpuSeries} color="#06b6d4" height={140} />

          <Typography variant="h4" sx={{ mt: 3, mb: 1.5 }}>Memory Usage</Typography>
          <MetricChart data={memSeries} color="#8b5cf6" height={140} />

          <Typography variant="h4" sx={{ mt: 3, mb: 1.5 }}>Network RX/TX</Typography>
          <MetricChart data={rxSeries} color="#10b981" height={140} />

          <Typography variant="h4" sx={{ mt: 3, mb: 1.5 }}>Disk Latency</Typography>
          <MetricChart data={diskSeries} color="#f59e0b" height={140} />
        </Grid>

        <Grid size={{ xs: 12, md: 4 }}>
          <Box sx={{ p: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
            <Typography variant="h4" sx={{ mb: 2 }}>System Info</Typography>
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
              <Box key={label} sx={{ display: 'flex', py: 0.5, borderBottom: `1px solid ${theme.palette.divider}` }}>
                <Typography variant="caption2" sx={{ minWidth: 120, color: theme.palette.text.secondary }}>
                  {label}
                </Typography>
                <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary }}>
                  {value}
                </Typography>
              </Box>
            ))}
          </Box>
        </Grid>
      </Grid>
    </Box>
  )
}
