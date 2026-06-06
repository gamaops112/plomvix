import { Box, Typography, Button, Chip } from '@mui/material'
import { Activity, FileText, GitBranch, Bell, Radio } from 'lucide-react'
import { useTheme } from '@mui/material/styles'
import { notify } from '../../lib/toast'

const datasets = [
  { icon: Activity, name: 'Infrastructure Metrics', desc: '11 hosts, 847 containers, 60min of metrics data' },
  { icon: FileText, name: 'Log Stream', desc: '200 log entries across 10 services' },
  { icon: GitBranch, name: 'Traces & Spans', desc: '12 traces, 9 spans per trace, service map' },
  { icon: Bell, name: 'Alerts & Incidents', desc: '8 firing alerts, 4 incidents, 10 alert rules' },
  { icon: Radio, name: 'Synthetics Monitors', desc: '10 monitors across HTTP/TCP/SSL/DNS types' },
]

export default function Demo() {
  const theme = useTheme()

  return (
    <Box sx={{ p: 3, maxWidth: 800 }}>
      <Typography variant="h2" sx={{ mb: 1 }}>Demo Data & Sandbox</Typography>
      <Typography variant="body2" sx={{ color: theme.palette.text.secondary, mb: 3, lineHeight: 1.6 }}>
        Load mock data to explore obsAdmin features. Data is generated locally and does not affect any real systems.
      </Typography>

      {datasets.map((ds) => {
        const Icon = ds.icon
        return (
          <Box key={ds.name} sx={{ p: 2, mb: 2, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px', display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
            <Icon size={22} color="#06b6d4" />
            <Box sx={{ flex: 1 }}>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>{ds.name}</Typography>
              <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>{ds.desc}</Typography>
            </Box>
            <Chip label="Loaded" size="small" sx={{ background: '#10b98120', color: '#10b981', fontWeight: 500 }} />
            <Button size="small" variant="outlined" sx={{ fontSize: 12 }} onClick={() => notify.success('Data reloaded')}>Reload</Button>
            <Button size="small" variant="outlined" sx={{ fontSize: 12, color: '#ef4444', borderColor: '#ef4444' }}>Clear</Button>
          </Box>
        )
      })}

      <Box sx={{ display: 'flex', gap: 1, mt: 3 }}>
        <Button variant="contained" onClick={() => notify.success('Demo data loaded successfully')}>
          Load All Demo Data
        </Button>
        <Button variant="outlined">Clear All Data</Button>
      </Box>
    </Box>
  )
}
