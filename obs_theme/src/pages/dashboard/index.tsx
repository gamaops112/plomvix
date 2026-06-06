import { useState, useEffect } from 'react'
import { Box, Typography, Select, MenuItem, Button, Grid } from '@mui/material'
import { Plus } from 'lucide-react'
import { notify } from '../../lib/toast'
import PageSkeleton from '../../components/common/PageSkeleton'
import ColorMetricCards from './components/ColorMetricCards'
import StatTiles from './components/StatTiles'
import TimeSeriesChart from './components/TimeSeriesChart'
import ServiceHealthGrid from './components/ServiceHealthGrid'
import ServiceMap from './components/ServiceMap'
import RecentAlerts from './components/RecentAlerts'
import LogsPreview from './components/LogsPreview'
import TracesPreview from './components/TracesPreview'

export default function Dashboard() {
  const [loading, setLoading] = useState(true)
  useEffect(() => { const t = setTimeout(() => setLoading(false), 1200); return () => clearTimeout(t) }, [])

  if (loading) return <PageSkeleton variant="dashboard" />

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'flex-start', mb: 3, flexWrap: 'wrap', gap: 2 }}>
        <Box sx={{ flex: 1, minWidth: 200 }}>
          <Typography variant="h2">Dashboard</Typography>
          <Select defaultValue="production" size="small" sx={{ mt: 0.5, fontSize: 13, height: 28 }}>
            <MenuItem value="production">Environment: production</MenuItem>
            <MenuItem value="staging">Environment: staging</MenuItem>
            <MenuItem value="development">Environment: development</MenuItem>
          </Select>
        </Box>
        <Box sx={{ display: 'flex', gap: 1, flexShrink: 0 }}>
          <Button variant="outlined" size="small" startIcon={<Plus size={14} />} onClick={() => notify.info('Widget library coming soon')}>
            Add Widget
          </Button>
          <Button variant="outlined" size="small" onClick={() => notify.info('Dashboard editing coming soon')}>
            Edit Dashboard
          </Button>
        </Box>
      </Box>

      <Grid container spacing={2}>
        <Grid size={12}><ColorMetricCards /></Grid>
        <Grid size={12}><StatTiles /></Grid>
        <Grid size={{ xs: 12, md: 8 }}><TimeSeriesChart /></Grid>
        <Grid size={{ xs: 12, md: 4 }}><ServiceHealthGrid /></Grid>
        <Grid size={{ xs: 12, md: 7 }}><ServiceMap /></Grid>
        <Grid size={{ xs: 12, md: 5 }}><RecentAlerts /></Grid>
        <Grid size={{ xs: 12, md: 6 }}><LogsPreview /></Grid>
        <Grid size={{ xs: 12, md: 6 }}><TracesPreview /></Grid>
      </Grid>
    </Box>
  )
}
