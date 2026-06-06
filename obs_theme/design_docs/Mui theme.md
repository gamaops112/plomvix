# MUI Theme Configuration

## File location
`src/theme/index.ts`

## Folder structure to create
```
src/
└── theme/
    ├── index.ts          ← paste the theme below
    ├── tokens.ts         ← color/spacing constants (reference theme-tokens.md)
```

---

## Full Theme Code

```typescript
import { createTheme, alpha } from '@mui/material/styles';

declare module '@mui/material/styles' {
  interface TypeBackground {
    surface: string;
    elevated: string;
    hover: string;
    selected: string;
  }
  interface Palette {
    accent: {
      default: string;
      hover: string;
      subtle: string;
      border: string;
    };
    chart: {
      colors: string[];
      grid: string;
      axis: string;
    };
    status: {
      success: string;
      warning: string;
      error: string;
      info: string;
      muted: string;
    };
  }
  interface PaletteOptions {
    accent?: {
      default?: string;
      hover?: string;
      subtle?: string;
      border?: string;
    };
    chart?: {
      colors?: string[];
      grid?: string;
      axis?: string;
    };
    status?: {
      success?: string;
      warning?: string;
      error?: string;
      info?: string;
      muted?: string;
    };
  }
  interface TypographyVariants {
    mono: React.CSSProperties;
    metric: React.CSSProperties;
    metricSm: React.CSSProperties;
    caption2: React.CSSProperties;
  }
  interface TypographyVariantsOptions {
    mono?: React.CSSProperties;
    metric?: React.CSSProperties;
    metricSm?: React.CSSProperties;
    caption2?: React.CSSProperties;
  }
}

declare module '@mui/material/Typography' {
  interface TypographyPropsVariantOverrides {
    mono: true;
    metric: true;
    metricSm: true;
    caption2: true;
  }
}

const FONTS = {
  ui: '"Inter", system-ui, -apple-system, sans-serif',
  mono: '"JetBrains Mono", "Fira Code", "Cascadia Code", monospace',
};

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
};

export const theme = createTheme({
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
  ] as any,

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
  },
});

export default theme;
```

---

## How to apply in your app

In `src/main.tsx`:

```tsx
import { ThemeProvider, CssBaseline } from '@mui/material';
import { theme } from './theme';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <App />
    </ThemeProvider>
  </React.StrictMode>
);
```

---

## Install fonts in `index.html`

```html
<link rel="preconnect" href="https://fonts.googleapis.com" />
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet" />
```
