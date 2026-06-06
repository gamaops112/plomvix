import {
  Box, Typography, TextField, Select, MenuItem, Button, Checkbox, FormControlLabel, Dialog, DialogTitle, DialogContent, DialogActions,
} from '@mui/material'
import { X } from 'lucide-react'
import { useForm, Controller } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTheme } from '@mui/material/styles'

const schema = z.object({
  name: z.string().min(1, 'Name is required'),
  severity: z.string().min(1),
  metric: z.string().min(1),
  operator: z.string().min(1),
  threshold: z.number({ message: 'Must be a number' }).positive(),
  thresholdUnit: z.string(),
  duration: z.number(),
  durationUnit: z.string(),
  notifySlack: z.boolean(),
  slackChannel: z.string().optional(),
  notifyPagerDuty: z.boolean(),
  notifyEmail: z.boolean(),
})

type FormData = z.infer<typeof schema>

interface CreateAlertModalProps {
  open: boolean
  onClose: () => void
}

export default function CreateAlertModal({ open, onClose }: CreateAlertModalProps) {
  const theme = useTheme()
  const { control, handleSubmit, watch, reset, formState: { errors } } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: '', severity: 'critical', metric: 'error_rate', operator: '>',
      threshold: 5, thresholdUnit: '%', duration: 5, durationUnit: 'minutes',
      notifySlack: true, slackChannel: '#alerts', notifyPagerDuty: true, notifyEmail: false,
    },
  })

  const notifySlack = watch('notifySlack')

  const onSubmit = () => {
    onClose()
    reset()
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth
      slotProps={{ paper: { sx: { background: theme.palette.background.paper, border: `1px solid ${theme.palette.divider}`, borderRadius: '6px' } } }}>
      <DialogTitle sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', px: 2.5, py: 2 }}>
        <Typography variant="h3">Create Alert Rule</Typography>
        <Button onClick={onClose} sx={{ color: theme.palette.text.secondary, minWidth: 'auto', p: 0.5 }}>
          <X size={18} />
        </Button>
      </DialogTitle>

      <DialogContent dividers sx={{ borderColor: theme.palette.divider }}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <Box>
            <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, mb: 0.5, display: 'block' }}>Name</Typography>
            <Controller name="name" control={control} render={({ field }) => (
              <TextField {...field} size="small" fullWidth error={!!errors.name} helperText={errors.name?.message} />
            )} />
          </Box>

          <Box sx={{ display: 'flex', gap: 1.5 }}>
            <Box sx={{ flex: 1 }}>
              <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, mb: 0.5, display: 'block' }}>Severity</Typography>
              <Controller name="severity" control={control} render={({ field }) => (
                <Select {...field} size="small" fullWidth sx={{ fontSize: 13 }}>
                  <MenuItem value="critical">Critical</MenuItem>
                  <MenuItem value="high">High</MenuItem>
                  <MenuItem value="warning">Warning</MenuItem>
                  <MenuItem value="info">Info</MenuItem>
                </Select>
              )} />
            </Box>
            <Box sx={{ flex: 1 }}>
              <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, mb: 0.5, display: 'block' }}>Service</Typography>
              <Select defaultValue="any" size="small" fullWidth sx={{ fontSize: 13 }}>
                <MenuItem value="any">Any</MenuItem>
              </Select>
            </Box>
          </Box>

          <Box>
            <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, mb: 0.5, display: 'block' }}>Condition</Typography>
            <Box sx={{ display: 'flex', gap: 1 }}>
              <Controller name="metric" control={control} render={({ field }) => (
                <Select {...field} size="small" sx={{ fontSize: 13, flex: 1 }}>
                  <MenuItem value="error_rate">error_rate</MenuItem>
                  <MenuItem value="latency_p99">latency_p99</MenuItem>
                  <MenuItem value="cpu_usage">cpu_usage</MenuItem>
                  <MenuItem value="memory_usage">memory_usage</MenuItem>
                </Select>
              )} />
              <Controller name="operator" control={control} render={({ field }) => (
                <Select {...field} size="small" sx={{ fontSize: 13, width: 70 }}>
                  <MenuItem value=">">&gt;</MenuItem>
                  <MenuItem value="<">&lt;</MenuItem>
                  <MenuItem value=">=">&gt;=</MenuItem>
                </Select>
              )} />
              <Controller name="threshold" control={control} render={({ field }) => (
                <TextField
                  {...field}
                  size="small"
                  type="number"
                  error={!!errors.threshold}
                  helperText={errors.threshold?.message}
                  sx={{ width: 90 }}
                  onChange={(e) => field.onChange(parseFloat(e.target.value) || 0)}
                />
              )} />
              <Controller name="thresholdUnit" control={control} render={({ field }) => (
                <Select {...field} size="small" sx={{ fontSize: 13, width: 80 }}>
                  <MenuItem value="%">%</MenuItem>
                  <MenuItem value="ms">ms</MenuItem>
                  <MenuItem value="s">s</MenuItem>
                </Select>
              )} />
            </Box>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 1 }}>
              <Typography variant="caption2" sx={{ color: theme.palette.text.secondary }}>For</Typography>
              <Controller name="duration" control={control} render={({ field }) => (
                <TextField {...field} size="small" type="number" sx={{ width: 70 }}
                  onChange={(e) => field.onChange(parseInt(e.target.value) || 0)} />
              )} />
              <Controller name="durationUnit" control={control} render={({ field }) => (
                <Select {...field} size="small" sx={{ fontSize: 13 }}>
                  <MenuItem value="minutes">minutes</MenuItem>
                  <MenuItem value="hours">hours</MenuItem>
                </Select>
              )} />
            </Box>
          </Box>

          <Box>
            <Typography variant="caption2" sx={{ color: theme.palette.text.secondary, mb: 0.5, display: 'block' }}>Notifications</Typography>
            <Controller name="notifySlack" control={control} render={({ field }) => (
              <FormControlLabel
                control={<Checkbox {...field} checked={field.value} size="small" sx={{ '&.Mui-checked': { color: '#06b6d4' } }} />}
                label="Slack"
                sx={{ fontSize: 13 }}
              />
            )} />
            {notifySlack && (
              <Controller name="slackChannel" control={control} render={({ field }) => (
                <TextField {...field} size="small" placeholder="#alerts" sx={{ ml: 3, width: 200 }} />
              )} />
            )}
            <Controller name="notifyPagerDuty" control={control} render={({ field }) => (
              <FormControlLabel
                control={<Checkbox {...field} checked={field.value} size="small" sx={{ '&.Mui-checked': { color: '#06b6d4' } }} />}
                label="PagerDuty"
                sx={{ fontSize: 13, display: 'block' }}
              />
            )} />
            <Controller name="notifyEmail" control={control} render={({ field }) => (
              <FormControlLabel
                control={<Checkbox {...field} checked={field.value} size="small" sx={{ '&.Mui-checked': { color: '#06b6d4' } }} />}
                label="Email"
                sx={{ fontSize: 13, display: 'block' }}
              />
            )} />
          </Box>
        </Box>
      </DialogContent>

      <DialogActions sx={{ px: 2.5, py: 2 }}>
        <Button onClick={onClose} sx={{ color: theme.palette.text.secondary, fontSize: 13 }}>Cancel</Button>
        <Button variant="contained" onClick={handleSubmit(onSubmit)} sx={{ fontSize: 13 }}>Create Rule</Button>
      </DialogActions>
    </Dialog>
  )
}
