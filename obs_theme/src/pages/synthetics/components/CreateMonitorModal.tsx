import { Box, Typography, TextField, Select, MenuItem, Button, Checkbox, FormControlLabel, Dialog, DialogTitle, DialogContent, DialogActions } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { X } from 'lucide-react'
import { useForm, Controller } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'

const schema = z.object({
  name: z.string().min(2, 'Name is required'),
  type: z.enum(['HTTP', 'TCP', 'SSL', 'DNS', 'Journey']),
  url: z.string().min(1, 'URL is required'),
  frequency: z.string(),
  locations: z.array(z.string()).min(1, 'Select at least one location'),
  notifyOnFailure: z.boolean(),
})

type FormData = z.infer<typeof schema>

interface CreateMonitorModalProps {
  open: boolean
  onClose: () => void
  editData?: FormData | null
}

const locationOptions = ['US East (N. Virginia)', 'EU West (Ireland)', 'Asia Pacific (Singapore)', 'US West (Oregon)', 'South America (São Paulo)']

const urlHelpers: Record<string, string> = {
  HTTP: 'https://example.com/health',
  TCP: 'hostname:port',
  SSL: 'https://example.com',
  DNS: 'example.com',
  Journey: 'https://example.com/login',
}

export default function CreateMonitorModal({ open, onClose, editData }: CreateMonitorModalProps) {
  const theme = useTheme()
  const isEdit = !!editData

  const { control, handleSubmit, watch, reset } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: editData || {
      name: '', type: 'HTTP', url: '', frequency: '1m',
      locations: ['US East (N. Virginia)', 'EU West (Ireland)'],
      notifyOnFailure: true,
    },
  })

  const watchType = watch('type')

  const onSubmit = () => {
    onClose()
    reset()
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth
      slotProps={{ paper: { sx: { background: theme.palette.background.paper, border: `1px solid ${theme.palette.divider}` } } }}>
      <DialogTitle sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', px: 2.5, py: 2 }}>
        <Typography variant="h3">{isEdit ? 'Edit Monitor' : 'Create Monitor'}</Typography>
        <Button onClick={onClose} sx={{ color: theme.palette.text.secondary, minWidth: 'auto', p: 0.5 }}><X size={18} /></Button>
      </DialogTitle>
      <DialogContent dividers sx={{ borderColor: theme.palette.divider }}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <Box>
            <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>Name</Typography>
            <Controller name="name" control={control} render={({ field }) => <TextField {...field} size="small" fullWidth />} />
          </Box>
          <Box>
            <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>Type</Typography>
            <Controller name="type" control={control} render={({ field }) => (
              <Select {...field} size="small" fullWidth>
                <MenuItem value="HTTP">HTTP</MenuItem>
                <MenuItem value="TCP">TCP</MenuItem>
                <MenuItem value="SSL">SSL</MenuItem>
                <MenuItem value="DNS">DNS</MenuItem>
                <MenuItem value="Journey">Journey</MenuItem>
              </Select>
            )} />
          </Box>
          <Box>
            <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>URL / Host</Typography>
            <Controller name="url" control={control} render={({ field }) =>
              <TextField {...field} size="small" fullWidth helperText={urlHelpers[watchType]} />
            } />
          </Box>
          <Typography variant="caption" sx={{ color: 'text.disabled', borderTop: 1, borderColor: 'divider', pt: 2 }}>SCHEDULE</Typography>
          <Box>
            <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>Check Frequency</Typography>
            <Controller name="frequency" control={control} render={({ field }) => (
              <Select {...field} size="small" fullWidth>
                <MenuItem value="30s">30 seconds</MenuItem>
                <MenuItem value="1m">1 minute</MenuItem>
                <MenuItem value="2m">2 minutes</MenuItem>
                <MenuItem value="5m">5 minutes</MenuItem>
                <MenuItem value="10m">10 minutes</MenuItem>
                <MenuItem value="15m">15 minutes</MenuItem>
                <MenuItem value="30m">30 minutes</MenuItem>
                <MenuItem value="1h">1 hour</MenuItem>
              </Select>
            )} />
          </Box>
          <Typography variant="caption" sx={{ color: 'text.disabled', borderTop: 1, borderColor: 'divider', pt: 2 }}>LOCATIONS</Typography>
          <Controller name="locations" control={control} render={({ field }) => (
            <Box>
              {locationOptions.map((loc) => (
                <FormControlLabel
                  key={loc}
                  control={<Checkbox size="small" checked={field.value.includes(loc)} onChange={(e) => {
                    field.onChange(e.target.checked ? [...field.value, loc] : field.value.filter((l: string) => l !== loc))
                  }} />}
                  label={loc}
                  sx={{ display: 'block' }}
                />
              ))}
            </Box>
          )} />
          <Typography variant="caption" sx={{ color: 'text.disabled', borderTop: 1, borderColor: 'divider', pt: 2 }}>NOTIFICATIONS</Typography>
          <Controller name="notifyOnFailure" control={control} render={({ field }) => (
            <FormControlLabel control={<Checkbox size="small" checked={field.value} onChange={field.onChange} />} label="Notify on failure" />
          )} />
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 2.5, py: 2 }}>
        <Button onClick={onClose} sx={{ fontSize: 13 }}>Cancel</Button>
        <Button variant="contained" onClick={handleSubmit(onSubmit)} sx={{ fontSize: 13 }}>
          {isEdit ? 'Update Monitor' : 'Create Monitor'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
