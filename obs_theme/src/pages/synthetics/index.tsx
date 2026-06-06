import { useState, useMemo } from 'react'
import { Box, Typography, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper, Chip, Tabs, Tab, TextField, Select, MenuItem, Button, Tooltip, IconButton } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { Plus, Search, Pause, Pencil, Trash2 } from 'lucide-react'
import CreateMonitorModal from './components/CreateMonitorModal'
import MonitorDetailDrawer from './components/MonitorDetailDrawer'
import { mockIncidents } from '../incidents/mockData'
import { notify } from '../../lib/toast'

const mockMonitors = [
  { id: 'm1', name: 'API Gateway Health', url: 'https://api.obsadmin.io/health', type: 'HTTP', status: 'up', availability: 99.9, avgDuration: 45, lastCheck: '30s ago', frequency: '1m' },
  { id: 'm2', name: 'Auth Service Check', url: 'https://auth.obsadmin.io/ping', type: 'HTTP', status: 'up', availability: 100, avgDuration: 23, lastCheck: '45s ago', frequency: '1m' },
  { id: 'm3', name: 'Search Endpoint', url: 'https://api.obsadmin.io/search', type: 'HTTP', status: 'down', availability: 97.2, avgDuration: null, lastCheck: '1m ago', frequency: '1m' },
  { id: 'm4', name: 'Payment API', url: 'https://pay.obsadmin.io', type: 'HTTP', status: 'up', availability: 99.8, avgDuration: 67, lastCheck: '20s ago', frequency: '2m' },
  { id: 'm5', name: 'Database TCP Check', url: 'postgres:5432', type: 'TCP', status: 'up', availability: 99.9, avgDuration: 4, lastCheck: '1m ago', frequency: '1m' },
  { id: 'm6', name: 'Redis TCP Check', url: 'redis:6379', type: 'TCP', status: 'degraded', availability: 98.1, avgDuration: 892, lastCheck: '30s ago', frequency: '1m' },
  { id: 'm7', name: 'SSL Certificate', url: 'https://obsadmin.io', type: 'SSL', status: 'up', availability: 100, avgDuration: null, lastCheck: '1h ago', frequency: '1h' },
  { id: 'm8', name: 'DNS Resolution', url: 'obsadmin.io', type: 'DNS', status: 'up', availability: 100, avgDuration: 12, lastCheck: '5m ago', frequency: '5m' },
  { id: 'm9', name: 'Full Journey Check', url: 'https://obsadmin.io/login', type: 'Journey', status: 'degraded', availability: 96.4, avgDuration: 3421, lastCheck: '10m ago', frequency: '10m' },
  { id: 'm10', name: 'CDN Edge Check', url: 'https://cdn.obsadmin.io', type: 'HTTP', status: 'up', availability: 99.7, avgDuration: 18, lastCheck: '1m ago', frequency: '1m' },
]

const statusDot: Record<string, string> = { up: '#10b981', down: '#ef4444', degraded: '#f59e0b' }

const statusServices = [
  { name: 'API Gateway', status: 'operational' },
  { name: 'Authentication', status: 'operational' },
  { name: 'Search', status: 'outage' },
  { name: 'Payments', status: 'operational' },
  { name: 'Notifications', status: 'operational' },
]

const uptimeData = Array.from({ length: 90 }, (_, i) => {
  const date = new Date()
  date.setDate(date.getDate() - (89 - i))
  const rand = Math.random()
  return {
    date: date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }),
    uptime: rand > 0.95 ? +(90 + Math.random() * 5).toFixed(1) : rand > 0.85 ? +(95 + Math.random() * 4).toFixed(1) : 100,
    incidents: rand > 0.95 ? 1 : rand > 0.92 ? 2 : 0,
  }
})

