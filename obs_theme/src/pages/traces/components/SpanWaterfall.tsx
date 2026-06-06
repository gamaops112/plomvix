import { Box, Typography } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import SpanRow from './SpanRow'
import type { SpanNode } from '../mockData'

interface SpanWaterfallProps {
  totalDuration: number
  rootSpan: SpanNode
  traceId: string
  rootOperation: string
}

export default function SpanWaterfall({ totalDuration, rootSpan, traceId, rootOperation }: SpanWaterfallProps) {
  const theme = useTheme()

  const markers = [0, 25, 50, 75, 100]

  return (
    <Box>
      <Box sx={{ mb: 1.5, px: 1 }}>
        <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary, mb: 0.5 }}>
          Trace: {traceId.substring(0, 8)} &bull; {rootOperation} &bull; {totalDuration}ms
        </Typography>

        <Box sx={{ display: 'flex', alignItems: 'center', height: 24, position: 'relative' }}>
          <Box sx={{ width: 176, flexShrink: 0 }} />
          <Box sx={{ flex: 1, position: 'relative', height: 24 }}>
            {markers.map((m) => (
              <Box
                key={m}
                sx={{
                  position: 'absolute',
                  left: `${m}%`,
                  transform: 'translateX(-50%)',
                }}
              >
                <Typography variant="caption2" sx={{ color: theme.palette.text.disabled, fontSize: 10 }}>
                  {Math.round((totalDuration * m) / 100)}ms
                </Typography>
                <Box sx={{ width: 1, height: 8, borderLeft: 1, borderColor: 'divider', mx: 'auto', mt: 0.25 }} />
              </Box>
            ))}
          </Box>
          <Box sx={{ width: 96, flexShrink: 0 }} />
        </Box>
      </Box>

      <SpanRow span={rootSpan} totalDuration={totalDuration} depth={0} />
    </Box>
  )
}
