import { Box, List, Typography, IconButton, Tooltip, useMediaQuery, useTheme } from '@mui/material'
import { NavLink, useLocation } from 'react-router-dom'
import { PanelLeftClose, PanelLeftOpen } from 'lucide-react'
import { useUIStore } from '../store/uiStore'
import { useSettingsStore } from '../store/settingsStore'
import { navSections } from './navConfig'

export default function Sidebar() {
  const sidebarCollapsed = useUIStore((s) => s.sidebarCollapsed)
  const toggleSidebar = useUIStore((s) => s.toggleSidebar)
  const location = useLocation()
  const theme = useTheme()
  const isMobile = useMediaQuery(theme.breakpoints.down('md'))
  const { showSectionLabels, showIcons } = useSettingsStore()

  const effectiveCollapsed = isMobile ? true : sidebarCollapsed
  const width = effectiveCollapsed ? 56 : 220

  return (
    <Box
      sx={{
        position: 'fixed',
        left: 0,
        top: 48,
        width,
        height: 'calc(100vh - 48px)',
        bgcolor: 'background.paper',
        borderRight: 1,
        borderColor: 'divider',
        transition: 'width 200ms ease',
        zIndex: 1000,
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}
    >
      <List sx={{ flex: 1, overflow: 'auto', pt: 1, pb: 1 }}>
        {navSections.map((section) => (
          <Box key={section.label}>
            {!effectiveCollapsed && (
              <Typography sx={{ fontSize: 10, fontWeight: 500, letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled', px: 2.5, pt: 2, pb: 0.5 }}>
                {section.label}
              </Typography>
            )}
            {section.items.map((item) => {
              const Icon = item.icon
              const isActive = location.pathname === item.path
              return (
                <Tooltip key={item.path} title={effectiveCollapsed ? item.label : ''} placement="right" arrow>
                  <Box
                    component={NavLink}
                    to={item.path}
                    sx={{
                      display: 'flex', alignItems: 'center', height: 34, px: 1.5, mx: 1, my: '1px', borderRadius: '4px',
                      color: isActive ? 'text.primary' : 'text.secondary',
                      bgcolor: isActive ? 'background.selected' : 'transparent',
                      borderLeft: '2px solid',
                      borderColor: isActive ? 'primary.main' : 'transparent',
                      textDecoration: 'none', gap: 1.5,
                      '&:hover': { bgcolor: 'background.hover', color: 'text.primary' },
                    }}
                  >
                    {showIcons && <Icon size={16} />}
            {showSectionLabels && !effectiveCollapsed && (
                      <Typography sx={{ fontSize: 13, fontWeight: isActive ? 500 : 400 }}>
                        {item.label}
                      </Typography>
                    )}
                  </Box>
                </Tooltip>
              )
            })}
          </Box>
        ))}
      </List>

      {!isMobile && (
        <Box sx={{ borderTop: 1, borderColor: 'divider', display: 'flex', justifyContent: sidebarCollapsed ? 'center' : 'flex-end', p: 1 }}>
          <IconButton onClick={toggleSidebar} size="small" sx={{ color: 'text.secondary' }}>
            {sidebarCollapsed ? <PanelLeftOpen size={18} /> : <PanelLeftClose size={18} />}
          </IconButton>
        </Box>
      )}
    </Box>
  )
}
