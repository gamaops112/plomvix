import { useState } from 'react'
import { Box, Typography, Grid, Button, Tabs, Tab } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { Activity, FileText, GitBranch, Search, Database, TrendingUp, Hash, Phone, Bell, Mail, GitMerge, Layers, Cloud } from 'lucide-react'

const allIntegrations = [
  { name: 'Prometheus', category: 'Data Sources', status: 'connected', icon: Activity, desc: 'Metrics collection and alerting' },
  { name: 'Loki', category: 'Data Sources', status: 'connected', icon: FileText, desc: 'Log aggregation system' },
  { name: 'Tempo', category: 'Data Sources', status: 'connected', icon: GitBranch, desc: 'Distributed tracing backend' },
  { name: 'Elasticsearch', category: 'Data Sources', status: 'error', icon: Search, desc: 'Search and analytics engine' },
  { name: 'ClickHouse', category: 'Data Sources', status: 'available', icon: Database, desc: 'Column-oriented database' },
  { name: 'InfluxDB', category: 'Data Sources', status: 'available', icon: TrendingUp, desc: 'Time series database' },
  { name: 'Slack', category: 'Alerting', status: 'connected', icon: Hash, desc: 'Team messaging and notifications' },
  { name: 'PagerDuty', category: 'Alerting', status: 'connected', icon: Phone, desc: 'Incident response platform' },
  { name: 'OpsGenie', category: 'Alerting', status: 'available', icon: Bell, desc: 'Alert management platform' },
  { name: 'Email', category: 'Alerting', status: 'connected', icon: Mail, desc: 'Email notifications' },
  { name: 'GitHub Actions', category: 'CI/CD', status: 'available', icon: GitMerge, desc: 'CI/CD and deployment tracking' },
  { name: 'GitLab CI', category: 'CI/CD', status: 'available', icon: GitMerge, desc: 'GitLab pipeline integration' },
  { name: 'Jenkins', category: 'CI/CD', status: 'available', icon: Layers, desc: 'Open source automation server' },
  { name: 'AWS', category: 'Cloud', status: 'available', icon: Cloud, desc: 'Amazon Web Services metrics' },
  { name: 'GCP', category: 'Cloud', status: 'available', icon: Cloud, desc: 'Google Cloud Platform metrics' },
  { name: 'Azure', category: 'Cloud', status: 'available', icon: Cloud, desc: 'Microsoft Azure metrics' },
]

const categories = ['All', 'Data Sources', 'Alerting', 'CI/CD', 'Cloud']
const statusColors: Record<string, string> = { connected: '#10b981', error: '#ef4444', available: '#8b93a8' }

export default function Integrations() {
  const theme = useTheme()
  const [tab, setTab] = useState(0)

  const filtered = tab === 0 ? allIntegrations : allIntegrations.filter((i) => i.category === categories[tab])

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h2" sx={{ mb: 3 }}>Integrations</Typography>

      <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ mb: 3 }}>
        {categories.map((c) => <Tab key={c} label={c} />)}
      </Tabs>

      <Grid container spacing={2}>
        {filtered.map((item) => {
          const Icon = item.icon
          const statusColor = statusColors[item.status]
          return (
            <Grid key={item.name} size={{ xs: 12, sm: 6, md: 4 }}>
              <Box sx={{
                p: 2,
                border: `1px solid ${theme.palette.divider}`,
                borderRadius: '4px',
                borderLeft: item.status === 'connected' ? `3px solid #10b981` : item.status === 'error' ? `3px solid #ef4444` : `3px solid transparent`,
              }}>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 1 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Icon size={20} color="#8b93a8" />
                    <Typography variant="h4">{item.name}</Typography>
                  </Box>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                    <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: statusColor }} />
                    <Typography variant="caption2" sx={{ color: statusColor, textTransform: 'capitalize' }}>
                      {item.status}
                    </Typography>
                  </Box>
                </Box>
                <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, display: 'block', mb: 1.5 }}>
                  {item.desc}
                </Typography>
                <Box sx={{ display: 'flex', gap: 1 }}>
                  {item.status === 'available' ? (
                    <Button variant="outlined" size="small" sx={{ fontSize: 12 }}>Configure</Button>
                  ) : (
                    <>
                      <Button variant="outlined" size="small" sx={{ fontSize: 12 }}>Configure</Button>
                      <Button variant="outlined" size="small" sx={{ fontSize: 12, color: '#ef4444', borderColor: '#ef4444' }}>Disconnect</Button>
                    </>
                  )}
                </Box>
              </Box>
            </Grid>
          )
        })}
      </Grid>
    </Box>
  )
}
