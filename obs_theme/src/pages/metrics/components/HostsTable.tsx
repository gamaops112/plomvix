import { useState } from 'react'
import {
  Box, Typography, Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Select, MenuItem, ToggleButtonGroup, ToggleButton, Paper,
} from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { ExternalLink } from 'lucide-react'
import type { Host } from '../mockData'
import { mockHosts, getUsageColor } from '../mockData'

interface HostsTableProps {
  onSelectHost: (host: Host) => void
}

export default function HostsTable({ onSelectHost }: HostsTableProps) {
  const theme = useTheme()
  const [osFilter, setOsFilter] = useState('Any')
  const [providerFilter, setProviderFilter] = useState('Any')
  const [limit, setLimit] = useState('100')

  const filteredHosts = mockHosts.filter((h) => {
    if (osFilter !== 'Any' && h.os !== osFilter) return false
    return true
  }).slice(0, parseInt(limit))

  return (
    <Box>
      <Box sx={{ display: 'flex', gap: 2, mb: 2 }}>
        <Select value={osFilter} onChange={(e) => setOsFilter(e.target.value)} size="small" sx={{ fontSize: 13, minWidth: 160 }}>
          <MenuItem value="Any">Operating System: Any</MenuItem>
          <MenuItem value="Ubuntu">Ubuntu</MenuItem>
          <MenuItem value="CentOS">CentOS</MenuItem>
          <MenuItem value="Debian">Debian</MenuItem>
        </Select>
        <Select value={providerFilter} onChange={(e) => setProviderFilter(e.target.value)} size="small" sx={{ fontSize: 13, minWidth: 160 }}>
          <MenuItem value="Any">Cloud Provider: Any</MenuItem>
          <MenuItem value="GCP">GCP</MenuItem>
          <MenuItem value="AWS">AWS</MenuItem>
        </Select>
        <ToggleButtonGroup
          value={limit}
          exclusive
          onChange={(_, v) => v && setLimit(v)}
          size="small"
          sx={{ ml: 'auto' }}
        >
          <ToggleButton value="10" sx={{ fontSize: 12, px: 1.5, color: theme.palette.text.secondary }}>10</ToggleButton>
          <ToggleButton value="20" sx={{ fontSize: 12, px: 1.5, color: theme.palette.text.secondary }}>20</ToggleButton>
          <ToggleButton value="50" sx={{ fontSize: 12, px: 1.5, color: theme.palette.text.secondary }}>50</ToggleButton>
          <ToggleButton value="100" sx={{ fontSize: 12, px: 1.5, color: theme.palette.text.secondary }}>100</ToggleButton>
          <ToggleButton value="500" sx={{ fontSize: 12, px: 1.5, color: theme.palette.text.secondary }}>500</ToggleButton>
        </ToggleButtonGroup>
      </Box>

      <TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell width={32} />
              <TableCell width={200} sx={headSx}>Name</TableCell>
              <TableCell width={140} sx={headSx}>Operating System</TableCell>
              <TableCell width={130} sx={headSx}>CPU usage (avg)</TableCell>
              <TableCell width={130} sx={headSx}>Disk Latency (avg)</TableCell>
              <TableCell width={110} sx={headSx}>RX (avg)</TableCell>
              <TableCell width={110} sx={headSx}>TX (avg)</TableCell>
              <TableCell width={140} sx={headSx}>Memory total (avg)</TableCell>
              <TableCell width={140} sx={headSx}>Memory usage (avg)</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {filteredHosts.map((host) => (
              <TableRow
                key={host.id}
                hover
                onClick={() => onSelectHost(host)}
                sx={{ cursor: 'pointer', height: 36 }}
              >
                <TableCell sx={cellSx}>
                  <ExternalLink size={12} color={theme.palette.text.secondary} />
                </TableCell>
                <TableCell sx={{ ...cellSx, color: '#06b6d4', fontFamily: theme.typography.mono.fontFamily }}>
                  {host.name}
                </TableCell>
                <TableCell sx={cellSx}>{host.os || '—'}</TableCell>
                <TableCell sx={cellSx}>
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.25 }}>
                    <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily }}>
                      {host.cpu}%
                    </Typography>
                    <Box sx={{ width: '100%', height: 3, borderRadius: 1, bgcolor: 'divider' }}>
                      <Box sx={{ width: `${Math.min(host.cpu, 100)}%`, height: 3, borderRadius: 1, background: getUsageColor(host.cpu) }} />
                    </Box>
                  </Box>
                </TableCell>
                <TableCell sx={cellSx}>{host.diskLatency > 0 ? `${host.diskLatency} ms` : '—'}</TableCell>
                <TableCell sx={cellSx}>{host.rx > 0 ? `${host.rx} Mbit/s` : '—'}</TableCell>
                <TableCell sx={cellSx}>{host.tx > 0 ? `${host.tx} Mbit/s` : '—'}</TableCell>
                <TableCell sx={cellSx}>
                  {host.memTotal > 0 ? `${host.memTotal} GB` : '—'}
                </TableCell>
                <TableCell sx={cellSx}>
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.25 }}>
                    <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily }}>
                      {host.memUsage > 0 ? `${host.memUsage}%` : '—'}
                    </Typography>
                    {host.memUsage > 0 && (
                      <Box sx={{ width: '100%', height: 3, borderRadius: 1, bgcolor: 'divider' }}>
                        <Box sx={{ width: `${Math.min(host.memUsage, 100)}%`, height: 3, borderRadius: 1, background: getUsageColor(host.memUsage) }} />
                      </Box>
                    )}
                  </Box>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  )
}

const headSx = { color: 'text.secondary', fontSize: '11px', fontWeight: 500, letterSpacing: '0.04em', textTransform: 'uppercase' as const, borderColor: 'divider', py: 1 }
const cellSx = { fontSize: 13, color: 'text.primary', borderColor: 'divider', py: '7px' }
