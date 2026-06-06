# Phase 3 — Logs Explorer Spec
## obsAdmin

---

## File Structure

```
src/
├── pages/
│   └── logs/
│       ├── index.tsx                  ← main logs explorer page
│       ├── components/
│       │   ├── LogsSidebar.tsx        ← left filter panel
│       │   ├── LogsToolbar.tsx        ← query bar + controls above log stream
│       │   ├── MonacoQueryEditor.tsx  ← monaco editor for query input
│       │   ├── LogsHistogram.tsx      ← bar chart showing log volume over time
│       │   ├── LogsTable.tsx          ← virtualized log stream table
│       │   ├── LogRowExpanded.tsx     ← inline expanded row detail
│       │   └── LogDetailDrawer.tsx    ← slide-out right drawer
│       └── mockData.ts                ← mock log data
```

---

## Page Layout

```
┌─────────────────────────────────────────────────────────┐
│  LOGS TOOLBAR (query bar + controls)                    │
├──────────────┬──────────────────────────────────────────┤
│              │  HISTOGRAM (log volume bar chart)        │
│  FILTERS     ├──────────────────────────────────────────┤
│  SIDEBAR     │  LOGS TABLE (virtualized)                │
│  (260px)     │                                          │
│              │  [expanded row if inline detail open]    │
│              │                                          │
└──────────────┴──────────────────────────────────────────┘
                                        [DETAIL DRAWER →]
```

---

## Install Dependencies

```bash
npm install @monaco-editor/react @tanstack/react-virtual
```

---

## Logs Toolbar

Height: 48px, background `#161b27`, border-bottom `1px solid #1f2535`, padding `0 16px`

### Left side
- **Monaco query editor toggle button** — icon `Code2` from lucide, outlined, small
  - When active: opens Monaco editor below toolbar in a 80px tall panel
  - When inactive: shows simple TextField search bar
- **Simple search bar** (default): placeholder `Search logs...`, width `400px`

### Monaco Editor Panel (toggled)
- Height: `80px`
- Background: `#0f1117`
- Border-bottom: `1px solid #2a3147`
- Language: `plaintext` (custom log query syntax)
- Theme: vs-dark with background overridden to `#0f1117`
- Options:
  ```
  minimap: { enabled: false }
  lineNumbers: 'off'
  scrollBeyondLastLine: false
  wordWrap: 'on'
  fontSize: 13
  fontFamily: 'JetBrains Mono'
  padding: { top: 8, bottom: 8 }
  ```
- Placeholder hint inside: `service:auth-service level:ERROR | last 1h`

### Right side controls
- **Log level filter** — multi-select dropdown: ALL / ERROR / WARN / INFO / DEBUG / TRACE
- **Time range** — same Select as topbar: Last 15m, 1h, 6h, 24h, 7d
- **Live tail toggle** — `<Switch>` + "Live" label, when on: auto-scrolls to bottom, new logs stream in
- **Columns button** — icon `Columns` — opens popover to show/hide table columns
- **Download button** — icon `Download` — exports visible logs as JSON

---

## Filters Sidebar

Width: `260px`, background `#161b27`, border-right `1px solid #1f2535`
Overflow-y: auto, padding `12px`

### Filter sections (each collapsible with `<Accordion>`)

#### Services
Checkbox list of services with log count badge
```typescript
[
  { name: 'api-gateway',      count: 24821 },
  { name: 'auth-service',     count: 12043 },
  { name: 'user-service',     count: 8932  },
  { name: 'payment-service',  count: 6721  },
  { name: 'search-service',   count: 5443  },
  { name: 'cache-service',    count: 3211  },
  { name: 'queue-service',    count: 2987  },
  { name: 'storage-service',  count: 1823  },
  { name: 'notification-svc', count: 934   },
  { name: 'analytics-svc',    count: 721   },
]
```

