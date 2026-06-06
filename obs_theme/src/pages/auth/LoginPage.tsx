import { Box, Typography, Divider } from '@mui/material'
import LoginLeftPanel from './components/LoginLeftPanel'
import SSOButtons from './components/SSOButtons'
import LoginForm from './components/LoginForm'
import DemoCredentials from './components/DemoCredentials'

export default function LoginPage() {
  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      <LoginLeftPanel />

      <Box sx={{ flex: '0 0 55%', bgcolor: 'background.paper', display: 'flex', justifyContent: 'center', px: 4, overflow: 'auto' }}>
        <Box sx={{ maxWidth: 400, width: '100%', py: 8 }}>
          <Typography variant="h2" sx={{ mb: 0.5 }}>Welcome back</Typography>
          <Typography variant="body2" sx={{ color: 'text.secondary', mb: 3 }}>
            Sign in to your obsAdmin instance
          </Typography>
          <SSOButtons />
          <Divider sx={{ my: 3 }}>
            <Typography variant="caption2" sx={{ color: 'text.disabled', px: 1 }}>or continue with</Typography>
          </Divider>
          <LoginForm />
          <DemoCredentials />
        </Box>
      </Box>
    </Box>
  )
}
