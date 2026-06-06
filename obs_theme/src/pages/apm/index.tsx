import { useState } from 'react'
import { Box, Typography, Select, MenuItem } from '@mui/material'
import ServiceOverviewCards from './components/ServiceOverviewCards'
import ApmServiceMap from './components/ApmServiceMap'
import ErrorTracking from './components/ErrorTracking'
import LatencyChart from './components/LatencyChart'
import type { ServiceData } from './mockData'
import { generateTimeSeries } from './mockData'
import MetricChart from '../metrics/components/MetricChart'
import { useTheme } from '@mui/material/styles'

export default function APM() {
  const theme = useTheme()
  const [selectedService, setSelectedService] = useState<ServiceData | null>(null)
  const [env, setEnv] = useState('production')

  const handleSelectService = (svc: ServiceData) => {
    setSelectedService(selectedService?.name === svc.name ? null : svc)
  }

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
        <Typography variant="h2">APM — Application Performance</Typography>
        <Select value={env} onChange={(e) => setEnv(e.target.value)} size="small" sx={{ fontSize: 13 }}>
          <MenuItem value="production">Environment: production</MenuItem>
          <MenuItem value="staging">Environment: staging</MenuItem>
        </Select>
      </Box>

      <Typography variant="h4" sx={{ mb: 2 }}>Services</Typography>
      <ServiceOverviewCards onSelectService={handleSelectService} />

      {selectedService && (
        <Box sx={{ mt: 3, p: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
            <Typography variant="h3">{selectedService.name}</Typography>
            <Typography variant="caption2" sx={{
              fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary,
            }}>
              {selectedService.instances} instances
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 3, mb: 2 }}>
            <Box>
              <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>Req/s</Typography>
              <Typography variant="metricSm">{selectedService.reqRate.toLocaleString()}</Typography>
            </Box>
            <Box>
              <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>Error Rate</Typography>
              <Typography variant="metricSm" sx={{ color: selectedService.errorRate > 5 ? '#ef4444' : selectedService.errorRate > 1 ? '#f59e0b' : theme.palette.text.primary }}>
                {selectedService.errorRate}%
              </Typography>
            </Box>
            <Box>
              <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>P50 / P95 / P99</Typography>
              <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily }}>
                {selectedService.p50}ms / {selectedService.p95 !== null ? `${selectedService.p95}ms` : '—'} / {selectedService.p99 !== null ? `${selectedService.p99}ms` : '—'}
              </Typography>
            </Box>
          </Box>
          <MetricChart data={generateTimeSeries(selectedService.p50, selectedService.p50 / 3)} color="#06b6d4" height={120} />
        </Box>
      )}

      <Typography variant="h4" sx={{ mt: 4, mb: 2 }}>Service Map</Typography>
      <Box sx={{ border: `1px solid ${theme.palette.divider}`, borderRadius: '4px', overflow: 'hidden' }}>
        <ApmServiceMap />
      </Box>

      <Typography variant="h4" sx={{ mt: 4, mb: 2 }}>Latency</Typography>
      <Box sx={{ p: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
        <LatencyChart />
      </Box>

      <Typography variant="h4" sx={{ mt: 4, mb: 2 }}>Error Tracking</Typography>
      <ErrorTracking />
    </Box>
  )
}