export default function Synthetics() {
  const theme = useTheme()
  const [tab, setTab] = useState(0)
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState('all')
  const [statusFilter, setStatusFilter] = useState('all')
  const [createOpen, setCreateOpen] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [selectedMonitor, setSelectedMonitor] = useState<(typeof mockMonitors)[0] | null>(null)

  const filteredMonitors = useMemo(() => mockMonitors.filter((m) => {
    const matchSearch = m.name.toLowerCase().includes(search.toLowerCase()) || m.url.toLowerCase().includes(search.toLowerCase())
    const matchType = typeFilter === 'all' || m.type === typeFilter
    const matchStatus = statusFilter === 'all' || m.status === statusFilter
    return matchSearch && matchType && matchStatus
  }), [search, typeFilter, statusFilter])

  const counts = { up: filteredMonitors.filter((m) => m.status === 'up').length, down: filteredMonitors.filter((m) => m.status === 'down').length, degraded: filteredMonitors.filter((m) => m.status === 'degraded').length }

  const overallStatus = useMemo(() => {
    const hasOutage = statusServices.some((s) => s.status === 'outage')
    const hasDegraded = statusServices.some((s) => s.status === 'degraded')
    if (hasOutage) return { label: 'Major Outage', color: '#ef4444' }
    if (hasDegraded) return { label: 'Partial Degradation', color: '#f59e0b' }
    return { label: 'All Systems Operational', color: '#10b981' }
  }, [])

  const recentIncidents = mockIncidents.slice(0, 3)

  const openMonitor = (m: (typeof mockMonitors)[0]) => { setSelectedMonitor(m); setDrawerOpen(true) }

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h2" sx={{ mb: 2 }}>Synthetics</Typography>
      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 3 }}>
        <Tab label="Monitors" /><Tab label="Status Page" />
      </Tabs>

      {tab === 0 && (
        <Box>
          <Box sx={{ display: 'flex', gap: 2, mb: 3, flexWrap: 'wrap' }}>
            {Object.entries(counts).map(([key, count]) => (
              <Box key={key} sx={{ display: 'flex', alignItems: 'center', gap: 1, px: 1.5, py: 0.5, background: `${statusDot[key]}15`, border: `1px solid ${statusDot[key]}40`, borderRadius: '4px' }}>
                <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: statusDot[key] }} />
                <Typography variant="caption2" sx={{ color: 'text.secondary', textTransform: 'capitalize' }}>{key}</Typography>
                <Typography variant="body2" sx={{ fontWeight: 600, color: statusDot[key] }}>{count}</Typography>
              </Box>
            ))}
          </Box>

          <Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center', flexWrap: 'wrap' }}>
            <TextField size="small" placeholder="Search monitors..." value={search} onChange={(e) => setSearch(e.target.value)}
              slotProps={{ input: { startAdornment: <Search size={14} color="#8b93a8" style={{ marginRight: 6 }} /> } }} sx={{ width: 280 }} />
            <Select size="small" value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)} sx={{ fontSize: 13, minWidth: 130 }}>
              <MenuItem value="all">All Types</MenuItem>
              <MenuItem value="HTTP">HTTP</MenuItem>
              <MenuItem value="TCP">TCP</MenuItem>
              <MenuItem value="SSL">SSL</MenuItem>
              <MenuItem value="DNS">DNS</MenuItem>
              <MenuItem value="Journey">Journey</MenuItem>
            </Select>
            <Select size="small" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} sx={{ fontSize: 13, minWidth: 130 }}>
              <MenuItem value="all">All Status</MenuItem>
              <MenuItem value="up">Up</MenuItem>
              <MenuItem value="down">Down</MenuItem>
              <MenuItem value="degraded">Degraded</MenuItem>
            </Select>
            <Box sx={{ flex: 1 }} />
            <Button variant="contained" size="small" startIcon={<Plus size={14} />} onClick={() => setCreateOpen(true)}>
              Create Monitor
            </Button>
          </Box>

          <TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell width={32} sx={headSx} />
                  <TableCell width={200} sx={headSx}>Name</TableCell>
                  <TableCell width={240} sx={headSx}>URL</TableCell>
                  <TableCell width={90} sx={headSx}>Type</TableCell>
                  <TableCell width={120} sx={headSx}>Availability</TableCell>
                  <TableCell width={120} sx={headSx}>Avg Duration</TableCell>
                  <TableCell width={100} sx={headSx}>Last Check</TableCell>
                  <TableCell width={90} sx={headSx}>Frequency</TableCell>
                  <TableCell width={100} align="right" sx={headSx}>Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {filteredMonitors.map((m) => (
                  <TableRow key={m.id} hover onClick={() => openMonitor(m)} sx={{ cursor: 'pointer', height: 36 }}>
                    <TableCell sx={cellSx}><Box sx={{ width: 6, height: 6, borderRadius: '50%', background: statusDot[m.status] }} /></TableCell>
                    <TableCell sx={{ ...cellSx, fontWeight: 500 }}>{m.name}</TableCell>
                    <TableCell sx={{ ...cellSx, width: 240, maxWidth: 240 }}>
                      <Tooltip title={m.url}>
                        <Typography sx={{ fontFamily: theme.typography.mono.fontFamily, color: 'text.secondary', fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 220 }}>
                          {m.url}
                        </Typography>
                      </Tooltip>
                    </TableCell>
                    <TableCell sx={cellSx}><Chip label={m.type} size="small" sx={{ background: '#1e2438', color: 'text.secondary', fontSize: 10, height: 20 }} /></TableCell>
                    <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily, color: m.availability < 98 ? '#ef4444' : '#10b981' }}>{m.availability}%</TableCell>
                    <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{m.avgDuration !== null ? `${m.avgDuration}ms` : '—'}</TableCell>
                    <TableCell sx={{ ...cellSx, color: 'text.secondary' }}>{m.lastCheck}</TableCell>
                    <TableCell sx={{ ...cellSx, color: 'text.secondary' }}>{m.frequency}</TableCell>
                    <TableCell align="right" sx={cellSx}>
                      <Box onClick={(e) => e.stopPropagation()} sx={{ display: 'flex', gap: 0.5, justifyContent: 'flex-end', opacity: 0, '.MuiTableRow-root:hover &': { opacity: 1 } }}>
                        <Tooltip title="Pause monitor"><IconButton size="small" onClick={() => notify.success('Monitor paused')}><Pause size={14} /></IconButton></Tooltip>
                        <Tooltip title="Edit monitor"><IconButton size="small" onClick={() => setCreateOpen(true)}><Pencil size={14} /></IconButton></Tooltip>
                        <Tooltip title="Delete monitor"><IconButton size="small" onClick={() => notify.success('Monitor deleted')}><Trash2 size={14} color="#ef4444" /></IconButton></Tooltip>
                      </Box>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Box>
      )}

      {tab === 1 && (
        <Box>
          <Box sx={{ p: 3, mb: 3, border: 1, borderColor: 'divider', borderRadius: '4px', textAlign: 'center' }}>
            <Typography variant="h2" sx={{ mb: 1 }}>obsAdmin Status</Typography>
            <Typography variant="body2" sx={{ color: 'text.secondary', mb: 2 }}>Real-time status of obsAdmin services</Typography>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, px: 2, py: 0.5, borderRadius: '20px', background: `${overallStatus.color}15`, border: `1px solid ${overallStatus.color}40` }}>
                <Box sx={{ width: 8, height: 8, borderRadius: '50%', background: overallStatus.color }} />
                <Typography variant="body2" sx={{ fontWeight: 500, color: overallStatus.color }}>{overallStatus.label}</Typography>
              </Box>
              <Typography variant="caption" sx={{ color: 'text.disabled', mt: 1, display: 'block' }}>Last updated: just now</Typography>
          </Box>

          <Typography variant="h4" sx={{ mb: 2 }}>Services</Typography>
          {statusServices.map((svc) => (
            <Box key={svc.name} sx={{ display: 'flex', alignItems: 'center', gap: 1.5, py: 1, borderBottom: 1, borderColor: 'divider' }}>
              <Box sx={{ width: 8, height: 8, borderRadius: '50%', background: svc.status === 'operational' ? '#10b981' : svc.status === 'outage' ? '#ef4444' : '#f59e0b' }} />
              <Typography variant="body2">{svc.name}</Typography>
              <Typography variant="caption2" sx={{ color: svc.status === 'operational' ? '#10b981' : svc.status === 'outage' ? '#ef4444' : '#f59e0b', ml: 'auto' }}>
                {svc.status === 'operational' ? 'Operational' : svc.status === 'outage' ? 'Major Outage' : 'Degraded'}
              </Typography>
            </Box>
          ))}

          <Box sx={{ mt: 4 }}>
            <Typography variant="h4" sx={{ mb: 2 }}>Recent Incidents</Typography>
            {recentIncidents.map((inc) => (
              <Box key={inc.id} sx={{ borderLeft: `3px solid ${inc.severity === 'critical' ? '#ef4444' : inc.severity === 'high' ? '#f97316' : '#f59e0b'}`, pl: 2, py: 1, mb: 1, bgcolor: 'background.paper', borderRadius: '0 4px 4px 0', border: 1, borderColor: 'divider' }}>
                <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                  <Typography variant="body2" sx={{ fontWeight: 500 }}>{inc.title}</Typography>
                  <Typography variant="caption" sx={{ color: 'text.secondary' }}>{inc.startedAt}</Typography>
                </Box>
                <Typography variant="caption" sx={{ color: 'text.secondary' }}>
                  {inc.status === 'resolved' ? '✓ Resolved' : '⚠ Ongoing'} &bull; {inc.affectedServices.join(', ')}
                </Typography>
              </Box>
            ))}
          </Box>

          <Box sx={{ mt: 4 }}>
            <Typography variant="h4" sx={{ mb: 2 }}>90-Day Uptime</Typography>
            <Box sx={{ display: 'flex', gap: 0.25, flexWrap: 'wrap' }}>
              {uptimeData.map((day, i) => (
                <Tooltip key={i} title={<Box><Typography variant="caption" sx={{ display: 'block' }}>{day.date}</Typography><Typography variant="caption" sx={{ display: 'block' }}>Uptime: {day.uptime}%</Typography>{day.incidents > 0 && <Typography variant="caption" sx={{ display: 'block', color: '#ef4444' }}>{day.incidents} incident{day.incidents > 1 ? 's' : ''}</Typography>}</Box>} arrow>
                  <Box sx={{ width: 14, height: 14, borderRadius: '2px', background: day.uptime === 100 ? '#10b981' : day.uptime >= 95 ? '#f59e0b' : '#ef4444', cursor: 'pointer', '&:hover': { opacity: 0.8, transform: 'scale(1.2)' }, transition: 'transform 0.1s' }} />
                </Tooltip>
              ))}
            </Box>
          </Box>
        </Box>
      )}

      <CreateMonitorModal open={createOpen} onClose={() => setCreateOpen(false)} />
      <MonitorDetailDrawer open={drawerOpen} monitor={selectedMonitor} onClose={() => setDrawerOpen(false)} />
    </Box>
  )
}

const headSx = { color: 'text.secondary', fontSize: '11px', fontWeight: 500, letterSpacing: '0.04em', textTransform: 'uppercase' as const, borderColor: 'divider', py: 1 }
const cellSx = { fontSize: 13, color: 'text.primary', borderColor: 'divider', py: '7px' }
