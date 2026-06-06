import { Box, Typography, Button } from '@mui/material'
import { useTheme } from '@mui/material/styles'
import type { LucideIcon } from 'lucide-react'

interface EmptyStateProps {
  icon?: LucideIcon
  title: string
  description: string
  action?: { label: string; onClick: () => void }
}

export default function EmptyState({ icon: Icon, title, description, action }: EmptyStateProps) {
  const theme = useTheme()

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        py: 8,
        px: 2,
      }}
    >
      {Icon && <Icon size={48} color="#4d566b" style={{ marginBottom: 16 }} />}
      <Typography variant="h4" sx={{ mb: 1, color: theme.palette.text.secondary }}>
        {title}
      </Typography>
      <Typography variant="body2" sx={{ color: theme.palette.text.disabled, mb: 3, textAlign: 'center', maxWidth: 360 }}>
        {description}
      </Typography>
      {action && (
        <Button variant="outlined" size="small" onClick={action.onClick} sx={{ fontSize: 13 }}>
          {action.label}
        </Button>
      )}
    </Box>
  )
}
