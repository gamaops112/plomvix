import { Box, Typography, Button, Grid } from '@mui/material'
import { useParams, useNavigate } from 'react-router-dom'
import { useTheme } from '@mui/material/styles'
import { ArrowLeft } from 'lucide-react'
import { mockTraces, mockSpanTree } from './mockData'
import type { Trace } from './mockData'
import SpanWaterfall from './components/SpanWaterfall'

export default function TraceDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const theme = useTheme()

  const trace: Trace | undefined = mockTraces.find((t: Trace) => t.id === id)

  if (!trace) {
    return (
      <Box sx={{ p: 3 }}>
        <Typography>Trace not found</Typography>
      </Box>
    )
  }

  const spanTree = mockSpanTree

  return (
    <Box sx={{ p: 3 }}>
      <Button
        startIcon={<ArrowLeft size={14} />}
        sx={{ color: theme.palette.text.secondary, fontSize: 13, mb: 2 }}
        onClick={() => navigate('/traces')}
      >
        Back to Traces
      </Button>

      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Box>
          <Typography variant="h2">Trace: {trace.id}</Typography>
          <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
            {trace.rootOp} &bull; {trace.rootService} &bull; {trace.duration > 1000 ? `${(trace.duration / 1000).toFixed(1)}s` : `${trace.duration}ms`} &bull; {trace.time}
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button variant="outlined" size="small" sx={{ fontSize: 13 }}>Copy ID</Button>
          <Button variant="outlined" size="small" sx={{ fontSize: 13 }}>Share</Button>
        </Box>
      </Box>

      <Grid container spacing={2} sx={{ mb: 3 }}>
        {[
          { label: 'Duration', value: trace.duration > 1000 ? `${(trace.duration / 1000).toFixed(1)}s` : `${trace.duration}ms` },
          { label: 'Spans', value: String(trace.spans) },
          { label: 'Errors', value: String(trace.errors) },
          { label: 'Services', value: '7' },
        ].map((stat) => (
          <Grid key={stat.label} size={{ xs: 6, md: 3 }}>
            <Box sx={{ p: 1.5, border: `1px solid ${theme.palette.divider}`, borderRadius: '4px' }}>
              <Typography variant="caption" sx={{ color: theme.palette.text.secondary }}>{stat.label}</Typography>
              <Typography variant="metricSm">{stat.value}</Typography>
            </Box>
          </Grid>
        ))}
      </Grid>

      <SpanWaterfall
        totalDuration={spanTree.totalDuration}
        rootSpan={spanTree.rootSpan}
        traceId={spanTree.traceId}
        rootOperation={spanTree.rootOperation}
      />
    </Box>
  )
}
