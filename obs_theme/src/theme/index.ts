import { createTheme, alpha } from '@mui/material/styles'

declare module '@mui/material/styles' {
  interface TypeBackground {
    surface: string
    elevated: string
    hover: string
    selected: string
  }
  interface Palette {
    accent: {
      default: string
      hover: string
      subtle: string
      border: string
    }
    chart: {
      colors: string[]
      grid: string
      axis: string
    }
    status: {
      success: string
      warning: string
      error: string
      info: string
      muted: string
    }
  }
  interface PaletteOptions {
    accent?: {
      default?: string
      hover?: string
      subtle?: string
      border?: string
    }
    chart?: {
      colors?: string[]
      grid?: string
      axis?: string
    }
    status?: {
      success?: string
      warning?: string
      error?: string
      info?: string
      muted?: string
    }
  }
  interface TypographyVariants {
    mono: React.CSSProperties
    metric: React.CSSProperties
    metricSm: React.CSSProperties
    caption2: React.CSSProperties
  }
  interface TypographyVariantsOptions {
    mono?: React.CSSProperties
    metric?: React.CSSProperties
    metricSm?: React.CSSProperties
    caption2?: React.CSSProperties
  }
}

declare module '@mui/material/Typography' {
  interface TypographyPropsVariantOverrides {
    mono: true
    metric: true
    metricSm: true
    caption2: true
  }
}

const FONTS = {
  ui: '"Inter", system-ui, -apple-system, sans-serif',
  mono: '"JetBrains Mono", "Fira Code", "Cascadia Code", monospace',
}

const COLORS = {
  bg: {
    page:     '#0f1117',
    surface:  '#161b27',
    elevated: '#1c2333',
    hover:    '#1e2438',
    selected: '#1a2540',
  },
  border: {
    subtle:  '#1f2535',
    default: '#2a3147',
    strong:  '#3d4663',
  },
  text: {
    primary:   '#e8eaf0',
    secondary: '#8b93a8',
    tertiary:  '#4d566b',
  },
  accent: {
    default: '#06b6d4',
    hover:   '#0891b2',
    subtle:  alpha('#06b6d4', 0.08),
    border:  alpha('#06b6d4', 0.25),
  },
  status: {
    success: '#10b981',
    warning: '#f59e0b',
    error:   '#ef4444',
    info:    '#06b6d4',
    muted:   '#4d566b',
  },
  chart: {
    colors: ['#06b6d4', '#8b5cf6', '#f59e0b', '#10b981', '#f97316', '#ec4899'],
    grid:   '#1f2535',
    axis:   '#4d566b',
  },
}

