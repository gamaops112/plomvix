# Phase 2 — Dashboard Page Spec
## obsAdmin

---

## File Structure

```
src/
├── pages/
│   └── dashboard/
│       ├── index.tsx                  ← main dashboard page
│       ├── components/
│       │   ├── ColorMetricCards.tsx   ← top colored cards (Elastic style)
│       │   ├── StatTiles.tsx          ← flat stat tiles row (Datadog style)
│       │   ├── ServiceHealthGrid.tsx  ← service status grid
│       │   ├── TimeSeriesChart.tsx    ← main time series chart
│       │   ├── RecentAlerts.tsx       ← alert feed panel
│       │   ├── LogsPreview.tsx        ← recent log lines
│       │   ├── TracesPreview.tsx      ← recent traces
│       │   └── ServiceMap.tsx         ← simple service dependency map
│       └── mockData.ts                ← all mock data lives here
```

---

## Page Layout

```
┌─────────────────────────────────────────────────────┐
│  Dashboard          [+ Add Widget]   [time range]   │
├─────────────────────────────────────────────────────┤
│  [COLOR CARD]  [COLOR CARD]  [COLOR CARD]  [COLOR]  │  ← row 1
├─────────────────────────────────────────────────────┤
│  [STAT] [STAT] [STAT] [STAT] [STAT] [STAT] [STAT]  │  ← row 2
├──────────────────────────┬──────────────────────────┤
│                          │                          │
│   TIME SERIES CHART      │   SERVICE HEALTH GRID    │
│   (Request Rate)         │                          │
│                          │                          │
├──────────────────────────┴──────────────────────────┤
│                          │                          │
│   SERVICE MAP            │   RECENT ALERTS          │
│                          │                          │
├──────────────────────────┴──────────────────────────┤
│                          │                          │
│   LOGS PREVIEW           │   TRACES PREVIEW         │
│                          │                          │
└──────────────────────────┴──────────────────────────┘
```

Grid: MUI `<Grid container spacing={2}>` throughout.

---

## Row 1 — Colored Metric Cards (Elastic style)

4 cards, equal width, `xs=3` each.

### Card anatomy
```
┌────────────────────────────┐
│ Label          sparkline   │
│                    ~~~~    │
│ VALUE UNIT              ▲  │
└────────────────────────────┘
```

| Card | Label | Value | Color | Lucide Icon |
|---|---|---|---|---|
| 1 | Total Services | 42 | `#06b6d4` tint | `Layers` |
| 2 | Request Rate | 12.4k/s | `#8b5cf6` tint | `Activity` |
| 3 | Error Rate | 0.8% | `#ef4444` tint | `AlertTriangle` |
| 4 | Avg Latency | 124ms | `#f59e0b` tint | `Timer` |

### Card styling
```
background: linear-gradient(135deg, {color}18 0%, {color}08 100%)
border: 1px solid {color}30
border-radius: 4px
padding: 16px
```

### Sparkline inside each card
- ECharts instance, no axes, no tooltip
- Line color: card accent color
- Height: 48px
- Data: 20 random points, slight upward trend
- Area fill: `{color}20`

### Mock data
```typescript
export const colorCardData = [
  {
    label: 'Total Services',
    value: '42',
    unit: 'services',
    change: '+3',
    changeType: 'positive',
    color: '#06b6d4',
    sparkline: [28,30,29,32,31,35,33,36,38,37,39,40,38,41,40,42,41,43,42,42],
  },
  {
    label: 'Request Rate',
    value: '12.4k',
    unit: 'req/s',
    change: '+8.2%',
    changeType: 'positive',
    color: '#8b5cf6',
    sparkline: [8.1,8.4,9.0,8.8,9.2,10.1,10.8,11.2,10.9,11.5,11.8,12.0,11.7,12.1,12.3,12.4,12.2,12.5,12.3,12.4],
  },
  {
    label: 'Error Rate',
    value: '0.8',
    unit: '%',
    change: '-0.2%',
    changeType: 'positive',
    color: '#ef4444',
    sparkline: [1.2,1.1,1.3,1.0,0.9,1.1,1.0,0.8,0.9,1.0,0.8,0.9,0.7,0.8,0.9,0.8,0.7,0.8,0.9,0.8],
  },
  {
    label: 'Avg Latency',
    value: '124',
    unit: 'ms',
    change: '+12ms',
    changeType: 'negative',
    color: '#f59e0b',
    sparkline: [98,102,108,112,105,118,115,120,118,122,119,124,121,123,125,122,124,126,123,124],
  },
];
```

---

## Row 2 — Flat Stat Tiles (Datadog style)

7 tiles in a row, `xs` flexible. Flat, no color, pure data.

### Tile anatomy
```
┌──────────────┐
│ LABEL        │
│ Value        │
│ ▲ +x% 1h    │
└──────────────┘
```

| Label | Value | Delta |
|---|---|---|
| Hosts | 128 | +2 |
| Containers | 847 | +14 |
| Processes | 2,341 | stable |
| Log Events/s | 8.2k | +1.1k |
| Active Traces | 234 | -12 |
| Open Alerts | 7 | +2 |
| Incidents | 1 | stable |

