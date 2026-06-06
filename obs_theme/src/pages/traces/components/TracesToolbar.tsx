import { Box, TextField, Select, MenuItem } from '@mui/material'
import { Search } from 'lucide-react'

interface TracesToolbarProps {
  search: string
  onSearchChange: (v: string) => void
  serviceFilter: string
  onServiceChange: (v: string) => void
  statusFilter: string
  onStatusChange: (v: string) => void
}

const services = ['All', 'api-gateway', 'auth-service', 'user-service', 'payment-service', 'search-service', 'notification-svc']

export default function TracesToolbar({
  search, onSearchChange, serviceFilter, onServiceChange, statusFilter, onStatusChange,
}: TracesToolbarProps) {

  return (
    <Box sx={{ display: 'flex', gap: 1.5, mb: 2, flexWrap: 'wrap', alignItems: 'center' }}>
      <TextField
        size="small"
        placeholder="Search traces..."
        value={search}
        onChange={(e) => onSearchChange(e.target.value)}
        sx={{ width: 240 }}
        slotProps={{
          input: {
            startAdornment: <Search size={14} color="#4d566b" style={{ marginRight: 6 }} />,
          },
        }}
      />
      <Select value={serviceFilter} onChange={(e) => onServiceChange(e.target.value)} size="small" sx={{ fontSize: 13, minWidth: 150 }}>
        <MenuItem value="All">Service: All</MenuItem>
        {services.filter((s) => s !== 'All').map((s) => (
          <MenuItem key={s} value={s}>{s}</MenuItem>
        ))}
      </Select>
      <Select value={statusFilter} onChange={(e) => onStatusChange(e.target.value)} size="small" sx={{ fontSize: 13, minWidth: 130 }}>
        <MenuItem value="All">Status: All</MenuItem>
        <MenuItem value="error">Error</MenuItem>
        <MenuItem value="slow">Slow (&gt;1s)</MenuItem>
        <MenuItem value="ok">OK</MenuItem>
      </Select>
    </Box>
  )
}
