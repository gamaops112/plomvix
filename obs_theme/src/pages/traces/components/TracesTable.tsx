import {
  Box, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper,
} from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { useNavigate } from 'react-router-dom'
import type { Trace } from '../mockData'

interface TracesTableProps {
  traces: Trace[]
  onSelectTrace: (trace: Trace) => void
}

const statusDot: Record<string, string> = {
  error: '#ef4444',
  slow: '#f59e0b',
  ok: '#10b981',
}

export default function TracesTable({ traces, onSelectTrace }: TracesTableProps) {
  const theme = useTheme()
  const navigate = useNavigate()

  return (
    <TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell width={32} sx={{ borderColor: 'divider' }} />
            <TableCell width={100} sx={headSx}>Trace ID</TableCell>
            <TableCell width={140} sx={headSx}>Root Service</TableCell>
            <TableCell width={200} sx={headSx}>Root Operation</TableCell>
            <TableCell width={100} sx={headSx}>Duration</TableCell>
            <TableCell width={72} sx={headSx}>Spans</TableCell>
            <TableCell width={72} sx={headSx}>Errors</TableCell>
            <TableCell width={140} sx={headSx}>Start Time</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {traces.map((trace) => (
            <TableRow
              key={trace.id}
              hover
              onClick={() => onSelectTrace(trace)}
              sx={{ cursor: 'pointer', height: 36 }}
            >
              <TableCell sx={{ ...cellSx, py: 0 }}>
                <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: statusDot[trace.status] }} />
              </TableCell>
              <TableCell
                onClick={(e) => { e.stopPropagation(); navigate(`/traces/${trace.id}`) }}
                sx={{ ...cellSx, color: '#06b6d4', fontFamily: theme.typography.mono.fontFamily, cursor: 'pointer' }}
              >
                {trace.id.substring(0, 8)}
              </TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{trace.rootService}</TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{trace.rootOp}</TableCell>
              <TableCell sx={{
                ...cellSx,
                fontFamily: theme.typography.mono.fontFamily,
                color: trace.duration > 1000 ? '#ef4444' : theme.palette.text.primary,
              }}>
                {trace.duration > 1000 ? `${(trace.duration / 1000).toFixed(1)}s` : `${trace.duration}ms`}
              </TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{trace.spans}</TableCell>
              <TableCell sx={{
                ...cellSx,
                fontFamily: theme.typography.mono.fontFamily,
                color: trace.errors > 0 ? '#ef4444' : theme.palette.text.disabled,
              }}>
                {trace.errors > 0 ? trace.errors : '—'}
              </TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary }}>
                {trace.time}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  )
}

const headSx = { color: 'text.secondary', fontSize: '11px', fontWeight: 500, letterSpacing: '0.04em', textTransform: 'uppercase' as const, borderColor: 'divider', py: 1 }
const cellSx = { fontSize: 13, color: 'text.primary', borderColor: 'divider', py: '7px' }
