# Phase 7 Fix — Dark/Light Mode Partial Application
## obsAdmin

---

## Root Cause

Components are using hardcoded hex color strings instead of
`theme.palette` tokens. When ThemeProvider switches from
darkTheme to lightTheme, these components don't update.

---

## The Fix Rule

**Every component must use `sx` prop or `useTheme()` for colors.
Zero hardcoded hex strings in component files.**

---

## Fix 1 — ColorMetricCards (Dashboard)

Problem: gradient backgrounds use hardcoded hex.

```typescript
// WRONG — hardcoded
background: 'linear-gradient(135deg, #06b6d418 0%, #06b6d408 100%)'
border: '1px solid #06b6d430'

// CORRECT — use alpha() from MUI
import { alpha, useTheme } from '@mui/material/styles';

const theme = useTheme();

background: `linear-gradient(135deg, 
  ${alpha(cardColor, 0.08)} 0%, 
  ${alpha(cardColor, 0.03)} 100%)`
border: `1px solid ${alpha(cardColor, 0.18)}`
```

---

## Fix 2 — All ECharts Components

Every ECharts instance must read colors from theme.

```typescript
// Add this pattern to EVERY chart component:
import { useTheme } from '@mui/material/styles';

const theme = useTheme();

const chartOptions = {
  backgroundColor: 'transparent',
  grid: {
    top: 32, right: 16, bottom: 32, left: 48
  },
  xAxis: {
    axisLine: { lineStyle: { color: theme.palette.divider } },
    axisLabel: { color: theme.palette.text.disabled, fontSize: 11 },
    splitLine: { show: false },
  },
  yAxis: {
    splitLine: { lineStyle: { color: theme.palette.divider } },
    axisLabel: { color: theme.palette.text.disabled, fontSize: 11 },
  },
  tooltip: {
    backgroundColor: theme.palette.background.elevated,
    borderColor: theme.palette.divider,
    textStyle: { color: theme.palette.text.primary, fontSize: 12 },
  },
  legend: {
    textStyle: { color: theme.palette.text.secondary },
  },
};
```

Apply this fix to ALL chart components:
- TimeSeriesChart.tsx
- LogsHistogram.tsx
- MetricsExplorerChart.tsx
- MetricChart.tsx (host detail)
- LatencyChart.tsx (APM)
- SpanWaterfall timeline
- AlertDetailDrawer metric chart
- ServiceMap (ECharts graph)
- All sparklines in ColorMetricCards + StatTiles

---

## Fix 3 — Inline sx hardcoded colors

Search entire `src/` for these patterns and replace:

```
// Find and replace all of these:
color: '#e8eaf0'      → color: 'text.primary'
color: '#8b93a8'      → color: 'text.secondary'
color: '#4d566b'      → color: 'text.disabled'
color: '#0f1117'      → color: 'background.default'
background: '#161b27' → background: 'background.paper'
background: '#1c2333' → background: 'background.elevated'
background: '#1e2438' → background: 'background.hover'
background: '#0f1117' → background: 'background.default'
borderColor: '#1f2535'→ borderColor: 'divider'
borderColor: '#2a3147'→ borderColor: 'divider'
```

For sx props use string tokens:
```typescript
// CORRECT in sx prop
sx={{ color: 'text.secondary', background: 'background.paper' }}

// CORRECT in useTheme()
const theme = useTheme();
style={{ color: theme.palette.text.secondary }}
```

---

## Fix 4 — LogsTable row colors

```typescript
// WRONG
'& .log-level-error': { color: '#ef4444' }

// CORRECT — use theme
const theme = useTheme();
'& .log-level-error': { color: theme.palette.error.main }
'& .log-level-warn':  { color: theme.palette.warning.main }
'& .log-level-info':  { color: theme.palette.primary.main }
'& .log-level-debug': { color: theme.palette.text.secondary }
'& .log-level-trace': { color: theme.palette.text.disabled }
```

---

## Fix 5 — SpanWaterfall bar colors

