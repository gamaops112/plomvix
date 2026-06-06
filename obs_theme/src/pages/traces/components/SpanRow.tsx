import { useState } from 'react'
import { Box, Typography } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { ChevronRight, ChevronDown } from 'lucide-react'
import type { SpanNode } from '../mockData'
import { serviceColors } from '../mockData'

interface SpanRowProps {
  span: SpanNode
  totalDuration: number
  depth: number
}

export default function SpanRow({ span, totalDuration, depth }: SpanRowProps) {
  const theme = useTheme()
  const [expanded, setExpanded] = useState(true)
  const hasChildren = span.children.length > 0

  const barLeft = (span.startOffset / totalDuration) * 100
  const barWidth = (span.duration / totalDuration) * 100
  const barColor = span.status === 'error' ? '#ef4444' : (serviceColors[span.service] || '#06b6d4')

  return (
    <Box>
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          height: 32,
          cursor: hasChildren ? 'pointer' : 'default',
          '&:hover': { background: theme.palette.background.hover },
          position: 'relative',
        }}
        onClick={() => hasChildren && setExpanded(!expanded)}
      >
        <Box sx={{ width: 16 * depth, flexShrink: 0 }} />
        <Box sx={{ width: 20, flexShrink: 0, display: 'flex', justifyContent: 'center' }}>
          {hasChildren ? (
            expanded ? <ChevronDown size={12} color="#8b93a8" /> : <ChevronRight size={12} color="#8b93a8" />
          ) : (
            <Box sx={{ width: 4, height: 4, borderRadius: '50%', background: '#4d566b' }} />
          )}
        </Box>
        <Typography sx={{ width: 140, flexShrink: 0, fontSize: 12, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary }}>
          {span.service}
        </Typography>
        <Typography sx={{ width: 180, flexShrink: 0, fontSize: 12, fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary }}>
          {span.operation}
        </Typography>
        <Box sx={{ flex: 1, position: 'relative', height: 12, mx: 1 }}>
          <Box
            sx={{
              position: 'absolute',
              left: `${barLeft}%`,
              width: `${Math.max(barWidth, 0.5)}%`,
              height: 12,
              borderRadius: '2px',
              background: barColor,
              opacity: 0.8,
              minWidth: 2,
            }}
          />
        </Box>
        <Typography sx={{
          width: 72, flexShrink: 0, textAlign: 'right', fontSize: 12,
          fontFamily: theme.typography.mono.fontFamily,
          color: span.duration > 1000 ? '#ef4444' : theme.palette.text.primary,
        }}>
          {span.duration}ms
        </Typography>
        {span.status === 'error' && (
          <Typography sx={{ width: 24, flexShrink: 0, textAlign: 'center', fontSize: 11, color: '#ef4444' }}>
            ❌
          </Typography>
        )}
      </Box>

      {hasChildren && expanded && span.children.map((child) => (
        <SpanRow
          key={child.id}
          span={child}
          totalDuration={totalDuration}
          depth={depth + 1}
        />
      ))}
    </Box>
  )
}
