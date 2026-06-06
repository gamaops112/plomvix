import { useState, useMemo } from 'react'
import {
  Box, Typography, Drawer, Tabs, Tab, IconButton, Button, Chip,
} from '@mui/material'
import { X, ExternalLink } from 'lucide-react'
import { useTheme } from '@mui/material/styles'
import { useNavigate } from 'react-router-dom'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { FiringAlert } from '../mockData'
import { severityColors, generateTimeSeries } from '../mockData'

echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])

interface AlertDetailDrawerProps {
  open: boolean
  alert: FiringAlert | null
  onClose: () => void
}

export default function AlertDetailDrawer({ open, alert, onClose }: AlertDetailDrawerProps) {
  const theme = useTheme()
  const navigate = useNavigate()
  const [tab, setTab] = useState(0)

  const thresholdValue = parseFloat(alert?.condition?.match(/([\d.]+)%?/)?.at(1) || '5')
  const currentValue = parseFloat(alert?.value || '0')

  const chartOpt = useMemo(() => ({
    tooltip: { trigger: 'axis' as const, backgroundColor: theme.palette.background.elevated, borderColor: theme.palette.divider, textStyle: { color: theme.palette.text.primary, fontSize: 12 } },
    grid: { top: 16, right: 16, bottom: 24, left: 48 },
    xAxis: { type: 'category' as const, data: Array.from({ length: 60 }, (_, i) => `${i}m ago`), axisLabel: { color: theme.palette.text.disabled, fontSize: 10 } },
    yAxis: { type: 'value' as const, splitLine: { lineStyle: { color: theme.palette.divider } }, axisLabel: { color: theme.palette.text.disabled, fontSize: 10 } },
    series: [
      { name: 'Value', type: 'line', data: generateTimeSeries(currentValue, currentValue * 0.3), smooth: true, symbol: 'none', lineStyle: { color: '#ef4444', width: 2 } },
      { name: 'Threshold', type: 'line', data: Array(60).fill(thresholdValue), symbol: 'none', lineStyle: { color: '#ef4444', width: 1, type: 'dashed' as const } },
    ],
  }), [currentValue, thresholdValue, theme])

  if (!alert) return null

  return (
    <Drawer anchor="right" open={open} onClose={onClose}
      slotProps={{ paper: { sx: { width: 480, background: theme.palette.background.paper, borderLeft: `1px solid ${theme.palette.divider}` } } }}>
      <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        <Box sx={{ px: 2, py: 1.5, borderBottom: `1px solid ${theme.palette.divider}` }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
            <Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                <Chip label={alert.severity.toUpperCase()} size="small"
                  sx={{ background: `${severityColors[alert.severity]}20`, color: severityColors[alert.severity], fontWeight: 600, fontSize: 10, height: 20 }} />
              </Box>
              <Typography variant="h3">{alert.name}</Typography>
              <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>
                {alert.service} &bull; firing for {alert.duration} &bull; started {alert.started}
              </Typography>
            </Box>
            <Box sx={{ display: 'flex', gap: 1 }}>
              <Button size="small" startIcon={<ExternalLink size={14} />} sx={{ fontSize: 12 }}
                onClick={() => { onClose(); navigate(`/alerts/${alert.id}`) }}>Open full page →</Button>
              <IconButton size="small" onClick={onClose} sx={{ color: theme.palette.text.secondary }}><X size={18} /></IconButton>
            </Box>
          </Box>
        </Box>

        <Tabs value={tab} onChange={(_, v) => setTab(v)}>
          <Tab label="Overview" />
          <Tab label="History" />
          <Tab label="Related Traces" />
          <Tab label="Runbook" />
        </Tabs>

        <Box sx={{ flex: 1, overflow: 'auto' }}>
          {tab === 0 && (
            <Box sx={{ p: 2 }}>
              <Typography variant="caption" sx={{ color: theme.palette.text.secondary, textTransform: 'uppercase', letterSpacing: '0.04em', mb: 0.5, display: 'block' }}>Condition</Typography>
              <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, mb: 2 }}>{alert.condition} for 5 minutes</Typography>

              <Typography variant="caption" sx={{ color: theme.palette.text.secondary, textTransform: 'uppercase', letterSpacing: '0.04em', mb: 0.5, display: 'block' }}>Current Value</Typography>
              <Typography variant="metricSm" sx={{ color: severityColors[alert.severity], mb: 2 }}>
                {alert.value} <Typography component="span" variant="caption2" sx={{ color: theme.palette.text.secondary }}>(threshold: {thresholdValue}%)</Typography>
              </Typography>

              <Box sx={{ height: 180, mb: 2 }}>
                <ReactEChartsCore echarts={echarts} option={chartOpt} style={{ height: 180 }} notMerge />
              </Box>

              <Typography variant="h4" sx={{ mb: 1 }}>Details</Typography>
              {[
                ['Service', alert.service],
                ['Triggered by', alert.condition.split(' >')[0]],
                ['Threshold', `> ${thresholdValue}%`],
                ['Duration', alert.duration],
                ['Notification', 'Slack #alerts, PagerDuty'],
                ['Assignee', alert.assignee || 'Unassigned'],
              ].map(([k, v]) => (
                <Box key={k} sx={{ display: 'flex', py: 0.5, borderBottom: `1px solid ${theme.palette.divider}` }}>
                  <Typography variant="caption" sx={{ minWidth: 130, color: theme.palette.text.secondary, textTransform: 'none', fontWeight: 500 }}>{k}</Typography>
                  <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.primary }}>{v}</Typography>
                </Box>
              ))}

              <Box sx={{ display: 'flex', gap: 1, mt: 2 }}>
                <Button variant="outlined" size="small" sx={{ fontSize: 12 }}>Silence 1h</Button>
                <Button variant="outlined" size="small" sx={{ fontSize: 12 }}>Silence 4h</Button>
                <Button variant="outlined" size="small" sx={{ fontSize: 12 }}>Acknowledge</Button>
                <Button variant="outlined" size="small" sx={{ fontSize: 12, color: '#10b981', borderColor: '#10b981' }}>Resolve</Button>
              </Box>
            </Box>
          )}

          {tab === 1 && (
            <Box sx={{ p: 2 }}>
              {[
                { time: '14:11:00', text: 'Alert fired — error rate reached 8.4%', color: '#ef4444' },
                { time: '14:06:00', text: 'Warning — error rate exceeded 3%', color: '#f59e0b' },
                { time: '13:58:00', text: 'Resolved — previous firing resolved', color: '#10b981' },
                { time: '13:45:00', text: 'Alert fired — error rate reached 6.1%', color: '#ef4444' },
              ].map((e, i) => (
                <Box key={i} sx={{ display: 'flex', gap: 1.5, py: 0.75, borderBottom: `1px solid ${theme.palette.divider}` }}>
                  <Box sx={{ width: 6, height: 6, borderRadius: '50%', background: e.color, mt: 0.5, flexShrink: 0 }} />
                  <Typography variant="caption2" sx={{ fontFamily: theme.typography.mono.fontFamily, color: theme.palette.text.secondary, minWidth: 64 }}>
                    {e.time}
                  </Typography>
                  <Typography variant="body2" sx={{ color: theme.palette.text.primary, fontSize: 13 }}>{e.text}</Typography>
                </Box>
              ))}
            </Box>
          )}

          {tab === 2 && (
            <Box sx={{ p: 2 }}>
              <Typography variant="body2" sx={{ color: theme.palette.text.secondary, textAlign: 'center', py: 2 }}>
                Related traces — connect in Phase 7
              </Typography>
            </Box>
          )}

          {tab === 3 && (
            <Box sx={{ p: 2 }}>
              <Box component="pre" sx={{
                fontFamily: theme.typography.mono.fontFamily, fontSize: 12, color: theme.palette.text.primary,
                bgcolor: 'background.default', borderRadius: '4px', p: 2, overflow: 'auto', maxHeight: 300, whiteSpace: 'pre-wrap',
              }}>
                {`# ${alert.name} Runbook

## Symptoms
- Error rate above threshold
- Possible service degradation
- Increased response times

## Investigation Steps
1. Check service logs for errors
2. Verify dependency health
3. Check recent deployments

## Resolution
- Restart affected pods if needed
- Scale resources if under pressure
- Contact on-call engineer if unresolved`}
              </Box>
            </Box>
          )}
        </Box>
      </Box>
    </Drawer>
  )
}
