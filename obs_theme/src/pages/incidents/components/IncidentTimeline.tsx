import { Box, Typography, TextField, Button } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { timelineEvents } from '../mockData'

export default function IncidentTimeline() {
  const theme = useTheme()

  return (
    <Box>
      {timelineEvents.map((e, i) => (
        <Box key={i} sx={{ display: 'flex', gap: 1.5, py: 1, borderBottom: `1px solid ${theme.palette.divider}` }}>
          <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: e.color, mt: 0.5, flexShrink: 0 }} />
          <Typography variant="caption2" sx={{
            fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary, minWidth: 48, fontSize: 11,
          }}>
            {e.time}
          </Typography>
          <Box sx={{ flex: 1 }}>
            <Typography variant="body2" sx={{ color: theme.palette.text.primary, fontSize: 13 }}>
              <Typography component="span" sx={{ fontWeight: 500, mr: 0.5 }}>{e.actor}</Typography>
              {e.text}
            </Typography>
          </Box>
        </Box>
      ))}

      <Box sx={{ display: 'flex', gap: 1, mt: 2, alignItems: 'center' }}>
        <TextField size="small" placeholder="Add a note..." fullWidth sx={{ fontSize: 13 }} />
        <Button variant="contained" size="small" sx={{ fontSize: 13, flexShrink: 0 }}>Send</Button>
      </Box>
    </Box>
  )
}
