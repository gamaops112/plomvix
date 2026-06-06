import {
  Box, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper, Chip,
} from '@mui/material'
import { useTheme } from '@mui/material/styles'
import type { Incident } from '../mockData'
import { severityColors } from '../../alerts/mockData'

interface IncidentsListProps {
  incidents: Incident[]
  onSelect: (inc: Incident) => void
}

const statusChip: Record<string, { color: string; bg: string }> = {
  open: { color: '#ef4444', bg: '#ef444420' },
  investigating: { color: '#f59e0b', bg: '#f59e0b20' },
  resolved: { color: '#10b981', bg: '#10b98120' },
}

export default function IncidentsList({ incidents, onSelect }: IncidentsListProps) {
  const theme = useTheme()

  return (
    <TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell width={90} sx={headSx}>Severity</TableCell>
            <TableCell width={280} sx={headSx}>Title</TableCell>
            <TableCell width={120} sx={headSx}>Status</TableCell>
            <TableCell width={220} sx={headSx}>Affected Services</TableCell>
            <TableCell width={100} sx={headSx}>Started</TableCell>
            <TableCell width={80} sx={headSx}>Duration</TableCell>
            <TableCell width={100} sx={headSx}>Assignee</TableCell>
            <TableCell width={80} sx={headSx}>Alerts</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {incidents.map((inc) => (
            <TableRow key={inc.id} hover onClick={() => onSelect(inc)} sx={{ cursor: 'pointer', height: 36 }}>
              <TableCell sx={cellSx}>
                <Chip label={inc.severity.toUpperCase()} size="small"
                  sx={{ background: `${severityColors[inc.severity]}20`, color: severityColors[inc.severity], fontWeight: 600, fontSize: 10, height: 20, borderRadius: '3px' }} />
              </TableCell>
              <TableCell sx={{ ...cellSx, fontWeight: 500 }}>{inc.title}</TableCell>
              <TableCell sx={cellSx}>
                <Chip label={inc.status.toUpperCase()} size="small"
                  sx={{ background: statusChip[inc.status]?.bg || '#1e2438', color: statusChip[inc.status]?.color || '#8b93a8', fontWeight: 600, fontSize: 10, height: 20 }} />
              </TableCell>
              <TableCell sx={cellSx}>
                <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                  {inc.affectedServices.map((s) => (
                    <Chip key={s} label={s} size="small"
                      sx={{ background: '#1e2438', color: '#8b93a8', fontSize: 10, height: 20, borderRadius: '3px', fontFamily: theme.typography.mono.fontFamily }} />
                  ))}
                </Box>
              </TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary }}>{inc.startedAt}</TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{inc.duration}</TableCell>
              <TableCell sx={{ ...cellSx, color: theme.palette.text.secondary }}>{inc.assignee || 'Unassigned'}</TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{inc.alerts}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  )
}

const headSx = { color: 'text.secondary', fontSize: '11px', fontWeight: 500, letterSpacing: '0.04em', textTransform: 'uppercase' as const, borderColor: 'divider', py: 1 }
const cellSx = { fontSize: 13, color: 'text.primary', borderColor: 'divider', py: '7px' }
