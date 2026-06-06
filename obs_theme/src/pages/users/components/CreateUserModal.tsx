import { useState, useMemo } from 'react'
import { Box, Typography, TextField, Button, Checkbox, FormControlLabel, Dialog, DialogTitle, DialogContent, DialogActions, Radio, LinearProgress, IconButton, InputAdornment } from '@mui/material'
import { useTheme, alpha } from '@mui/material/styles'
import { X, Eye, EyeOff } from 'lucide-react'
import { useForm, Controller } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'

const schema = z.object({
  name: z.string().min(2, 'Name must be at least 2 characters'),
  email: z.string().email('Invalid email address'),
  role: z.enum(['Admin', 'Editor', 'Viewer']),
  password: z.string().min(8, 'Min 8 characters').regex(/[A-Z]/, 'Must contain uppercase').regex(/[0-9]/, 'Must contain a number').regex(/[^A-Za-z0-9]/, 'Must contain special character'),
  confirmPassword: z.string(),
  sendWelcomeEmail: z.boolean(),
}).refine((d) => d.password === d.confirmPassword, { message: "Passwords don't match", path: ['confirmPassword'] })

type FormData = z.infer<typeof schema>

interface CreateUserModalProps { open: boolean; onClose: () => void }

export default function CreateUserModal({ open, onClose }: CreateUserModalProps) {
  const theme = useTheme()
  const [showPw, setShowPw] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)

  const { control, handleSubmit, watch, reset, formState: { errors } } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { name: '', email: '', role: 'Editor', password: '', confirmPassword: '', sendWelcomeEmail: true },
  })

  const watchPw = watch('password')
  const watchRole = watch('role')

  const strength = useMemo(() => {
    const pw = watchPw || ''
    let score = 0
    if (pw.length >= 8) score++
    if (pw.length >= 12) score++
    if (/[A-Z]/.test(pw)) score++
    if (/[0-9]/.test(pw)) score++
    if (/[^A-Za-z0-9]/.test(pw)) score++
    if (score <= 1) return { label: 'Weak', color: '#ef4444', value: 20 }
    if (score <= 2) return { label: 'Fair', color: '#f59e0b', value: 40 }
    if (score <= 3) return { label: 'Good', color: '#f59e0b', value: 60 }
    if (score <= 4) return { label: 'Strong', color: '#10b981', value: 80 }
    return { label: 'Very Strong', color: '#10b981', value: 100 }
  }, [watchPw])

  const onSubmit = () => { onClose(); reset() }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth
      slotProps={{ paper: { sx: { bgcolor: 'background.paper', border: `1px solid ${theme.palette.divider}` } } }}>
      <DialogTitle sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', px: 2.5, py: 2 }}>
        <Typography variant="h3">Add User</Typography>
        <Button onClick={onClose} sx={{ color: 'text.secondary', minWidth: 'auto', p: 0.5 }}><X size={18} /></Button>
      </DialogTitle>
      <DialogContent dividers sx={{ borderColor: theme.palette.divider }}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <Box>
            <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>Full Name</Typography>
            <Controller name="name" control={control} render={({ field }) => <TextField {...field} size="small" fullWidth error={!!errors.name} helperText={errors.name?.message} />} />
          </Box>
          <Box>
            <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>Email Address</Typography>
            <Controller name="email" control={control} render={({ field }) => <TextField {...field} size="small" fullWidth type="email" error={!!errors.email} helperText={errors.email?.message} />} />
          </Box>
          <Typography variant="caption" sx={{ color: 'text.disabled', borderTop: 1, borderColor: 'divider', pt: 2, textTransform: 'uppercase', letterSpacing: '0.04em' }}>Role</Typography>
          <Controller name="role" control={control} render={({ field }) => (
            <Box>
              {(['Admin', 'Editor', 'Viewer'] as const).map((role) => (
                <Box key={role} onClick={() => field.onChange(role)} sx={{ border: `1px solid ${watchRole === role ? theme.palette.primary.main : theme.palette.divider}`, bgcolor: watchRole === role ? alpha(theme.palette.primary.main, 0.06) : 'transparent', borderRadius: '4px', p: 1.5, cursor: 'pointer', mb: 1, display: 'flex', alignItems: 'center', gap: 1.5 }}>
                  <Radio checked={watchRole === role} size="small" sx={{ p: 0 }} />
                  <Box>
                    <Typography variant="body2" sx={{ fontWeight: 500 }}>{role}</Typography>
                    <Typography variant="caption" sx={{ color: 'text.secondary' }}>
                      {role === 'Admin' ? 'Full access to all features' : role === 'Editor' ? 'Can edit but not manage users or delete' : 'Read-only access to all data'}
                    </Typography>
                  </Box>
                </Box>
              ))}
            </Box>
          )} />
          <Typography variant="caption" sx={{ color: 'text.disabled', borderTop: 1, borderColor: 'divider', pt: 2, textTransform: 'uppercase', letterSpacing: '0.04em' }}>Set Password</Typography>
          <Controller name="password" control={control} render={({ field }) => (
            <TextField {...field} size="small" fullWidth label="Password" type={showPw ? 'text' : 'password'} error={!!errors.password} helperText={errors.password?.message}
              slotProps={{ input: { endAdornment: <InputAdornment position="end"><IconButton size="small" onClick={() => setShowPw(!showPw)}>{showPw ? <EyeOff size={16} /> : <Eye size={16} />}</IconButton></InputAdornment> } }} />
          )} />
          <Controller name="confirmPassword" control={control} render={({ field }) => (
            <TextField {...field} size="small" fullWidth label="Confirm Password" type={showConfirm ? 'text' : 'password'} error={!!errors.confirmPassword} helperText={errors.confirmPassword?.message}
              slotProps={{ input: { endAdornment: <InputAdornment position="end"><IconButton size="small" onClick={() => setShowConfirm(!showConfirm)}>{showConfirm ? <EyeOff size={16} /> : <Eye size={16} />}</IconButton></InputAdornment> } }} />
          )} />
          <Box>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
              <Typography variant="caption2" sx={{ color: 'text.secondary' }}>Password strength</Typography>
              <Typography variant="caption2" sx={{ color: strength.color, fontWeight: 500 }}>{strength.label}</Typography>
            </Box>
            <LinearProgress variant="determinate" value={strength.value} sx={{ height: 4, borderRadius: 2, '& .MuiLinearProgress-bar': { background: strength.color } }} />
          </Box>
          <Typography variant="caption" sx={{ color: 'text.disabled', borderTop: 1, borderColor: 'divider', pt: 2, textTransform: 'uppercase', letterSpacing: '0.04em' }}>Options</Typography>
          <Controller name="sendWelcomeEmail" control={control} render={({ field }) => (
            <FormControlLabel control={<Checkbox size="small" checked={field.value} onChange={field.onChange} />} label="Send welcome email to user" />
          )} />
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 2.5, py: 2 }}>
        <Button onClick={onClose} sx={{ fontSize: 13 }}>Cancel</Button>
        <Button variant="contained" onClick={handleSubmit(onSubmit)} sx={{ fontSize: 13 }}>Create User</Button>
      </DialogActions>
    </Dialog>
  )
}