export const darkTheme = createTheme({
  palette: {
    mode: 'dark',
    background: {
      default: COLORS.bg.page,
      paper:   COLORS.bg.surface,
      surface:  COLORS.bg.surface,
      elevated: COLORS.bg.elevated,
      hover:    COLORS.bg.hover,
      selected: COLORS.bg.selected,
    },
    primary: {
      main:  COLORS.accent.default,
      dark:  COLORS.accent.hover,
      light: '#22d3ee',
      contrastText: '#0f1117',
    },
    secondary: {
      main: '#8b5cf6',
      contrastText: '#ffffff',
    },
    success: {
      main: COLORS.status.success,
      contrastText: '#0f1117',
    },
    warning: {
      main: COLORS.status.warning,
      contrastText: '#0f1117',
    },
    error: {
      main: COLORS.status.error,
      contrastText: '#ffffff',
    },
    info: {
      main: COLORS.status.info,
      contrastText: '#0f1117',
    },
    text: {
      primary:   COLORS.text.primary,
      secondary: COLORS.text.secondary,
      disabled:  COLORS.text.tertiary,
    },
    divider: COLORS.border.subtle,
    accent: COLORS.accent,
    chart:  COLORS.chart,
    status: COLORS.status,
  },

  typography: {
    fontFamily: FONTS.ui,
    fontSize: 13,
    h1: { fontSize: '24px', fontWeight: 600, lineHeight: 1.3, letterSpacing: '-0.02em' },
    h2: { fontSize: '20px', fontWeight: 600, lineHeight: 1.3, letterSpacing: '-0.01em' },
    h3: { fontSize: '16px', fontWeight: 600, lineHeight: 1.4 },
    h4: { fontSize: '14px', fontWeight: 600, lineHeight: 1.4 },
    h5: { fontSize: '13px', fontWeight: 600, lineHeight: 1.4 },
    h6: { fontSize: '12px', fontWeight: 600, lineHeight: 1.4 },
    body1: { fontSize: '14px', fontWeight: 400, lineHeight: 1.6 },
    body2: { fontSize: '13px', fontWeight: 400, lineHeight: 1.5 },
    caption: {
      fontSize: '11px',
      fontWeight: 500,
      lineHeight: 1.4,
      letterSpacing: '0.04em',
      textTransform: 'uppercase' as const,
      color: COLORS.text.secondary,
    },
    overline: {
      fontSize: '10px',
      fontWeight: 600,
      letterSpacing: '0.08em',
      textTransform: 'uppercase' as const,
    },
    mono: {
      fontFamily: FONTS.mono,
      fontSize: '13px',
      fontWeight: 400,
      lineHeight: 1.6,
    },
    metric: {
      fontFamily: FONTS.ui,
      fontSize: '28px',
      fontWeight: 600,
      lineHeight: 1.2,
      letterSpacing: '-0.02em',
    },
    metricSm: {
      fontFamily: FONTS.ui,
      fontSize: '20px',
      fontWeight: 600,
      lineHeight: 1.2,
      letterSpacing: '-0.01em',
    },
    caption2: {
      fontFamily: FONTS.ui,
      fontSize: '11px',
      fontWeight: 400,
      lineHeight: 1.4,
      color: COLORS.text.secondary,
    },
  },

  shape: {
    borderRadius: 4,
  },

  spacing: 4,

  shadows: [
    'none',
    'none', 'none', 'none', 'none',
    'none', 'none', 'none', 'none',
    'none', 'none', 'none', 'none',
    'none', 'none', 'none', 'none',
    'none', 'none', 'none', 'none',
    'none', 'none', 'none', 'none',
  ],

  components: {
    MuiCssBaseline: {
      styleOverrides: {
        '*': { boxSizing: 'border-box' },
        body: {
          background: COLORS.bg.page,
          scrollbarWidth: 'thin',
          scrollbarColor: `${COLORS.border.default} transparent`,
        },
        '::-webkit-scrollbar': { width: '6px', height: '6px' },
        '::-webkit-scrollbar-track': { background: 'transparent' },
        '::-webkit-scrollbar-thumb': {
          background: COLORS.border.default,
          borderRadius: '3px',
          '&:hover': { background: COLORS.border.strong },
        },
      },
    },

    MuiButton: {
      defaultProps: {
        disableElevation: true,
        size: 'small',
      },
      styleOverrides: {
        root: {
          textTransform: 'none',
          fontWeight: 500,
          fontSize: '13px',
          borderRadius: '4px',
          padding: '5px 12px',
          minHeight: '30px',
        },
        contained: {
          background: '#06b6d4',
          color: '#0f1117',
          '&:hover': { background: '#0891b2' },
        },
        outlined: {
          borderColor: '#2a3147',
          color: '#e8eaf0',
          '&:hover': {
            background: '#1e2438',
            borderColor: '#3d4663',
          },
        },
        text: {
          color: '#8b93a8',
          '&:hover': { background: '#1e2438', color: '#e8eaf0' },
        },
      },
    },

    MuiIconButton: {
      defaultProps: { size: 'small' },
      styleOverrides: {
        root: {
          borderRadius: '4px',
          color: '#8b93a8',
          '&:hover': { background: '#1e2438', color: '#e8eaf0' },
        },
      },
    },

    MuiOutlinedInput: {
      styleOverrides: {
        root: {
          fontSize: '13px',
          borderRadius: '4px',
          background: '#161b27',
          '& .MuiOutlinedInput-notchedOutline': {
            borderColor: '#2a3147',
          },
          '&:hover .MuiOutlinedInput-notchedOutline': {
            borderColor: '#3d4663',
          },
          '&.Mui-focused .MuiOutlinedInput-notchedOutline': {
            borderColor: '#06b6d4',
            borderWidth: '1px',
            boxShadow: '0 0 0 2px rgba(6,182,212,0.15)',
          },
        },
        input: {
          padding: '6px 10px',
          height: '20px',
          color: '#e8eaf0',
          '&::placeholder': { color: '#4d566b', opacity: 1 },
        },
      },
    },

    MuiInputLabel: {
      styleOverrides: {
        root: {
          fontSize: '13px',
          color: '#8b93a8',
          '&.Mui-focused': { color: '#06b6d4' },
        },
      },
    },

    MuiSelect: {
      defaultProps: { size: 'small' },
      styleOverrides: {
        icon: { color: '#8b93a8' },
      },
    },

    MuiMenu: {
      styleOverrides: {
        paper: {
          background: '#1c2333',
          border: '1px solid #2a3147',
          borderRadius: '4px',
          boxShadow: 'none',
        },
      },
    },

    MuiMenuItem: {
      styleOverrides: {
        root: {
          fontSize: '13px',
          minHeight: '32px',
          padding: '6px 12px',
          color: '#e8eaf0',
          '&:hover': { background: '#1e2438' },
          '&.Mui-selected': {
            background: '#1a2540',
            '&:hover': { background: '#1e2438' },
          },
        },
      },
    },

    MuiTable: {
      styleOverrides: {
        root: { borderCollapse: 'collapse' },
      },
    },

    MuiTableHead: {
      styleOverrides: {
        root: {
          '& .MuiTableCell-root': {
            background: '#161b27',
            borderBottom: '1px solid #2a3147',
            color: '#8b93a8',
            fontSize: '11px',
            fontWeight: 500,
            letterSpacing: '0.04em',
            textTransform: 'uppercase',
            padding: '8px 12px',
            whiteSpace: 'nowrap',
          },
        },
      },
    },

    MuiTableBody: {
      styleOverrides: {
        root: {
          '& .MuiTableRow-root': {
            '&:hover': { background: '#1e2438' },
            '&.Mui-selected': { background: '#1a2540' },
          },
        },
      },
    },

    MuiTableCell: {
      styleOverrides: {
        root: {
          fontSize: '13px',
          padding: '7px 12px',
          borderBottom: '1px solid #1f2535',
          color: '#e8eaf0',
          height: '36px',
        },
        body: {
          color: '#e8eaf0',
        },
      },
    },

    MuiCard: {
      defaultProps: { elevation: 0 },
      styleOverrides: {
        root: {
          background: '#161b27',
          border: '1px solid #1f2535',
          borderRadius: '4px',
        },
      },
    },

    MuiCardContent: {
      styleOverrides: {
        root: {
          padding: '16px',
          '&:last-child': { paddingBottom: '16px' },
        },
      },
    },

    MuiCardHeader: {
      styleOverrides: {
        root: {
          padding: '12px 16px',
          borderBottom: '1px solid #1f2535',
        },
        title: {
          fontSize: '13px',
          fontWeight: 600,
          color: '#e8eaf0',
        },
        subheader: {
          fontSize: '12px',
          color: '#8b93a8',
        },
      },
    },

    MuiChip: {
      styleOverrides: {
        root: {
          borderRadius: '3px',
          fontSize: '11px',
          fontWeight: 500,
          height: '20px',
        },
        filled: {
          '&.MuiChip-colorSuccess': {
            background: 'rgba(16,185,129,0.12)',
            color: '#10b981',
          },
          '&.MuiChip-colorWarning': {
            background: 'rgba(245,158,11,0.12)',
            color: '#f59e0b',
          },
          '&.MuiChip-colorError': {
            background: 'rgba(239,68,68,0.12)',
            color: '#ef4444',
          },
          '&.MuiChip-colorInfo': {
            background: 'rgba(6,182,212,0.12)',
            color: '#06b6d4',
          },
          '&.MuiChip-colorDefault': {
            background: '#1e2438',
            color: '#8b93a8',
          },
        },
      },
    },

    MuiTabs: {
      styleOverrides: {
        root: {
          minHeight: '36px',
          borderBottom: '1px solid #1f2535',
        },
        indicator: {
          background: '#06b6d4',
          height: '2px',
        },
      },
    },

    MuiTab: {
      styleOverrides: {
        root: {
          minHeight: '36px',
          fontSize: '13px',
          fontWeight: 400,
          textTransform: 'none',
          color: '#8b93a8',
          padding: '0 16px',
          '&.Mui-selected': {
            color: '#e8eaf0',
            fontWeight: 500,
          },
          '&:hover': { color: '#e8eaf0', background: '#1e2438' },
        },
      },
    },

    MuiTooltip: {
      styleOverrides: {
        tooltip: {
          background: '#1c2333',
          border: '1px solid #2a3147',
          color: '#e8eaf0',
          fontSize: '12px',
          borderRadius: '4px',
          padding: '6px 10px',
          boxShadow: 'none',
        },
        arrow: {
          color: '#1c2333',
        },
      },
    },

    MuiDivider: {
      styleOverrides: {
        root: {
          borderColor: '#1f2535',
        },
      },
    },

    MuiDrawer: {
      styleOverrides: {
        paper: {
          background: '#161b27',
          border: 'none',
          borderRight: '1px solid #1f2535',
        },
      },
    },

    MuiAppBar: {
      defaultProps: { elevation: 0 },
      styleOverrides: {
        root: {
          background: '#161b27',
          borderBottom: '1px solid #1f2535',
          color: '#e8eaf0',
        },
      },
    },

    MuiBreadcrumbs: {
      styleOverrides: {
        root: { fontSize: '13px' },
        separator: { color: '#4d566b' },
        ol: { flexWrap: 'nowrap' },
      },
    },

    MuiLinearProgress: {
      styleOverrides: {
        root: {
          height: '3px',
          borderRadius: '2px',
          background: '#2a3147',
        },
        bar: {
          borderRadius: '2px',
        },
      },
    },

    MuiSkeleton: {
      styleOverrides: {
        root: {
          background: '#1c2333',
          '&::after': {
            background: 'linear-gradient(90deg, transparent, rgba(255,255,255,0.03), transparent)',
          },
        },
      },
    },
  },
})

