import {
  Typography, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper, Chip,
} from '@mui/material'
import { useTheme } from '@mui/material/styles'
import type { FiringAlert } from '../mockData'
import { severityColors } from '../mockData'

interface FiringAlertsTableProps {
  alerts: FiringAlert[]
  onSelectAlert: (alert: FiringAlert) => void
}

export default function FiringAlertsTable({ alerts, onSelectAlert }: FiringAlertsTableProps) {
  const theme = useTheme()

  return (
    <TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell width={90} sx={headSx}>Severity</TableCell>
            <TableCell width={220} sx={headSx}>Alert Name</TableCell>
            <TableCell width={140} sx={headSx}>Service</TableCell>
            <TableCell width={200} sx={headSx}>Condition</TableCell>
            <TableCell width={100} sx={headSx}>Value</TableCell>
            <TableCell width={100} sx={headSx}>Duration</TableCell>
            <TableCell width={120} sx={headSx}>Started</TableCell>
            <TableCell width={120} sx={headSx}>Assignee</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {alerts.map((alert) => (
            <TableRow
              key={alert.id}
              hover
              onClick={() => onSelectAlert(alert)}
              sx={{ cursor: 'pointer', height: 36, opacity: alert.silenced ? 0.5 : 1 }}
            >
              <TableCell sx={cellSx}>
                <Chip
                  label={alert.severity.toUpperCase()}
                  size="small"
                  sx={{
                    background: `${severityColors[alert.severity]}20`,
                    color: severityColors[alert.severity],
                    fontWeight: 600,
                    fontSize: 10,
                    height: 20,
                    borderRadius: '3px',
                  }}
                />
              </TableCell>
              <TableCell sx={{ ...cellSx, color: '#06b6d4', fontWeight: 500 }}>{alert.name}</TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{alert.service}</TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{alert.condition}</TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily, color: severityColors[alert.severity] }}>
                {alert.value}
              </TableCell>
              <TableCell sx={cellSx}>
                {alert.silenced ? (
                  <Chip label="SILENCED" size="small" sx={{ background: '#1e2438', color: '#8b93a8', fontSize: 10, height: 20 }} />
                ) : (
                  <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary }}>
                    {alert.duration}
                  </Typography>
                )}
              </TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary }}>
                {alert.started}
              </TableCell>
              <TableCell sx={{ ...cellSx, color: theme.palette.text.secondary }}>
                {alert.assignee || 'Unassigned'}
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
