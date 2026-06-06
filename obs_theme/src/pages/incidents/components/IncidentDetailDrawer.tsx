import { useState } from 'react'
import {
  Box, Typography, Drawer, Tabs, Tab, IconButton, Button, Chip,
} from '@mui/material'
import { X, ExternalLink } from 'lucide-react'
import { useTheme } from '@mui/material/styles'
import { useNavigate } from 'react-router-dom'
import type { Incident } from '../mockData'
import { severityColors } from '../../alerts/mockData'
import IncidentTimeline from './IncidentTimeline'

interface IncidentDetailDrawerProps {
  open: boolean
  incident: Incident | null
  onClose: () => void
}

export default function IncidentDetailDrawer({ open, incident, onClose }: IncidentDetailDrawerProps) {
  const theme = useTheme()
  const navigate = useNavigate()
  const [tab, setTab] = useState(0)

  if (!incident) return null

  return (
    <Drawer anchor="right" open={open} onClose={onClose}
      slotProps={{ paper: { sx: { width: 560, background: theme.palette.background.paper, borderLeft: `1px solid ${theme.palette.divider}` } } }}>
      <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        <Box sx={{ px: 2, py: 1.5, borderBottom: `1px solid ${theme.palette.divider}` }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                <Chip label={incident.severity.toUpperCase()} size="small"
                  sx={{ background: `${severityColors[incident.severity]}20`, color: severityColors[incident.severity], fontWeight: 600, fontSize: 10, height: 20 }} />
                <Typography variant="caption2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary }}>
                  INC-{incident.id}
                </Typography>
              </Box>
              <Typography variant="h3">{incident.title}</Typography>
              <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, textTransform: 'capitalize' }}>
                {incident.status} &bull; started {incident.startedAt} &bull; {incident.duration}
              </Typography>
            </Box>
            <Box sx={{ display: 'flex', gap: 1 }}>
              <Button size="small" startIcon={<ExternalLink size={14} />} sx={{ fontSize: 12 }}
                onClick={() => { onClose(); navigate(`/incidents/${incident.id}`) }}>Open full page →</Button>
              <IconButton size="small" onClick={onClose} sx={{ color: theme.palette.text.secondary }}><X size={18} /></IconButton>
            </Box>
          </Box>
          <Box sx={{ display: 'flex', gap: 1, mt: 1.5 }}>
            <Button variant="outlined" size="small" sx={{ fontSize: 12 }}>Acknowledge</Button>
            <Button variant="outlined" size="small" sx={{ fontSize: 12, color: '#10b981', borderColor: '#10b981' }}>Resolve</Button>
            <Button variant="outlined" size="small" sx={{ fontSize: 12 }}>Create Postmortem</Button>
          </Box>
        </Box>

        <Tabs value={tab} onChange={(_, v) => setTab(v)}>
          <Tab label="Overview" />
          <Tab label="Timeline" />
          <Tab label="Alerts" />
          <Tab label="Runbook" />
        </Tabs>

        <Box sx={{ flex: 1, overflow: 'auto' }}>
          {tab === 0 && (
            <Box sx={{ p: 2 }}>
              <Typography variant="h4" sx={{ mb: 1 }}>Affected Services</Typography>
              <Box sx={{ display: 'flex', gap: 0.5, mb: 2, flexWrap: 'wrap' }}>
                {incident.affectedServices.map((s) => (
                  <Chip key={s} label={s} size="small" sx={{ background: '#1e2438', color: '#8b93a8', fontFamily: theme.typography.mono.fontFamily }} />
                ))}
              </Box>

              <Typography variant="h4" sx={{ mb: 1 }}>Summary</Typography>
              <Typography variant="body2" sx={{ color: theme.palette.text.secondary, mb: 2, lineHeight: 1.6 }}>
                Service became unavailable. Investigating root cause. Team working on mitigation.
              </Typography>

              <Typography variant="h4" sx={{ mb: 1 }}>Assignee</Typography>
              <Typography variant="body2" sx={{ mb: 2 }}>{incident.assignee || 'Unassigned'}</Typography>

              <Typography variant="h4" sx={{ mb: 1 }}>Responders</Typography>
              <Button variant="outlined" size="small" sx={{ fontSize: 12 }}>+ Add Responder</Button>
            </Box>
          )}

          {tab === 1 && (
            <Box sx={{ p: 2 }}>
              <IncidentTimeline />
            </Box>
          )}

          {tab === 2 && (
            <Box sx={{ p: 2 }}>
              <Typography variant="body2" sx={{ color: theme.palette.text.secondary, textAlign: 'center', py: 2 }}>
                Linked alerts — connect in Phase 7
              </Typography>
            </Box>
          )}

          {tab === 3 && (
            <Box sx={{ p: 2 }}>
              <Box component="pre" sx={{
                fontFamily: theme.typography.mono.fontFamily, fontSize: 12, color: theme.palette.text.primary,
                bgcolor: 'background.default', borderRadius: '4px', p: 2, overflow: 'auto', maxHeight: 300, whiteSpace: 'pre-wrap',
              }}>
                {`# ${incident.title} Runbook

## Symptoms
- Service degradation detected
- Error rates elevated

## Investigation
1. Check monitoring dashboards
2. Review recent deployments
3. Check dependency health

## Resolution
- Follow standard incident response procedure
- Escalate if needed`}
              </Box>
            </Box>
          )}
        </Box>
      </Box>
    </Drawer>
  )
}