export const getDensityTokens = (density: 'compact' | 'default' | 'comfortable') => ({
  tableRowHeight: density === 'compact' ? 28 : density === 'comfortable' ? 44 : 36,
  cardPadding: density === 'compact' ? 12 : density === 'comfortable' ? 24 : 16,
  inputHeight: density === 'compact' ? 28 : density === 'comfortable' ? 40 : 32,
})

export const lightTheme = createTheme({
  palette: {
    mode: 'light',
    background: {
      default: '#f4f5f7',
      paper: '#ffffff',
      surface: '#ffffff',
      elevated: '#f9fafb',
      hover: '#f3f4f6',
      selected: '#eff6ff',
    },
    primary: {
      main: '#0891b2',
      dark: '#0e7490',
      light: '#06b6d4',
      contrastText: '#ffffff',
    },
    secondary: {
      main: '#8b5cf6',
      contrastText: '#ffffff',
    },
    success: { main: '#10b981', contrastText: '#0f1117' },
    warning: { main: '#f59e0b', contrastText: '#0f1117' },
    error: { main: '#ef4444', contrastText: '#ffffff' },
    info: { main: '#0891b2', contrastText: '#ffffff' },
    text: { primary: '#111827', secondary: '#6b7280', disabled: '#9ca3af' },
    divider: '#e5e7eb',
    accent: {
      default: '#0891b2',
      hover: '#0e7490',
      subtle: alpha('#0891b2', 0.08),
      border: alpha('#0891b2', 0.25),
    },
    chart: {
      colors: ['#0891b2','#8b5cf6','#f59e0b','#10b981','#f97316','#ec4899'],
      grid: '#e5e7eb',
      axis: '#9ca3af',
    },
    status: {
      success: '#10b981', warning: '#f59e0b', error: '#ef4444', info: '#0891b2', muted: '#9ca3af',
    },
  },
  typography: darkTheme.typography,
  shape: darkTheme.shape,
  spacing: darkTheme.spacing,
  shadows: darkTheme.shadows,
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        '*': { boxSizing: 'border-box' },
        body: { background: '#f4f5f7' },
        '::-webkit-scrollbar': { width: '6px', height: '6px' },
        '::-webkit-scrollbar-track': { background: 'transparent' },
        '::-webkit-scrollbar-thumb': { background: '#d1d5db', borderRadius: '3px' },
      },
    },
    MuiButton: {
      defaultProps: { disableElevation: true, size: 'small' as const },
      styleOverrides: {
        root: { textTransform: 'none' as const, fontWeight: 500, fontSize: '13px', borderRadius: '4px', padding: '5px 12px', minHeight: '30px' },
        contained: { background: '#0891b2', color: '#ffffff', '&:hover': { background: '#0e7490' } },
        outlined: { borderColor: '#d1d5db', color: '#111827', '&:hover': { background: '#f3f4f6', borderColor: '#9ca3af' } },
        text: { color: '#6b7280', '&:hover': { background: '#f3f4f6', color: '#111827' } },
      },
    },
    MuiIconButton: { defaultProps: { size: 'small' }, styleOverrides: { root: { borderRadius: '4px', color: '#6b7280', '&:hover': { background: '#f3f4f6', color: '#111827' } } } },
    MuiOutlinedInput: {
      styleOverrides: {
        root: {
          background: '#ffffff',
          fontSize: '13px', borderRadius: '4px',
          '& .MuiOutlinedInput-notchedOutline': { borderColor: '#d1d5db' },
          '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: '#9ca3af' },
          '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: '#0891b2', boxShadow: '0 0 0 2px rgba(8,145,178,0.12)' },
        },
        input: { padding: '6px 10px', height: '20px', color: '#111827' },
      },
    },
    MuiInputLabel: { styleOverrides: { root: { fontSize: '13px', color: '#6b7280', '&.Mui-focused': { color: '#0891b2' } } } },
    MuiCard: { defaultProps: { elevation: 0 }, styleOverrides: { root: { background: '#ffffff', border: '1px solid #e5e7eb', borderRadius: '4px' } } },
    MuiCardContent: { styleOverrides: { root: { padding: '16px', '&:last-child': { paddingBottom: '16px' } } } },
    MuiCardHeader: { styleOverrides: { root: { padding: '12px 16px', borderBottom: '1px solid #e5e7eb' }, title: { fontSize: '13px', fontWeight: 600, color: '#111827' }, subheader: { fontSize: '12px', color: '#6b7280' } } },
    MuiDrawer: { styleOverrides: { paper: { background: '#ffffff', borderRight: '1px solid #e5e7eb' } } },
    MuiAppBar: { defaultProps: { elevation: 0 }, styleOverrides: { root: { background: '#ffffff', borderBottom: '1px solid #e5e7eb', color: '#111827' } } },
    MuiTableHead: { styleOverrides: { root: { '& .MuiTableCell-root': { background: '#f9fafb', borderBottom: '1px solid #e5e7eb', color: '#6b7280' } } } },
    MuiTableBody: { styleOverrides: { root: { '& .MuiTableRow-root': { '&:hover': { background: '#f3f4f6' }, '&.Mui-selected': { background: '#eff6ff' } } } } },
    MuiTableCell: { styleOverrides: { root: { fontSize: '13px', padding: '7px 12px', borderBottom: '1px solid #e5e7eb', color: '#111827' }, body: { color: '#111827' } } },
    MuiChip: darkTheme.components?.MuiChip,
    MuiTable: darkTheme.components?.MuiTable,
    MuiSelect: darkTheme.components?.MuiSelect,
    MuiMenu: { styleOverrides: { paper: { background: '#ffffff', border: '1px solid #e5e7eb', borderRadius: '4px', boxShadow: 'none' } } },
    MuiMenuItem: { styleOverrides: { root: { fontSize: '13px', minHeight: '32px', padding: '6px 12px', color: '#111827', '&:hover': { background: '#f3f4f6' }, '&.Mui-selected': { background: '#eff6ff' } } } },
    MuiTabs: { styleOverrides: { root: { minHeight: '36px', borderBottom: '1px solid #e5e7eb' }, indicator: { background: '#0891b2', height: '2px' } } },
    MuiTab: { styleOverrides: { root: { minHeight: '36px', fontSize: '13px', fontWeight: 400, textTransform: 'none', color: '#6b7280', '&.Mui-selected': { color: '#111827', fontWeight: 500 } } } },
    MuiTooltip: { styleOverrides: { tooltip: { background: '#1f2937', border: '1px solid #374151', color: '#f9fafb', fontSize: '12px', borderRadius: '4px' } } },
    MuiDivider: { styleOverrides: { root: { borderColor: '#e5e7eb' } } },
    MuiLinearProgress: darkTheme.components?.MuiLinearProgress,
    MuiSkeleton: darkTheme.components?.MuiSkeleton,
  },
})
