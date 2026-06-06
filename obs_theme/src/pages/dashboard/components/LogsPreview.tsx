import { Box, Typography, Card, CardContent, CardHeader } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { Link } from 'react-router-dom'
import { logsPreviewData, logLevelColors } from '../mockData'

export default function LogsPreview() {
  const theme = useTheme()

  return (
    <Card sx={{ height: '100%' }}>
      <CardHeader
        title="Logs Preview"
        sx={{ borderBottom: `1px solid ${theme.palette.divider}` }}
      />
      <CardContent sx={{ p: 0, '&:last-child': { pb: 0 } }}>
        {logsPreviewData.map((log, i) => (
          <Box
            key={i}
            sx={{
              display: 'flex',
              alignItems: 'center',
              px: 2,
              height: 28,
              borderBottom: `1px solid ${theme.palette.divider}`,
              '&:hover': { background: theme.palette.background.hover },
              '&:last-child': { borderBottom: 'none' },
            }}
          >
            <Box
              sx={{
                fontSize: 10,
                fontWeight: 600,
                color: logLevelColors[log.level],
                background: `${logLevelColors[log.level]}20`,
                borderRadius: '3px',
                px: 0.75,
                py: 0.25,
                mr: 1,
                lineHeight: 1.2,
                minWidth: 40,
                textAlign: 'center',
              }}
            >
              {log.level}
            </Box>
            <Typography
              variant="caption2"
              sx={{
                fontFamily: theme.typography.mono.fontFamily,
                color: theme.palette.text.disabled,
                mr: 1,
                minWidth: 52,
              }}
            >
              {log.time}
            </Typography>
            <Typography
              variant="caption2"
              sx={{
                fontFamily: theme.typography.mono.fontFamily,
                color: theme.palette.text.secondary,
                mr: 1.5,
                minWidth: 100,
              }}
            >
              {log.service}
            </Typography>
            <Typography
              variant="body2"
              sx={{
                fontFamily: theme.typography.mono.fontFamily,
                color: theme.palette.text.primary,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                flex: 1,
              }}
            >
              {log.message}
            </Typography>
          </Box>
        ))}
        <Box sx={{ display: 'flex', justifyContent: 'flex-end', p: 1 }}>
          <Typography
            component={Link}
            to="/logs"
            variant="caption2"
            sx={{ color: theme.palette.primary.main, textDecoration: 'none', '&:hover': { textDecoration: 'underline' } }}
          >
            View all logs →
          </Typography>
        </Box>
      </CardContent>
    </Card>
  )
}
