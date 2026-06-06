export const COLORS = {
  bg: {
    page:     '#0f1117',
    surface:  '#161b27',
    elevated: '#1c2333',
    hover:    '#1e2438',
    selected: '#1a2540',
    overlay:  'rgba(0,0,0,0.6)',
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
    inverse:   '#0f1117',
  },
  accent: {
    default: '#06b6d4',
    hover:   '#0891b2',
    subtle:  '#06b6d415',
    border:  '#06b6d440',
  },
  status: {
    success:    '#10b981',
    successBg:  '#10b98115',
    warning:    '#f59e0b',
    warningBg:  '#f59e0b15',
    error:      '#ef4444',
    errorBg:    '#ef444415',
    info:       '#06b6d4',
    infoBg:     '#06b6d415',
    muted:      '#4d566b',
  },
  chart: {
    colors: ['#06b6d4', '#8b5cf6', '#f59e0b', '#10b981', '#f97316', '#ec4899'],
    grid:   '#1f2535',
    axis:   '#4d566b',
  },
} as const

export const SPACING = {
  1: 4,
  2: 8,
  3: 12,
  4: 16,
  5: 20,
  6: 24,
  8: 32,
  10: 40,
} as const

export const RADIUS = {
  sm: 3,
  md: 4,
  lg: 6,
} as const
