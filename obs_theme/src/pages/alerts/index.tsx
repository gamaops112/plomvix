import { useState, useMemo } from 'react'
import { Box, Typography, Tabs, Tab, TextField, Button, Select, MenuItem } from '@mui/material'
import { Plus, Search } from 'lucide-react'
import AlertsSummaryBar from './components/AlertsSummaryBar'
import FiringAlertsTable from './components/FiringAlertsTable'
import AlertRulesTable from './components/AlertRulesTable'
import AlertDetailDrawer from './components/AlertDetailDrawer'
import CreateAlertModal from './components/CreateAlertModal'
import NotificationChannels from './components/NotificationChannels'
import { mockFiringAlerts } from './mockData'
import type { FiringAlert } from './mockData'

export default function Alerts() {
  const [tab, setTab] = useState(0)
  const [search, setSearch] = useState('')
  const [sevFilter, setSevFilter] = useState('All')
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [selectedAlert, setSelectedAlert] = useState<FiringAlert | null>(null)
  const [createOpen, setCreateOpen] = useState(false)

  const filtered = useMemo(() => {
    let r = mockFiringAlerts
    if (sevFilter !== 'All') r = r.filter((a) => a.severity === sevFilter)
    if (search) r = r.filter((a) => a.name.toLowerCase().includes(search.toLowerCase()))
    return r
  }, [sevFilter, search])

  const openAlert = (alert: FiringAlert) => {
    setSelectedAlert(alert)
    setDrawerOpen(true)
  }

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h2" sx={{ mb: 2 }}>Alerts</Typography>
      <AlertsSummaryBar />

      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 2 }}>
        <Tab label="Firing Alerts" />
        <Tab label="Alert Rules" />
        <Tab label="Notification Channels" />
      </Tabs>

      {tab === 0 && (
        <Box>
          <Box sx={{ display: 'flex', gap: 1.5, mb: 2, flexWrap: 'wrap' }}>
            <TextField size="small" placeholder="Search alerts..." value={search} onChange={(e) => setSearch(e.target.value)}
              slotProps={{ input: { startAdornment: <Search size={14} color="#4d566b" style={{ marginRight: 6 }} /> } }} sx={{ width: 240 }} />
            <Select value={sevFilter} onChange={(e) => setSevFilter(e.target.value)} size="small" sx={{ fontSize: 13, minWidth: 140 }}>
              <MenuItem value="All">Severity: All</MenuItem>
              <MenuItem value="critical">Critical</MenuItem>
              <MenuItem value="high">High</MenuItem>
              <MenuItem value="warning">Warning</MenuItem>
              <MenuItem value="info">Info</MenuItem>
            </Select>
            <Box sx={{ flex: 1 }} />
            <Button variant="outlined" size="small" startIcon={<Plus size={14} />} onClick={() => setCreateOpen(true)} sx={{ fontSize: 13 }}>
              Create Alert Rule
            </Button>
          </Box>
          <FiringAlertsTable alerts={filtered} onSelectAlert={openAlert} />
        </Box>
      )}

      {tab === 1 && (
        <Box>
          <Box sx={{ display: 'flex', gap: 1.5, mb: 2 }}>
            <TextField size="small" placeholder="Search rules..." sx={{ width: 240 }} />
            <Box sx={{ flex: 1 }} />
            <Button variant="outlined" size="small" startIcon={<Plus size={14} />} onClick={() => setCreateOpen(true)} sx={{ fontSize: 13 }}>
              Create Rule
            </Button>
          </Box>
          <AlertRulesTable />
        </Box>
      )}

      {tab === 2 && <NotificationChannels />}

      <AlertDetailDrawer open={drawerOpen} alert={selectedAlert} onClose={() => setDrawerOpen(false)} />
      <CreateAlertModal open={createOpen} onClose={() => setCreateOpen(false)} />
    </Box>
  )
}
