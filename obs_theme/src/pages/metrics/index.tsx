import { useState } from 'react'
import { Box, Typography, Tabs, Tab } from '@mui/material'
import InfrastructureTab from './components/InfrastructureTab'
import MetricsExplorerTab from './components/MetricsExplorerTab'

export default function Metrics() {
  const [tab, setTab] = useState(0)

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h2" sx={{ mb: 2 }}>Metrics & Infrastructure</Typography>

      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 3 }}>
        <Tab label="Infrastructure" />
        <Tab label="Metrics Explorer" />
      </Tabs>

      {tab === 0 && <InfrastructureTab />}
      {tab === 1 && <MetricsExplorerTab />}
    </Box>
  )
}
