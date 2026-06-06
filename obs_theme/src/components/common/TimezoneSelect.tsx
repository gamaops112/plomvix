import { useMemo } from 'react'
import { Autocomplete, TextField, Box, Typography } from '@mui/material'
import { getTimeZones } from '@vvo/tzdb'

interface TimezoneSelectProps {
  value: string
  onChange: (value: string) => void
}

export default function TimezoneSelect({ value, onChange }: TimezoneSelectProps) {
  const timezones = useMemo(() => {
    return getTimeZones().map((tz) => ({
      label: `${tz.currentTimeFormat} — ${tz.name}`,
      value: tz.name,
      offset: tz.currentTimeOffsetInMinutes,
      region: tz.continentName,
      abbreviation: tz.abbreviation,
    }))
  }, [])

  const selected = timezones.find((tz) => tz.value === value) ?? null

  return (
    <Autocomplete
      options={timezones}
      groupBy={(option) => option.region}
      getOptionLabel={(option) => option.label}
      value={selected}
      onChange={(_, newValue) => onChange(newValue?.value ?? 'UTC')}
      renderInput={(params) => (
        <TextField {...params} size="small" placeholder="Search timezone..." />
      )}
      renderOption={(props, option) => (
        <Box component="li" {...props} sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
          <Typography variant="body2">{option.value}</Typography>
          <Typography variant="caption" sx={{ color: 'text.disabled' }}>
            {option.abbreviation}  {option.offset >= 0 ? '+' : ''}{Math.floor(option.offset / 60)}:{String(Math.abs(option.offset % 60)).padStart(2, '0')}
          </Typography>
        </Box>
      )}
      sx={{ width: '100%', maxWidth: 400 }}
      slotProps={{ listbox: { sx: { maxHeight: 300 } } }}
    />
  )
}
