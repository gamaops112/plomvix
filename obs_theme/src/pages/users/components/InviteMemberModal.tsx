import { Box, Typography, TextField, Select, MenuItem, Button, Dialog, DialogTitle, DialogContent, DialogActions } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { X } from 'lucide-react'
import { useForm, Controller } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'

const schema = z.object({
  email: z.string().email('Invalid email'),
  role: z.enum(['Admin', 'Editor', 'Viewer']),
  message: z.string().optional(),
})

type FormData = z.infer<typeof schema>

interface InviteMemberModalProps { open: boolean; onClose: () => void }

export default function InviteMemberModal({ open, onClose }: InviteMemberModalProps) {
  const theme = useTheme()

  const { control, handleSubmit, reset } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { email: '', role: 'Editor', message: '' },
  })

  const onSubmit = () => { onClose(); reset() }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth
      slotProps={{ paper: { sx: { bgcolor: 'background.paper', border: `1px solid ${theme.palette.divider}` } } }}>
      <DialogTitle sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', px: 2.5, py: 2 }}>
        <Typography variant="h3">Invite Team Member</Typography>
        <Button onClick={onClose} sx={{ color: 'text.secondary', minWidth: 'auto', p: 0.5 }}><X size={18} /></Button>
      </DialogTitle>
      <DialogContent dividers sx={{ borderColor: theme.palette.divider }}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <Box>
            <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>Email Address</Typography>
            <Controller name="email" control={control} render={({ field }) => <TextField {...field} size="small" fullWidth type="email" />} />
          </Box>
          <Box>
            <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>Role</Typography>
            <Controller name="role" control={control} render={({ field }) => (
              <Select {...field} size="small" fullWidth>
                <MenuItem value="Admin">Admin</MenuItem>
                <MenuItem value="Editor">Editor</MenuItem>
                <MenuItem value="Viewer">Viewer</MenuItem>
              </Select>
            )} />
          </Box>
          <Box>
            <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>Personal message (optional)</Typography>
            <Controller name="message" control={control} render={({ field }) => (
              <TextField {...field} size="small" fullWidth multiline rows={3} placeholder="Hi! I'd like to invite you to..." />
            )} />
          </Box>
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 2.5, py: 2 }}>
        <Button onClick={onClose} sx={{ fontSize: 13 }}>Cancel</Button>
        <Button variant="contained" onClick={handleSubmit(onSubmit)} sx={{ fontSize: 13 }}>Send Invite</Button>
      </DialogActions>
    </Dialog>
  )
}
