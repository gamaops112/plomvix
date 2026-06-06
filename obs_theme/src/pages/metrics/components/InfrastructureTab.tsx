import { useState } from 'react'
import { Box } from '@mui/material'
import HostsColorCards from './HostsColorCards'
import HostsTable from './HostsTable'
import HostDetailDrawer from './HostDetailDrawer'
import type { Host } from '../mockData'

export default function InfrastructureTab() {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [selectedHost, setSelectedHost] = useState<Host | null>(null)

  const openHost = (host: Host) => {
    setSelectedHost(host)
    setDrawerOpen(true)
  }

  return (
    <Box>
      <Box sx={{ mb: 3 }}>
        <HostsColorCards />
      </Box>
      <HostsTable onSelectHost={openHost} />
      <HostDetailDrawer
        open={drawerOpen}
        host={selectedHost}
        onClose={() => setDrawerOpen(false)}
      />
    </Box>
  )
}
