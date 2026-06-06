import { Box, Skeleton, Grid } from '@mui/material'

interface PageSkeletonProps {
  variant?: 'table' | 'cards' | 'chart' | 'dashboard'
}

export default function PageSkeleton({ variant = 'dashboard' }: PageSkeletonProps) {
  if (variant === 'table') {
    return (
      <Box sx={{ p: 3 }}>
        <Skeleton variant="text" width={200} height={32} sx={{ mb: 2 }} />
        <Skeleton variant="rounded" height={40} sx={{ mb: 2 }} />
        {Array.from({ length: 8 }).map((_, i) => (
          <Skeleton key={i} variant="rounded" height={36} sx={{ mb: 0.5 }} />
        ))}
      </Box>
    )
  }

  if (variant === 'cards') {
    return (
      <Box sx={{ p: 3 }}>
        <Skeleton variant="text" width={200} height={32} sx={{ mb: 2 }} />
        <Grid container spacing={2}>
          {Array.from({ length: 6 }).map((_, i) => (
            <Grid key={i} size={{ xs: 12, sm: 6, md: 4 }}>
              <Skeleton variant="rounded" height={140} />
            </Grid>
          ))}
        </Grid>
      </Box>
    )
  }

  if (variant === 'chart') {
    return (
      <Box sx={{ p: 3 }}>
        <Skeleton variant="text" width={200} height={32} sx={{ mb: 2 }} />
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 8 }}>
            <Skeleton variant="rounded" height={320} />
          </Grid>
          <Grid size={{ xs: 12, md: 4 }}>
            <Skeleton variant="rounded" height={320} />
          </Grid>
        </Grid>
      </Box>
    )
  }

  return (
    <Box sx={{ p: 3 }}>
      <Skeleton variant="text" width={200} height={32} sx={{ mb: 2 }} />
      <Grid container spacing={2} sx={{ mb: 2 }}>
        {Array.from({ length: 4 }).map((_, i) => (
          <Grid key={i} size={{ xs: 12, sm: 6, md: 3 }}>
            <Skeleton variant="rounded" height={100} />
          </Grid>
        ))}
      </Grid>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 8 }}>
          <Skeleton variant="rounded" height={300} />
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <Skeleton variant="rounded" height={300} />
        </Grid>
      </Grid>
    </Box>
  )
}
