import { Box, Typography, Grid, Button } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import MetricChart from '../../metrics/components/MetricChart'
import type { ServiceData } from '../mockData'
import { mockServices, generateTimeSeries } from '../mockData'

interface ServiceOverviewCardsProps {
  onSelectService: (svc: ServiceData) => void
}

const statusColor: Record<string, string> = { healthy: '#10b981', degraded: '#f59e0b', down: '#ef4444' }

export default function ServiceOverviewCards({ onSelectService }: ServiceOverviewCardsProps) {
  const theme = useTheme()

  return (
    <Grid container spacing={2}>
      {mockServices.map((svc) => (
        <Grid key={svc.name} size={{ xs: 12, sm: 6, md: 4 }}>
          <Box
            sx={{
              p: 2,
              border: `1px solid ${theme.palette.divider}`,
              borderRadius: '4px',
              borderLeft: `3px solid ${statusColor[svc.status]}`,
              cursor: 'pointer',
            }}
          >
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Box sx={{ width: 8, height: 8, borderRadius: '50%', background: statusColor[svc.status] }} />
                <Typography variant="h4">{svc.name}</Typography>
              </Box>
              <Typography variant="caption2" sx={{ color: statusColor[svc.status], textTransform: 'uppercase' }}>
                {svc.status}
              </Typography>
            </Box>

            <Box sx={{ display: 'flex', gap: 3, mb: 1 }}>
              <Box>
                <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>Req/s</Typography>
                <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily }}>{svc.reqRate.toLocaleString()}</Typography>
              </Box>
              <Box>
                <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>Errors</Typography>
                <Typography variant="body2" sx={{
                  fontFamily: theme.typography.mono.fontFamily,
                  color: svc.errorRate > 5 ? '#ef4444' : svc.errorRate > 1 ? '#f59e0b' : theme.palette.text.primary,
                }}>
                  {svc.errorRate}%
                </Typography>
              </Box>
              <Box>
                <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>P50/P95</Typography>
                <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily }}>
                  {svc.p50}ms / {svc.p95 !== null ? `${svc.p95}ms` : '—'}
                </Typography>
              </Box>
            </Box>

            <MetricChart data={generateTimeSeries(svc.reqRate / 100, svc.reqRate / 500)} color={statusColor[svc.status]} height={40} showArea={false} />

            <Button size="small" sx={{ fontSize: 12, mt: 1 }} onClick={(e) => { e.stopPropagation(); onSelectService(svc) }}>
              View Service →
            </Button>
          </Box>
        </Grid>
      ))}
    </Grid>
  )
}
