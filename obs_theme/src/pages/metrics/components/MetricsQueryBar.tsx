import { Box, Typography, Select, MenuItem } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { metricOptions, aggOptions, groupByOptions } from '../mockData'

interface MetricsQueryBarProps {
  metric: string
  onMetricChange: (v: string) => void
  aggregation: string
  onAggChange: (v: string) => void
  groupBy: string
  onGroupByChange: (v: string) => void
}

const selectSx = { fontSize: 13, height: 30, minWidth: 160 }

export default function MetricsQueryBar({
  metric, onMetricChange, aggregation, onAggChange, groupBy, onGroupByChange,
}: MetricsQueryBarProps) {
  const theme = useTheme()

  return (
    <Box sx={{ display: 'flex', gap: 1.5, mb: 2, flexWrap: 'wrap', alignItems: 'center' }}>
      <Box>
        <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, mb: 0.25, display: 'block' }}>
          Metric
        </Typography>
        <Select value={metric} onChange={(e) => onMetricChange(e.target.value)} size="small" sx={selectSx}>
          {metricOptions.map((m) => (
            <MenuItem key={m} value={m} dense>{m}</MenuItem>
          ))}
        </Select>
      </Box>

      <Box>
        <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, mb: 0.25, display: 'block' }}>
          Aggregation
        </Typography>
        <Select value={aggregation} onChange={(e) => onAggChange(e.target.value)} size="small" sx={{ ...selectSx, minWidth: 100 }}>
          {aggOptions.map((a) => (
            <MenuItem key={a} value={a} dense>{a}</MenuItem>
          ))}
        </Select>
      </Box>

      <Box>
        <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, mb: 0.25, display: 'block' }}>
          Group by
        </Typography>
        <Select value={groupBy} onChange={(e) => onGroupByChange(e.target.value)} size="small" sx={{ ...selectSx, minWidth: 140 }}>
          {groupByOptions.map((g) => (
            <MenuItem key={g} value={g} dense>{g}</MenuItem>
          ))}
        </Select>
      </Box>
    </Box>
  )
}
