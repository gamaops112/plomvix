import { useState } from 'react'
import { Box, Typography, Button, TextField } from '@mui/material'
import { Plus, Search } from 'lucide-react'
import IncidentsList from './components/IncidentsList'
import IncidentDetailDrawer from './components/IncidentDetailDrawer'
import { mockIncidents } from './mockData'
import type { Incident } from './mockData'

export default function Incidents() {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [selectedIncident, setSelectedIncident] = useState<Incident | null>(null)

  const openIncident = (inc: Incident) => {
    setSelectedIncident(inc)
    setDrawerOpen(true)
  }

  const counts = { open: 1, investigating: 2, resolved: 4 }

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Typography variant="h2">Incidents</Typography>
        <Button variant="outlined" size="small" startIcon={<Plus size={14} />} sx={{ fontSize: 13 }}>
          Create Incident
        </Button>
      </Box>

      <Box sx={{ display: 'flex', gap: 1.5, mb: 2, flexWrap: 'wrap' }}>
        {(Object.entries(counts) as Array<[string, number]>).map(([key, count]) => {
          const color = key === 'open' ? '#ef4444' : key === 'investigating' ? '#f59e0b' : '#10b981'
          return (
            <Box key={key} sx={{ display: 'flex', alignItems: 'center', gap: 1, px: 1.5, py: 0.5, background: `${color}15`, border: `1px solid ${color}40`, borderRadius: '4px' }}>
              <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: color }} />
              <Typography variant="caption2" sx={{ color: '#8b93a8', textTransform: 'capitalize' }}>{key}</Typography>
              <Typography variant="body2" sx={{ fontWeight: 600, color }}>{count}</Typography>
            </Box>
          )
        })}
      </Box>

      <Box sx={{ display: 'flex', gap: 1.5, mb: 2 }}>
        <TextField size="small" placeholder="Search incidents..."
          slotProps={{ input: { startAdornment: <Search size={14} color="#4d566b" style={{ marginRight: 6 }} /> } }}
          sx={{ width: 280 }} />
      </Box>

      <IncidentsList incidents={mockIncidents} onSelect={openIncident} />
      <IncidentDetailDrawer open={drawerOpen} incident={selectedIncident} onClose={() => setDrawerOpen(false)} />
    </Box>
  )
}