### Tile styling
```
background: #161b27
border: 1px solid #1f2535
border-radius: 4px
padding: 12px 16px
```

Delta colors:
- Positive number (more hosts = good): `#10b981`
- Negative change on error metrics: `#10b981`
- Positive change on error metrics: `#ef4444`
- Stable: `#4d566b`

---

## Row 3 — Time Series Chart + Service Health Grid

### Time Series Chart (left, `xs=8`)

ECharts line chart showing request rate + error rate over time.

```typescript
export const timeSeriesData = {
  timestamps: // 60 points, last 1 hour, every 1 min
    Array.from({length: 60}, (_, i) => {
      const d = new Date();
      d.setMinutes(d.getMinutes() - (59 - i));
      return d.toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'});
    }),
  requestRate: [/* 60 values between 10000-14000, realistic wave pattern */],
  errorRate:   [/* 60 values between 0.4-1.8, with one spike around index 35 */],
};
```

ECharts config:
```
background: transparent
grid: { top: 32, right: 16, bottom: 32, left: 48 }
xAxis: { type: 'category', axisLine: { lineStyle: { color: '#2a3147' } }, axisLabel: { color: '#4d566b', fontSize: 11 } }
yAxis: { splitLine: { lineStyle: { color: '#1f2535' } }, axisLabel: { color: '#4d566b', fontSize: 11 } }
series[0]: Request Rate — color #06b6d4, smooth: true, areaStyle opacity 0.08
series[1]: Error Rate — color #ef4444, smooth: true, yAxisIndex: 1
tooltip: background #1c2333, border #2a3147, text #e8eaf0
legend: top, text color #8b93a8
```

Card header: "Request Rate & Error Rate" + "last 1h" label on right

### Service Health Grid (right, `xs=4`)

Grid of service health status cards.

```typescript
export const serviceHealthData = [
  { name: 'api-gateway',      status: 'healthy',  latency: '45ms',  uptime: '99.9%' },
  { name: 'auth-service',     status: 'healthy',  latency: '23ms',  uptime: '100%' },
  { name: 'user-service',     status: 'degraded', latency: '312ms', uptime: '99.1%' },
  { name: 'payment-service',  status: 'healthy',  latency: '67ms',  uptime: '99.8%' },
  { name: 'notification-svc', status: 'healthy',  latency: '18ms',  uptime: '100%' },
  { name: 'search-service',   status: 'down',     latency: '—',     uptime: '97.2%' },
  { name: 'storage-service',  status: 'healthy',  latency: '89ms',  uptime: '99.9%' },
  { name: 'cache-service',    status: 'healthy',  latency: '4ms',   uptime: '100%' },
  { name: 'queue-service',    status: 'degraded', latency: '445ms', uptime: '98.7%' },
  { name: 'analytics-svc',    status: 'healthy',  latency: '134ms', uptime: '99.5%' },
];
```

Each service row:
```
● service-name     45ms    99.9%
```
- `●` dot: green/amber/red based on status
- Name: `body2`, mono font
- Latency + uptime: `caption2`, right-aligned
- Row height: 32px
- Hover: `bg.hover`
- Border bottom: `#1f2535`

---

## Row 4 — Service Map + Recent Alerts

### Service Map (left, `xs=7`)

Simple node-graph using ECharts Graph chart.

```typescript
export const serviceMapData = {
  nodes: [
    { id: 'gateway',  name: 'api-gateway',     x: 300, y: 200, symbolSize: 40, status: 'healthy' },
    { id: 'auth',     name: 'auth-service',     x: 150, y: 100, symbolSize: 30, status: 'healthy' },
    { id: 'user',     name: 'user-service',     x: 150, y: 300, symbolSize: 30, status: 'degraded' },
    { id: 'payment',  name: 'payment-service',  x: 450, y: 100, symbolSize: 30, status: 'healthy' },
    { id: 'search',   name: 'search-service',   x: 450, y: 300, symbolSize: 30, status: 'down' },
    { id: 'cache',    name: 'cache-service',    x: 600, y: 200, symbolSize: 25, status: 'healthy' },
    { id: 'storage',  name: 'storage-service',  x: 300, y: 350, symbolSize: 25, status: 'healthy' },
  ],
  edges: [
    { source: 'gateway', target: 'auth' },
    { source: 'gateway', target: 'user' },
    { source: 'gateway', target: 'payment' },
    { source: 'gateway', target: 'search' },
    { source: 'search',  target: 'cache' },
    { source: 'user',    target: 'storage' },
    { source: 'payment', target: 'storage' },
  ],
};
```

Node colors by status:
- healthy: `#10b981`
- degraded: `#f59e0b`
- down: `#ef4444`

Edge color: `#2a3147`
Label color: `#8b93a8`, fontSize 11

### Recent Alerts (right, `xs=5`)

