import { useState } from 'react'
import {
  Box, Typography, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper, Drawer, IconButton,
} from '@mui/material'
import { X } from 'lucide-react'
import { useTheme } from '@mui/material/styles'
import { mockErrors } from '../mockData'

export default function ErrorTracking() {
  const theme = useTheme()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [selectedError, setSelectedError] = useState<(typeof mockErrors)[0] | null>(null)

  const openError = (err: (typeof mockErrors)[0]) => {
    setSelectedError(err)
    setDrawerOpen(true)
  }

  return (
    <Box>
      <TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell sx={headSx}>Error</TableCell>
              <TableCell sx={headSx}>Service</TableCell>
              <TableCell sx={headSx}>Count</TableCell>
              <TableCell sx={headSx}>Users</TableCell>
              <TableCell sx={headSx}>First Seen</TableCell>
              <TableCell sx={headSx}>Last Seen</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {mockErrors.map((err) => (
              <TableRow key={err.id} hover onClick={() => openError(err)} sx={{ cursor: 'pointer', height: 36 }}>
                <TableCell sx={{ ...cellSx, maxWidth: 280, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontFamily: theme.typography.mono.fontFamily }}>
                  {err.message}
                </TableCell>
                <TableCell sx={cellSx}>{err.service}</TableCell>
                <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{err.count.toLocaleString()}</TableCell>
                <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{err.users ?? '—'}</TableCell>
                <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary }}>{err.firstSeen}</TableCell>
                <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary }}>{err.lastSeen}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      <Drawer
        anchor="right" open={drawerOpen} onClose={() => setDrawerOpen(false)}
        slotProps={{ paper: { sx: { width: 420, background: theme.palette.background.paper, borderLeft: `1px solid ${theme.palette.divider}` } } }}
      >
        {selectedError && (
          <Box sx={{ p: 2 }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
              <Box>
                <Typography variant="h3" sx={{ fontSize: 14 }}>{selectedError.message}</Typography>
                <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
                  {selectedError.service} &bull; {selectedError.count.toLocaleString()} occurrences
                </Typography>
              </Box>
              <IconButton size="small" onClick={() => setDrawerOpen(false)} sx={{ color: theme.palette.text.secondary }}>
                <X size={18} />
              </IconButton>
            </Box>

            <Typography variant="h4" sx={{ mb: 1 }}>Stack Trace</Typography>
            <Box
              component="pre"
              sx={{
                fontFamily: theme.typography.mono.fontFamily,
                fontSize: 12,
                color: theme.palette.text.primary,
                bgcolor: 'background.default',
                borderRadius: '4px',
                p: 1.5,
                mb: 2,
                overflow: 'auto',
                height: 200,
              }}
            >
              {`Error: ${selectedError.message}
  at TCPConnectWrap.afterConnect [as oncomplete]
  at net.js:1141:16
  at processTicksAndRejections (node:internal/process/task_queues:78:11)`}
            </Box>

            <Typography variant="h4" sx={{ mb: 1 }}>Tags</Typography>
            {[
              ['error.type', 'ConnectionRefusedError'],
              ['db.system', 'redis'],
              ['net.peer', 'redis:6379'],
            ].map(([k, v]) => (
              <Box key={k} sx={{ display: 'flex', py: 0.25 }}>
                <Typography variant="caption2" sx={{ minWidth: 100, color: theme.palette.text.secondary }}>{k}</Typography>
                <Typography variant="caption2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary }}>{v}</Typography>
              </Box>
            ))}
          </Box>
        )}
      </Drawer>
    </Box>
  )
}

const headSx = {
  color: 'text.secondary', fontSize: '11px', fontWeight: 500, letterSpacing: '0.04em',
  textTransform: 'uppercase' as const, borderColor: 'divider', py: 1,
}
const cellSx = {
  fontSize: 13, color: 'text.primary', borderColor: 'divider', py: '7px',
}
