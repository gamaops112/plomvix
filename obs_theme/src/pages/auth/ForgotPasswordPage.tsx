import { useState } from 'react'
import { Box, Typography, TextField, Button } from '@mui/material'
import { useForm, Controller } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { ArrowLeft, Check } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useTheme } from '@mui/material/styles'
import { notify } from '../../lib/toast'
import LoginLeftPanel from './components/LoginLeftPanel'

const schema = z.object({ email: z.string().email('Please enter a valid email') })
type FormData = z.infer<typeof schema>

export default function ForgotPasswordPage() {
  const theme = useTheme()
  const [sent, setSent] = useState(false)
  const { control, handleSubmit, getValues, formState: { errors } } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { email: 'demo@obsadmin.io' },
  })

  const onSubmit = () => {
    setSent(true)
    notify.info('Password reset not available in demo mode')
  }

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      <LoginLeftPanel />
      <Box sx={{ flex: '0 0 55%', bgcolor: 'background.paper', display: 'flex', justifyContent: 'center', px: 4, overflow: 'auto' }}>
        <Box sx={{ maxWidth: 400, width: '100%', py: 8 }}>
          {!sent ? (
            <Box>
              <Typography variant="h2" sx={{ mb: 1 }}>Forgot your password?</Typography>
              <Typography variant="body2" sx={{ color: 'text.secondary', mb: 3, lineHeight: 1.6 }}>
                Enter your email address and we'll send you a reset link.
              </Typography>
              <Box component="form" onSubmit={handleSubmit(onSubmit)} sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <Box>
                  <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>Email</Typography>
                  <Controller name="email" control={control} render={({ field }) => (
                    <TextField {...field} size="small" fullWidth type="email" error={!!errors.email} helperText={errors.email?.message} />
                  )} />
                </Box>
                <Button type="submit" variant="contained" fullWidth sx={{ height: 40, fontSize: 14 }}>
                  Send Reset Link
                </Button>
              </Box>
              <Link to="/login" style={{ textDecoration: 'none', display: 'flex', alignItems: 'center', gap: 0.5, marginTop: 24 }}>
                <ArrowLeft size={14} color="#8b93a8" />
                <Typography variant="caption2" sx={{ color: 'text.secondary' }}>Back to sign in</Typography>
              </Link>
            </Box>
          ) : (
            <Box sx={{ textAlign: 'center' }}>
              <Box sx={{ width: 48, height: 48, borderRadius: '50%', bgcolor: '#10b98120', display: 'flex', alignItems: 'center', justifyContent: 'center', mx: 'auto', mb: 2 }}>
                <Check size={24} color="#10b981" />
              </Box>
              <Typography variant="h2" sx={{ mb: 1 }}>Check your email</Typography>
              <Typography variant="body2" sx={{ color: 'text.secondary', mb: 0.5 }}>
                We sent a password reset link to
              </Typography>
              <Typography variant="body2" sx={{ fontFamily: theme.typography.mono.fontFamily, fontWeight: 500, mb: 3 }}>
                {getValues('email')}
              </Typography>
              <Typography variant="caption2" sx={{ color: 'text.disabled', display: 'block', mb: 3 }}>
                The link expires in 30 minutes.
              </Typography>
              <Link to="/login" style={{ textDecoration: 'none', display: 'flex', alignItems: 'center', gap: 0.5, justifyContent: 'center' }}>
                <ArrowLeft size={14} color="#8b93a8" />
                <Typography variant="caption2" sx={{ color: 'text.secondary' }}>Back to sign in</Typography>
              </Link>
            </Box>
          )}
        </Box>
      </Box>
    </Box>
  )
}