#### Log Levels
Checkbox list with colored indicators
```
● ERROR   3,421
● WARN    8,932
● INFO    52,841
● DEBUG   18,234
● TRACE   4,211
```

#### Hosts
Checkbox list
```
host-prod-01   18,432
host-prod-02   21,043
host-prod-03   17,821
host-staging   9,432
```

#### Kubernetes
```
Namespace: production, staging, monitoring
Pod: (dynamic from logs)
Container: (dynamic from logs)
```

### Filter item styling
```
height: 28px
fontSize: 12px
checkbox: size small, accent color #06b6d4
count badge: caption2, color text.tertiary, right-aligned
hover: bg.hover
```

### Active filters bar
When filters are selected, show a row below toolbar:
```
Filters: [auth-service ×] [ERROR ×] [host-prod-01 ×]   Clear all
```
Chips: small, deletable, background `#1e2438`

---

## Logs Histogram

Height: `80px`, background transparent, no card border
ECharts bar chart showing log volume per minute

```typescript
export const histogramData = {
  // 60 time buckets, last 1 hour
  times:  Array.from({length:60}, (_,i) => `${String(Math.floor(i/60*24)).padStart(2,'0')}:${String(i%60).padStart(2,'0')}`),
  counts: {
    error: [/* 60 low values 0-15, spike at index 35-38 */],
    warn:  [/* 60 values 20-80 */],
    info:  [/* 60 values 200-800 */],
    debug: [/* 60 values 50-200 */],
  }
}
```

ECharts stacked bar config:
```
bar series: error=#ef444488, warn=#f59e0b88, info=#06b6d488, debug=#4d566b88
stack: 'logs'
barMaxWidth: 6
grid: { top: 4, right: 0, bottom: 20, left: 40 }
xAxis: { axisLabel: { color: '#4d566b', fontSize: 10 } }
yAxis: { splitLine: { lineStyle: { color: '#1f2535' } }, axisLabel: { color: '#4d566b', fontSize: 10 } }
tooltip: shared, dark style
```

Clicking a bar filters the time range to that minute.

---

## Logs Table

Uses `@tanstack/react-virtual` for virtualization — must handle 100k+ rows without lag.

### Columns (default visible)
| Column | Width | Content |
|---|---|---|
| Expand | 32px | `>` chevron icon, rotates when expanded |
| Timestamp | 160px | `14:23:01.234` mono, color `text.secondary` |
| Level | 72px | `<Chip>` with log level color |
| Service | 140px | service name, mono, `text.secondary` |
| Message | flex 1 | log message, truncated, mono |
| Host | 120px | hostname, mono, `text.tertiary` |
| Actions | 64px | copy icon + open drawer icon, show on hover |

### Optional columns (shown via Columns button)
- TraceID, SpanID, Pod, Namespace, Container, Duration

### Row styling
```
height: 28px (compact)
font: JetBrains Mono, 12px
border-bottom: 1px solid #1f2535
hover: background #1e2438, show action buttons
cursor: pointer
```

### Log level chip colors
```
ERROR: bg #ef444420, color #ef4444
WARN:  bg #f59e0b20, color #f59e0b
INFO:  bg #06b6d420, color #06b6d4
DEBUG: bg #8b93a820, color #8b93a8
TRACE: bg #4d566b20, color #4d566b
```

### Mock log data
Generate 200 realistic log entries:
```typescript
export const mockLogs = Array.from({ length: 200 }, (_, i) => ({
  id: `log_${i}`,
  timestamp: new Date(Date.now() - i * 3000).toISOString(),
  level: ['ERROR','ERROR','WARN','INFO','INFO','INFO','INFO','DEBUG'][Math.floor(Math.random()*8)],
  service: ['api-gateway','auth-service','user-service','payment-service','search-service','cache-service'][Math.floor(Math.random()*6)],
  host: ['host-prod-01','host-prod-02','host-prod-03'][Math.floor(Math.random()*3)],
  message: [
    'GET /api/v2/users 200 45ms',
    'POST /api/v2/checkout 500 892ms',
    'Token validation failed for user_id=8821',
    'Connection refused: redis:6379',
    'Cache hit ratio: 94.2%',
    'Response time exceeded threshold: 312ms',
    'Transaction processed: txn_id=pp_9921',
    'Timeout after 5000ms waiting for index response',
    'Auto-scaling triggered: cpu=87%',
    'Deployment health check passed',
  ][Math.floor(Math.random()*10)],
  traceId: Math.random().toString(36).substring(2,10),
  spanId:  Math.random().toString(36).substring(2,8),
  pod: `pod-${Math.random().toString(36).substring(2,6)}`,
  namespace: 'production',
}));
```