```typescript
export const recentAlertsData = [
  { id: 1, severity: 'critical', title: 'search-service is down',           service: 'search-service',   time: '2m ago',  status: 'firing' },
  { id: 2, severity: 'warning',  title: 'High latency on user-service',     service: 'user-service',     time: '8m ago',  status: 'firing' },
  { id: 3, severity: 'warning',  title: 'Queue depth above threshold',      service: 'queue-service',    time: '15m ago', status: 'firing' },
  { id: 4, severity: 'info',     title: 'Deployment completed',             service: 'auth-service',     time: '22m ago', status: 'resolved' },
  { id: 5, severity: 'critical', title: 'Payment timeout spike',            service: 'payment-service',  time: '34m ago', status: 'resolved' },
  { id: 6, severity: 'info',     title: 'Auto-scaled: added 3 instances',  service: 'api-gateway',      time: '41m ago', status: 'resolved' },
];
```

Each alert row:
```
[●] title                    service    2m ago   [FIRING]
```
- Severity dot: red=critical, amber=warning, cyan=info
- Title: `body2`
- Service: `caption2`, mono
- Time: `caption2`, muted
- Status chip: `<Chip color="error" />` for firing, `<Chip color="default" />` for resolved
- Row height: 40px

---

## Row 5 — Logs Preview + Traces Preview

### Logs Preview (left, `xs=6`)

Last 8 log lines, most recent first. Monospace, dense.

```typescript
export const logsPreviewData = [
  { level: 'ERROR', time: '14:23:01', service: 'search-service',  message: 'Connection refused: redis:6379' },
  { level: 'WARN',  time: '14:22:58', service: 'user-service',    message: 'Response time exceeded 300ms threshold' },
  { level: 'INFO',  time: '14:22:55', service: 'auth-service',    message: 'Token refreshed for user_id=8821' },
  { level: 'INFO',  time: '14:22:54', service: 'api-gateway',     message: 'GET /api/v2/users 200 45ms' },
  { level: 'ERROR', time: '14:22:51', service: 'search-service',  message: 'Timeout after 5000ms waiting for index' },
  { level: 'DEBUG', time: '14:22:49', service: 'cache-service',   message: 'Cache hit ratio: 94.2%' },
  { level: 'INFO',  time: '14:22:47', service: 'payment-service', message: 'Transaction processed: txn_id=pp_9921' },
  { level: 'WARN',  time: '14:22:44', service: 'queue-service',   message: 'Queue depth: 1847 (threshold: 1000)' },
];
```

Row format:
```
[ERROR] 14:23:01  search-service   Connection refused: redis:6379
```

- Level badge: colored chip, compact
- Time + service: mono, `caption2`
- Message: mono, `body2`
- Row height: 28px
- "View all logs →" link at bottom right

### Traces Preview (right, `xs=6`)

Last 6 traces.

```typescript
export const tracesPreviewData = [
  { traceId: 'a3f9c2',  service: 'api-gateway',    operation: 'POST /checkout',      duration: '892ms', status: 'error',   spans: 12 },
  { traceId: 'b7d1e4',  service: 'api-gateway',    operation: 'GET /users/profile',  duration: '124ms', status: 'ok',      spans: 4  },
  { traceId: 'c2a8f7',  service: 'payment-service',operation: 'processPayment',      duration: '445ms', status: 'ok',      spans: 7  },
  { traceId: 'd5b3c1',  service: 'search-service', operation: 'search/query',        duration: '5012ms',status: 'error',   spans: 3  },
  { traceId: 'e9f4a2',  service: 'auth-service',   operation: 'validateToken',       duration: '23ms',  status: 'ok',      spans: 2  },
  { traceId: 'f1c7b9',  service: 'user-service',   operation: 'getUserById',         duration: '312ms', status: 'slow',    spans: 5  },
];
```

Row format:
```
[●] POST /checkout    api-gateway    892ms   12 spans   [ERROR]
```

- Status dot: green=ok, red=error, amber=slow
- Operation: `body2`, mono
- Service: `caption2`, muted
- Duration: `body2`, right-aligned, red if >1000ms
- Spans count: `caption2`, muted
- Status chip
- "View all traces →" link at bottom right

---

## Page Header

```
Dashboard                              [+ Add Widget]  [Edit Dashboard]
Environment: production ▼
```

- Title: `h2`
- Environment selector: `<Select>` with options: production, staging, development
- Buttons: outlined, small

---

## Install ECharts

```bash
npm install echarts echarts-for-react
```

---

## Prompt for Deepseek

```
Read all files in docs/design-system/.
Now read docs/design-system/phase2-dashboard.md.

Build the complete Dashboard page at src/pages/dashboard/index.tsx
with all sub-components as specified.

Rules:
1. Create src/pages/dashboard/mockData.ts with all mock data exactly as specified
2. Build each component file separately as listed in the file structure
3. Use echarts-for-react for all charts — install it first
4. Use MUI Grid for layout, spacing={2}
5. Follow all token values from theme-tokens.md — no hardcoded colors
6. Use theme.palette values everywhere, not raw hex strings in components
7. All chart backgrounds must be transparent
8. Card headers must have a bottom border matching border.subtle token

Do not invent any sections not in the spec.
Do not add animations unless specified.
```