```typescript
// Service colors must still use the fixed palette
// BUT background/text around bars must use theme

// KEEP hardcoded service colors (these don't change with theme):
export const serviceColors: Record<string, string> = {
  'api-gateway': '#06b6d4',
  // ... etc
};

// FIX the surrounding UI:
// Row background, text, borders → all use theme tokens
sx={{
  borderBottom: `1px solid ${theme.palette.divider}`,
  '&:hover': { background: theme.palette.background.hover },
  color: theme.palette.text.primary,
}}
```

---

## Fix 6 — Sidebar & Topbar

```typescript
// These are in MUI component overrides in lightTheme
// so they should work — but verify these sx overrides
// in Sidebar.tsx and Topbar.tsx use theme tokens:

// Active nav item
sx={{
  background: isActive
    ? theme.palette.background.selected
    : 'transparent',
  color: isActive
    ? theme.palette.text.primary
    : theme.palette.text.secondary,
  borderLeft: isActive
    ? `2px solid ${theme.palette.primary.main}`
    : '2px solid transparent',
  '&:hover': {
    background: theme.palette.background.hover,
    color: theme.palette.text.primary,
  }
}}
```

---

## Fix 7 — Section labels in Sidebar

```typescript
// WRONG
color: '#4d566b'

// CORRECT
color: 'text.disabled'   // in sx prop
// or
theme.palette.text.disabled  // in useTheme()
```

---

## Fix 8 — Cards with custom backgrounds

Stat tiles, info boxes, demo credentials box:

```typescript
// WRONG
background: '#06b6d415'
border: '1px solid #06b6d440'

// CORRECT
import { alpha } from '@mui/material/styles';
background: alpha(theme.palette.primary.main, 0.08)
border: `1px solid ${alpha(theme.palette.primary.main, 0.25)}`
```

---

## Fix 9 — Monaco Editor theme

```typescript
// Monaco must respond to theme mode
import { useTheme } from '@mui/material/styles';
const theme = useTheme();

<MonacoEditor
  theme={theme.palette.mode === 'dark' ? 'vs-dark' : 'light'}
  options={{
    // override background to match app
    ...options,
  }}
/>

// Also override Monaco background via beforeMount:
const handleBeforeMount = (monaco: Monaco) => {
  monaco.editor.defineTheme('obsadmin-dark', {
    base: 'vs-dark',
    inherit: true,
    rules: [],
    colors: {
      'editor.background': '#0f1117',
    },
  });
  monaco.editor.defineTheme('obsadmin-light', {
    base: 'vs',
    inherit: true,
    rules: [],
    colors: {
      'editor.background': '#f9fafb',
    },
  });
};

// Then use:
theme={theme.palette.mode === 'dark' ? 'obsadmin-dark' : 'obsadmin-light'}
```

---

## Verification Checklist

After applying fixes, toggle theme and verify each page:

| Page | Check |
|---|---|
| Dashboard | Colored cards visible in light mode, charts update |
| Logs | Table rows readable, histogram colors update |
| Metrics | Host table, chart grid lines update |
| Traces | Waterfall text readable, row borders update |
| APM | Service cards, latency chart update |
| Alerts | Table rows, severity chips readable |
| Settings | All form fields readable |
| Login | Both panels readable in both modes |

---

## Prompt for Deepseek

```
Read docs/design-system/phase7-darkmode-fix.md.

The dark/light theme toggle is partially working.
The issue is hardcoded hex color strings in components.

Apply ALL fixes in order:

1. Install pattern: add `const theme = useTheme()` to every
   component that uses colors

2. Fix ALL ECharts instances — add theme-aware options as
   specified in Fix 2. Apply to every single chart component.

3. Global find+replace hardcoded hex strings in sx props
   as specified in Fix 3

4. Fix ColorMetricCards gradients using alpha() — Fix 1

5. Fix LogsTable level colors — Fix 4

6. Fix SpanWaterfall surrounding UI — Fix 5

7. Fix Sidebar active state — Fix 6

8. Fix all alpha/tinted backgrounds — Fix 8

9. Fix Monaco Editor theme switching — Fix 9

IMPORTANT:
- Never remove service colors from serviceColors map
- Never change status colors (success/warning/error) — these
  are semantic and stay consistent across both themes
- chart.colors array stays the same in both themes
- Only UI chrome colors need to respond to theme

After fixing, toggle between dark and light on every page
and verify no hardcoded colors remain.
```