---

## Log Row — Inline Expanded Detail

When chevron clicked, row expands below with full log details:

```
┌──────────────────────────────────────────────────────────┐
│ ▼  [timestamp]  [LEVEL]  [service]  message...           │
├──────────────────────────────────────────────────────────┤
│  FIELDS                          RAW                     │
│  ─────────────────────────────────────────────────────── │
│  timestamp    2024-01-15T14:23:01.234Z                   │
│  level        ERROR                                      │
│  service      search-service                             │
│  host         host-prod-01                               │
│  message      Connection refused: redis:6379             │
│  traceId      a3f9c2b1                                   │
│  spanId       f4e2d1                                     │
│  pod          pod-x7k2                                   │
│  namespace    production                                 │
│                                                          │
│  [Open in Drawer]  [Copy JSON]  [View Trace →]           │
└──────────────────────────────────────────────────────────┘
```

Expanded area background: `#0f1117`
Field label: `caption`, color `text.secondary`
Field value: `mono`, color `text.primary`
Row padding: `12px 16px 12px 48px`

---

## Log Detail Drawer

MUI `<Drawer anchor="right">`, width `480px`

### Header
```
Log Detail                              [×]
search-service  •  14:23:01.234  •  ERROR
```

### Tabs: Fields | Raw JSON | Trace

#### Fields tab
Two-column layout:
```
FIELD          VALUE
timestamp      2024-01-15T14:23:01.234Z
level          ERROR
service        search-service
...
```

#### Raw JSON tab
Monaco editor, read-only, language `json`, height fills drawer
Shows full log object pretty-printed

#### Trace tab
If traceId exists:
- Shows mini trace summary — service, operation, duration
- Button "Open full trace in Traces →" — navigates to /traces page

### Footer
```
[← Previous log]              [Next log →]
```

---

## Live Tail Mode

When Live toggle is ON:
- New mock log prepended every 800ms using `setInterval`
- Auto-scroll to top (newest first)
- "LIVE" badge pulsing in toolbar (red dot + "LIVE" text)
- Pause on hover over table (stop auto-scroll while reading)

---

## Prompt for Deepseek

```
Read all files in docs/design-system/.
Now read docs/design-system/phase3-logs.md.

Build the complete Logs Explorer at src/pages/logs/index.tsx
with all sub-components as specified.

Install required packages first:
npm install @monaco-editor/react @tanstack/react-virtual

Rules:
1. mockData.ts — generate 200 log entries as specified
2. LogsSidebar.tsx — collapsible accordion filter sections
3. MonacoQueryEditor.tsx — toggled Monaco panel, vs-dark theme, custom bg
4. LogsHistogram.tsx — stacked ECharts bar chart, 60 buckets
5. LogsTable.tsx — TanStack Virtual, 28px rows, hover actions
6. LogRowExpanded.tsx — inline expanded detail with fields + action buttons
7. LogDetailDrawer.tsx — right drawer, 480px, 3 tabs (Fields/Raw/Trace)
8. Wire live tail with setInterval, prepend new log every 800ms when active
9. Active filters bar below toolbar when filters selected
10. Use JetBrains Mono for all log content
11. All colors from theme tokens only — no hardcoded hex

Do not add features not in the spec.
```
