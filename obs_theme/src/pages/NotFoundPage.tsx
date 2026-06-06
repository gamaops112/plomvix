import { Box, Typography, Button } from '@mui/material'
import { ArrowLeft } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

export default function NotFoundPage() {
  const navigate = useNavigate()
  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', bgcolor: 'background.default', gap: 2 }}>
      <Typography sx={{ fontSize: '72px', fontWeight: 700, color: 'text.disabled', lineHeight: 1 }}>404</Typography>
      <Typography variant="h3">Page not found</Typography>
      <Typography variant="body2" sx={{ color: 'text.secondary' }}>The page you're looking for doesn't exist or has been moved.</Typography>
      <Box sx={{ display: 'flex', gap: 1, mt: 1 }}>
        <Button variant="outlined" onClick={() => navigate(-1)} startIcon={<ArrowLeft size={14} />}>Go back</Button>
        <Button variant="contained" onClick={() => navigate('/')}>Dashboard</Button>
      </Box>
    </Box>
  )
}
