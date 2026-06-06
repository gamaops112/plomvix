import { useState } from 'react'
import { Box, Typography, TextField, Button, Alert, IconButton, InputAdornment, CircularProgress } from '@mui/material'
import { useForm, Controller } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { Eye, EyeOff } from 'lucide-react'
import { useNavigate, useLocation, Link, type Location } from 'react-router-dom'
import { useAuthStore } from '../../../store/authStore'
import { notify } from '../../../lib/toast'

const schema = z.object({
  username: z.string().min(3, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
})
type FormData = z.infer<typeof schema>

export default function LoginForm() {
  const [showPassword, setShowPassword] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const login = useAuthStore((s) => s.login)
  const navigate = useNavigate()
  const location = useLocation()

  const { control, handleSubmit, formState: { errors } } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { username: 'admin', password: 'changeme' },
  })

  const onSubmit = async (data: FormData) => {
    setLoading(true)
    setError('')
    const result = await login(data.username, data.password)
    setLoading(false)
    if (result.success) {
      notify.success(`Welcome back, ${data.username}!`)
      const from = (location.state as { from?: Location } | null)?.from?.pathname ?? '/app'
      navigate(from, { replace: true })
    } else {
      setError(result.error ?? 'Invalid username or password')
    }
  }

  return (
    <Box component="form" onSubmit={handleSubmit(onSubmit)} sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      {error && <Alert severity="error" sx={{ fontSize: 13 }}>{error}</Alert>}

      <Box>
        <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 0.5, display: 'block' }}>Username</Typography>
        <Controller name="username" control={control} render={({ field }) => (
          <TextField {...field} size="small" fullWidth disabled={loading}
            error={!!errors.username} helperText={errors.username?.message}
            type="text" slotProps={{ input: { sx: { fontSize: 13 } } }} />
        )} />
      </Box>

      <Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
          <Typography variant="caption2" sx={{ color: 'text.secondary' }}>Password</Typography>
          <Typography variant="caption2" component={Link} to="/forgot-password"
            sx={{ color: '#06b6d4', textDecoration: 'none', '&:hover': { textDecoration: 'underline' } }}>
            Forgot password?
          </Typography>
        </Box>
        <Controller name="password" control={control} render={({ field }) => (
          <TextField {...field} size="small" fullWidth disabled={loading}
            error={!!errors.password} helperText={errors.password?.message}
            type={showPassword ? 'text' : 'password'}
            slotProps={{ input: { sx: { fontSize: 13 },
              endAdornment: (
                <InputAdornment position="end">
                  <IconButton size="small" onClick={() => setShowPassword(!showPassword)} edge="end" tabIndex={-1}>
                    {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                  </IconButton>
                </InputAdornment>
              ),
            } }} />
        )} />
      </Box>

      <Button type="submit" variant="contained" fullWidth disabled={loading}
        sx={{ height: 40, fontSize: 14, mt: 1 }}>
        {loading ? <CircularProgress size={16} sx={{ mr: 1 }} /> : null}
        {loading ? 'Signing in...' : 'Sign In'}
      </Button>
    </Box>
  )
}
