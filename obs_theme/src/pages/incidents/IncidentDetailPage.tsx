import { Box, Typography, Button, Grid, Chip } from '@mui/material'
import { useParams, useNavigate } from 'react-router-dom'
import { useTheme } from '@mui/material/styles'
import { ArrowLeft } from 'lucide-react'
import { mockIncidents } from './mockData'
import type { Incident } from './mockData'
import { severityColors } from '../alerts/mockData'
import IncidentTimeline from './components/IncidentTimeline'

export default function IncidentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const theme = useTheme()

  const incident: Incident | undefined = mockIncidents.find((i: Incident) => i.id === id)

  if (!incident) {
    return <Box sx={{ p: 3 }}><Typography>Incident not found</Typography></Box>
  }

  return (
    <Box sx={{ p: 3 }}>
      <Button startIcon={<ArrowLeft size={14} />} sx={{ color: theme.palette.text.secondary, fontSize: 13, mb: 2 }}
        onClick={() => navigate('/incidents')}>Back to Incidents</Button>

      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
            <Chip label={incident.severity.toUpperCase()} size="small"
              sx={{ background: `${severityColors[incident.severity]}20`, color: severityColors[incident.severity], fontWeight: 600, fontSize: 10, height: 20 }} />
            <Typography variant="caption2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary }}>
              INC-{incident.id}
            </Typography>
          </Box>
          <Typography variant="h2">{incident.title}</Typography>
          <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, textTransform: 'capitalize' }}>
            {incident.status} &bull; {incident.duration}
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button variant="outlined" size="small" sx={{ fontSize: 13 }}>Acknowledge</Button>
          <Button variant="outlined" size="small" sx={{ fontSize: 13, color: '#10b981', borderColor: '#10b981' }}>Resolve</Button>
          <Button variant="outlined" size="small" sx={{ fontSize: 13 }}>Create Postmortem</Button>
        </Box>
      </Box>

      <Grid container spacing={2} sx={{ mb: 3 }}>
        {[
          { label: 'Status', value: incident.status.toUpperCase() },
          { label: 'Duration', value: incident.duration },
          { label: 'Alerts', value: String(incident.alerts) },
          { label: 'Responders', value: '1' },
        ].map((stat) => (
          <Grid key={stat.label} size={{ xs: 6, md: 3 }}>
            <Box sx={{ p: 1.5, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
              <Typography variant="caption" sx={{ color: theme.palette.text.secondary }}>{stat.label}</Typography>
              <Typography variant="metricSm">{stat.value}</Typography>
            </Box>
          </Grid>
        ))}
      </Grid>

      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 5 }}>
          <Box sx={{ p: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
            <Typography variant="h4" sx={{ mb: 2 }}>Timeline</Typography>
            <IncidentTimeline />
          </Box>
        </Grid>
        <Grid size={{ xs: 12, md: 7 }}>
          <Box sx={{ p: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
            <Typography variant="h4" sx={{ mb: 2 }}>Affected Services</Typography>
            <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', mb: 2 }}>
              {incident.affectedServices.map((s: string) => (
                <Chip key={s} label={s} size="small" sx={{ background: '#1e2438', color: '#8b93a8', fontFamily: theme.typography.mono.fontFamily }} />
              ))}
            </Box>
            <Typography variant="h4" sx={{ mb: 1 }}>Summary</Typography>
            <Typography variant="body2" sx={{ color: theme.palette.text.secondary, lineHeight: 1.6 }}>
              Service became unavailable. Investigating root cause. Team working on mitigation.
            </Typography>
          </Box>
        </Grid>
      </Grid>
    </Box>
  )
}
