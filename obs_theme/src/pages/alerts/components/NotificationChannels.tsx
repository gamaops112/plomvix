import { Box, Typography, Grid, Button } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { Bell, Mail, Globe, Plus, Hash } from 'lucide-react'
import { mockChannels } from '../mockData'

const typeIcons: Record<string, typeof Bell> = {
  slack: Hash,
  pagerduty: Bell,
  email: Mail,
  webhook: Globe,
}

export default function NotificationChannels() {
  const theme = useTheme()

  return (
    <Box>
      <Box sx={{ display: 'flex', justifyContent: 'flex-end', mb: 2 }}>
        <Button variant="outlined" size="small" startIcon={<Plus size={14} />} sx={{ fontSize: 13 }}>
          Add Channel
        </Button>
      </Box>
      <Grid container spacing={2}>
        {mockChannels.map((ch) => {
          const Icon = typeIcons[ch.type] || Globe
          const isError = ch.status === 'error'
          return (
            <Grid key={ch.id} size={{ xs: 12, sm: 6, md: 4 }}>
              <Box sx={{
                p: 2,
                border: `1px solid ${isError ? '#ef444440' : theme.palette.divider}`,
                borderRadius: '4px',
              }}>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 1 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Icon size={18} color={isError ? '#ef4444' : '#8b93a8'} />
                    <Box>
                      <Typography variant="body2" sx={{ fontWeight: 500, textTransform: 'capitalize' }}>{ch.type}</Typography>
                      <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>{ch.name}</Typography>
                    </Box>
                  </Box>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                    <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: isError ? '#ef4444' : '#10b981' }} />
                    <Typography variant="caption2" sx={{ color: isError ? '#ef4444' : '#10b981', textTransform: 'capitalize' }}>
                      {ch.status}
                    </Typography>
                  </Box>
                </Box>
                <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
                  Last tested: {ch.lastTest}
                </Typography>
                <Box sx={{ display: 'flex', gap: 1, mt: 1.5 }}>
                  <Button size="small" variant="outlined" sx={{ fontSize: 11, py: 0 }}>Test</Button>
                  <Button size="small" variant="outlined" sx={{ fontSize: 11, py: 0 }}>Edit</Button>
                  <Button size="small" variant="outlined" sx={{ fontSize: 11, py: 0, color: '#ef4444', borderColor: '#ef4444' }}>Delete</Button>
                </Box>
              </Box>
            </Grid>
          )
        })}
      </Grid>
    </Box>
  )
}
