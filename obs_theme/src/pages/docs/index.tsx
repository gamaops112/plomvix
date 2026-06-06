import { Box, Typography, Grid } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { BookOpen, Rocket, Wrench, Code, MessageCircle, ExternalLink } from 'lucide-react'

interface DocItem { label: string; desc: string; external?: boolean }

const sections: { title: string; icon: typeof Rocket; items: DocItem[] }[] = [
  {
    title: 'Getting Started',
    icon: Rocket,
    items: [{ label: 'Quick Start Guide', desc: 'Get up and running in 5 minutes' }, { label: 'Installation', desc: 'Deploy obsAdmin in your environment' }, { label: 'Configuration', desc: 'Configure data sources and settings' }],
  },
  {
    title: 'Features',
    icon: Wrench,
    items: [{ label: 'Logs Explorer', desc: 'Search and analyze log data' }, { label: 'Metrics & Infrastructure', desc: 'Monitor hosts and services' }, { label: 'Traces & APM', desc: 'Distributed tracing and performance' }, { label: 'Alerts & Incidents', desc: 'Alerting and incident management' }, { label: 'Synthetics', desc: 'Synthetic monitoring and status pages' }],
  },
  {
    title: 'API Reference',
    icon: Code,
    items: [{ label: 'REST API', desc: 'Full REST API documentation' }, { label: 'Query Language', desc: 'Log and metric query syntax' }],
  },
  {
    title: 'Community',
    icon: MessageCircle,
    items: [{ label: 'GitHub', desc: 'github.com/obsadmin', external: true }, { label: 'Discord', desc: 'Join our community server', external: true }, { label: 'Contributing Guide', desc: 'How to contribute to obsAdmin', external: true }],
  },
]

export default function DocsPage() {
  const theme = useTheme()

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 3 }}>
        <BookOpen size={24} color="#06b6d4" />
        <Typography variant="h2">Documentation</Typography>
      </Box>

      <Grid container spacing={3}>
        {sections.map((section) => {
          const Icon = section.icon
          return (
            <Grid key={section.title} size={{ xs: 12, md: 6 }}>
              <Box sx={{ mb: 2 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1.5 }}>
                  <Icon size={18} color="#06b6d4" />
                  <Typography variant="h4">{section.title}</Typography>
                </Box>
                {section.items.map((item) => (
                  <Box
                    key={item.label}
                    sx={{
                      p: 1.5, mb: 1,
                      border: `1px solid ${theme.palette.divider}`,
                      borderRadius: '4px',
                      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                      cursor: 'pointer',
                      '&:hover': { background: theme.palette.background.hover },
                    }}
                  >
                    <Box>
                      <Typography variant="body2" sx={{ fontWeight: 500 }}>{item.label}</Typography>
                      <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>{item.desc}</Typography>
                    </Box>
                    {item.external && <ExternalLink size={14} color="#8b93a8" />}
                  </Box>
                ))}
              </Box>
            </Grid>
          )
        })}
      </Grid>
    </Box>
  )
}
