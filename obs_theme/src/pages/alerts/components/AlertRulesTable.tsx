import { useState } from 'react'
import {
  Box, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper, Chip, Switch,
} from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { mockAlertRules, severityColors } from '../mockData'

export default function AlertRulesTable() {
  const theme = useTheme()
  const [rules, setRules] = useState(mockAlertRules)

  const toggleRule = (id: string) => {
    setRules((prev) => prev.map((r) => r.id === id ? { ...r, status: r.status === 'enabled' ? 'disabled' as const : 'enabled' as const } : r))
  }

  const notifIcons: Record<string, { label: string; color: string }> = {
    slack: { label: '#', color: '#10b981' },
    pagerduty: { label: 'P', color: '#10b981' },
    email: { label: '@', color: '#06b6d4' },
  }

  return (
    <TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell width={90} sx={headSx}>Severity</TableCell>
            <TableCell width={200} sx={headSx}>Name</TableCell>
            <TableCell width={120} sx={headSx}>Service</TableCell>
            <TableCell width={200} sx={headSx}>Condition</TableCell>
            <TableCell width={80} sx={headSx}>Status</TableCell>
            <TableCell width={120} sx={headSx}>Last Fired</TableCell>
            <TableCell width={180} sx={headSx}>Notifications</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rules.map((rule) => (
            <TableRow key={rule.id} hover sx={{ height: 36 }}>
              <TableCell sx={cellSx}>
                <Chip
                  label={rule.severity.toUpperCase()}
                  size="small"
                  sx={{
                    background: `${severityColors[rule.severity]}20`,
                    color: severityColors[rule.severity],
                    fontWeight: 600, fontSize: 10, height: 20, borderRadius: '3px',
                  }}
                />
              </TableCell>
              <TableCell sx={{ ...cellSx, fontWeight: 500 }}>{rule.name}</TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{rule.service}</TableCell>
              <TableCell sx={{ ...cellSx, fontFamily: theme.typography.mono.fontFamily }}>{rule.condition}</TableCell>
              <TableCell sx={cellSx}>
                <Switch
                  size="small"
                  checked={rule.status === 'enabled'}
                  onChange={() => toggleRule(rule.id)}
                  sx={{ '& .MuiSwitch-switchBase.Mui-checked': { color: '#10b981' }, '& .MuiSwitch-switchBase.Mui-checked + .MuiSwitch-track': { background: '#10b981' } }}
                />
              </TableCell>
              <TableCell sx={{ ...cellSx, color: theme.palette.text.secondary }}>{rule.lastFired}</TableCell>
              <TableCell sx={cellSx}>
                <Box sx={{ display: 'flex', gap: 0.5 }}>
                  {rule.notifications.map((n) => {
                    const icon = notifIcons[n]
                    if (!icon) return null
                    return (
                      <Chip
                        key={n}
                        label={icon.label}
                        size="small"
                        sx={{ background: `${icon.color}20`, color: icon.color, fontWeight: 600, fontSize: 10, height: 20, minWidth: 24 }}
                      />
                    )
                  })}
                </Box>
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
