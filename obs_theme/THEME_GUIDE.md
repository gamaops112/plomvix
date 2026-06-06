# Adapting obsAdmin Theme to a New Observability Project

## Step 1: Core Files

Copy these files into your project:

```
src/
├── theme/index.ts        # Dark + Light themes, getDensityTokens
├── store/
│   ├── uiStore.ts        # Theme mode toggle
│   └── settingsStore.ts  # User preferences
├── lib/
│   └── toast.ts          # Sonner wrapper
├── components/common/
│   ├── ThemeToggle.tsx   # Sun/Moon button
│   └── EmptyState.tsx    # Optional reusable empty state
```

## Step 2: Install Dependencies

```bash
npm install @mui/material @mui/icons-material @emotion/react @emotion/styled \
  react-router-dom zustand dayjs lucide-react sonner
```

## Step 3: Wrap Your App

```tsx
// main.tsx
import { darkTheme, lightTheme } from './theme';
import { useUIStore } from './store/uiStore';
import { ThemeProvider, CssBaseline } from '@mui/material';

function ThemedApp() {
  const { themeMode } = useUIStore();
  const theme = themeMode === 'dark' ? darkTheme : lightTheme;
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <YourApp />
    </ThemeProvider>
  );
}
```

## Step 4: Customize the Theme

Edit `src/theme/index.ts` to match your brand colors:

```tsx
// In COLORS object, change:
accent: { default: '#YOUR_BRAND_COLOR', hover: '#YOUR_DARKER_COLOR', ... }
primary: { main: '#YOUR_BRAND_COLOR', dark: '#YOUR_DARKER_COLOR', ... }
```

Update the matching light theme palette:
```tsx
primary: { main: '#DARKER_VARIANT_FOR_LIGHT_MODE', ... }
```

## Step 5: Component Patterns

### Use Theme Tokens in SX

```tsx
// ✅ Good — responds to theme mode
<Box sx={{ bgcolor: 'background.paper', border: 1, borderColor: 'divider' }}>
  <Typography variant="body2" sx={{ color: 'text.secondary' }}>Label</Typography>
  <Typography sx={{ color: 'text.primary' }}>Value</Typography>
</Box>

// ❌ Bad — stays dark in light mode
<Box sx={{ background: '#161b27', border: '1px solid #1f2535' }}>
```

### Cards

```tsx
<Card sx={{ border: 1, borderColor: 'divider' }}>
  <CardHeader title="Title" sx={{ borderBottom: 1, borderColor: 'divider' }} />
  <CardContent>Content</CardContent>
</Card>
```

### Tables

```tsx
<TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
  <Table size="small">
    <TableHead>
      <TableRow>
        <TableCell sx={{ color: 'text.secondary', borderBottom: 1, borderColor: 'divider' }}>
          Column
        </TableCell>
      </TableRow>
    </TableHead>
    <TableBody>
      <TableRow hover>
        <TableCell sx={{ color: 'text.primary', borderBottom: 1, borderColor: 'divider' }}>
          Data
        </TableCell>
      </TableRow>
    </TableBody>
  </Table>
</TableContainer>
```

### ECharts Integration

```tsx
import ReactEChartsCore from 'echarts-for-react/esm/core';
import * as echarts from 'echarts/core';
import { LineChart } from 'echarts/charts';
import { GridComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import { useTheme } from '@mui/material/styles';

echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer]);

function MyChart() {
  const theme = useTheme();
  const option = useMemo(() => ({
    tooltip: {
      backgroundColor: theme.palette.background.elevated,
      borderColor: theme.palette.divider,
      textStyle: { color: theme.palette.text.primary },
    },
    grid: { top: 16, right: 16, bottom: 32, left: 48 },
    xAxis: {
      axisLabel: { color: theme.palette.text.disabled, fontSize: 11 },
      axisLine: { lineStyle: { color: theme.palette.divider } },
    },
    yAxis: {
      splitLine: { lineStyle: { color: theme.palette.divider } },
      axisLabel: { color: theme.palette.text.disabled, fontSize: 11 },
    },
    series: [{
      type: 'line', data: yourData, smooth: true, symbol: 'none',
      lineStyle: { color: theme.palette.primary.main, width: 2 },
    }],
  }), [theme, yourData]);

  return <ReactEChartsCore echarts={echarts} option={option} style={{ height: 280 }} notMerge />;
}
```

## Semantic Color Map

These tokens are for meaning, not decoration. Never use them for visual variety:

| Token | Color | Use For |
|---|---|---|
| `success.main` | `#10b981` | Healthy, operational, OK |
| `warning.main` | `#f59e0b` | Degraded, slow, at risk |
| `error.main` | `#ef4444` | Down, critical, failing |
| `info.main` | `#06b6d4` | Informational, neutral alerts |
| `primary.main` | `#06b6d4` | Interactive elements, accent, brand |

For chart series, use the chart color palette (6 colors, ordered by priority):
```
'#06b6d4' '#8b5cf6' '#f59e0b' '#10b981' '#f97316' '#ec4899'
```

## Layout Pattern

If you want the same sidebar + topbar layout:

```
┌─────────────────────────────────────────────┐
│               TOPBAR (48px)                  │
├──────────────┬──────────────────────────────┤
│   SIDEBAR    │       BREADCRUMBS (32px)      │
│  (220/56px)  ├──────────────────────────────┤
│              │                              │
│              │       CONTENT AREA           │
│              │       (scrollable)           │
└──────────────┴──────────────────────────────┘
```

```tsx
// AppShell.tsx pattern
const sidebarWidth = sidebarCollapsed ? 56 : 220;

<Box sx={{ minHeight: '100vh', bgcolor: 'background.default' }}>
  <Topbar />                              {/* fixed, top: 0, z-index: 1100 */}
  <Sidebar />                             {/* fixed, top: 48, z-index: 1000 */}
  {/* Breadcrumbs */}
  <Box sx={{ position: 'fixed', top: 48, left: sidebarWidth, right: 0, height: 32, ... }}>
    <Breadcrumbs>...</Breadcrumbs>
  </Box>
  {/* Content */}
  <Box sx={{ mt: '80px', ml: `${sidebarWidth}px`, ... }}>
    {children}
  </Box>
</Box>
```

## Density System Integration

```tsx
import { useSettingsStore } from '../store/settingsStore';
import { getDensityTokens } from '../theme';

function MyTable() {
  const { density } = useSettingsStore();
  const { tableRowHeight } = getDensityTokens(density);
  // Use tableRowHeight for row sizing
}
```

## Key Conventions

- No box-shadow on cards or panels
- No border-radius above 6px
- No `fontWeight: 700` — max is 600
- No font-size below 11px
- Always multiples of 4 for spacing
- Log lines, trace IDs, timestamps in JetBrains Mono
- Metric values in `metric` or `metricSm` typography variants
- Status colors exclusively for their documented meaning
