# Phase 5 — Traces & APM Spec
## obsAdmin

---

## File Structure

```
src/
├── pages/
│   ├── traces/
│   │   ├── index.tsx                    ← traces list page with tabs
│   │   ├── components/
│   │   │   ├── TracesToolbar.tsx        ← filters + search bar
│   │   │   ├── TracesTable.tsx          ← trace list table
│   │   │   ├── TraceDetailDrawer.tsx    ← slide-out waterfall drawer
│   │   │   ├── SpanWaterfall.tsx        ← waterfall chart component
│   │   │   ├── SpanRow.tsx              ← single span row in waterfall
│   │   │   └── TraceDetailPage.tsx      ← full page trace detail
│   │   └── mockData.ts
│   └── apm/
│       ├── index.tsx                    ← APM overview page
│       ├── components/
│       │   ├── ServiceOverviewCards.tsx ← service health cards
│       │   ├── ServiceDetail.tsx        ← single service detail
│       │   ├── ApmServiceMap.tsx        ← service dependency map
│       │   ├── ErrorTracking.tsx        ← error groups table
│       │   └── LatencyChart.tsx         ← p50/p95/p99 chart
│       └── mockData.ts
```

---

## Page 1 — Traces Explorer `/traces`

### Tabs
```
[Overview]  [Traces]  [Service Map]
```

---

### Tab 1 — Overview (APM summary inside Traces)

4 stat cards row:
| Card | Value | Color |
|---|---|---|
| Total Traces | 24,821 | `#06b6d4` |
| Error Traces | 1,842 | `#ef4444` |
| Avg Duration | 234ms | `#8b5cf6` |
| Throughput | 412/min | `#10b981` |

Below: 2-column layout

**Left (xs=8)** — Latency distribution chart
- ECharts line chart
- 3 series: P50, P95, P99
- Colors: `#10b981`, `#f59e0b`, `#ef4444`
- Time range: last 1h, 60 points
- Y-axis: milliseconds
- Title: "Latency Percentiles"

**Right (xs=4)** — Top error services
```
SERVICE             ERRORS   ERROR%
search-service      1,241    8.4%
user-service        312      1.8%
payment-service     89       0.2%
api-gateway         67       0.4%
queue-service       71       0.3%
```
Error% colored: > 5% red, 1-5% amber, < 1% default

---

### Tab 2 — Traces List

#### Toolbar
```
[Search traces...]  [Service ▼]  [Operation ▼]  [Status ▼]  [Duration ▼]  [time range]
```

Filter options:
- Service: all services from mock data
- Operation: HTTP GET, HTTP POST, DB Query, Cache, gRPC
- Status: All / Error / Slow (>1s) / OK
- Duration: Any / <100ms / 100-500ms / 500ms-1s / >1s

#### Traces Table

Columns:
| Column | Width | Content |
|---|---|---|
| Status | 32px | colored dot |
| Trace ID | 100px | first 8 chars, mono, cyan |
| Root Service | 140px | service name, mono |
| Root Operation | 200px | operation name |
| Duration | 100px | total duration, colored if slow |
| Spans | 72px | span count |
| Errors | 72px | error span count, red if > 0 |
| Start Time | 140px | timestamp, mono |
| Actions | 64px | drawer + page icons on hover |

Row height: 36px
Click row → opens TraceDetailDrawer
Click Trace ID → navigates to `/traces/:id`

Status dot colors:
- error: `#ef4444`
- slow (>1s): `#f59e0b`
- ok: `#10b981`

#### Mock traces data
```typescript
export const mockTraces = [
  { id: 'a3f9c2b1', rootService: 'api-gateway',     rootOp: 'POST /checkout',        duration: 892,  spans: 12, errors: 2, status: 'error',  time: '14:23:01' },
  { id: 'b7d1e4c2', rootService: 'api-gateway',     rootOp: 'GET /users/profile',    duration: 124,  spans: 4,  errors: 0, status: 'ok',     time: '14:22:58' },
  { id: 'c2a8f7d3', rootService: 'payment-service', rootOp: 'processPayment',        duration: 445,  spans: 7,  errors: 0, status: 'ok',     time: '14:22:55' },
  { id: 'd5b3c1e4', rootService: 'search-service',  rootOp: 'search/query',          duration: 5012, spans: 3,  errors: 1, status: 'error',  time: '14:22:51' },
  { id: 'e9f4a2b5', rootService: 'auth-service',    rootOp: 'validateToken',         duration: 23,   spans: 2,  errors: 0, status: 'ok',     time: '14:22:49' },
  { id: 'f1c7b9d6', rootService: 'user-service',    rootOp: 'getUserById',           duration: 1312, spans: 5,  errors: 0, status: 'slow',   time: '14:22:47' },
  { id: 'g4e2a8c7', rootService: 'api-gateway',     rootOp: 'GET /products',         duration: 67,   spans: 3,  errors: 0, status: 'ok',     time: '14:22:44' },
  { id: 'h8d1f3b8', rootService: 'api-gateway',     rootOp: 'DELETE /cart/item',     duration: 234,  spans: 6,  errors: 0, status: 'ok',     time: '14:22:41' },
  { id: 'i2b9e7a9', rootService: 'notification-svc',rootOp: 'sendEmailNotification', duration: 891,  spans: 4,  errors: 1, status: 'error',  time: '14:22:38' },
  { id: 'j6a4c2f0', rootService: 'payment-service', rootOp: 'refundTransaction',     duration: 2341, spans: 9,  errors: 0, status: 'slow',   time: '14:22:35' },
  { id: 'k1f8b6d1', rootService: 'api-gateway',     rootOp: 'GET /orders',           duration: 189,  spans: 5,  errors: 0, status: 'ok',     time: '14:22:32' },
  { id: 'l5c3a9e2', rootService: 'search-service',  rootOp: 'indexDocument',         duration: 3421, spans: 2,  errors: 1, status: 'error',  time: '14:22:29' },
];
```

---

### Tab 3 — Service Map

Reuse and enhance the service map from Dashboard.
Add:
- Click node → shows service detail popover
- Edge thickness = request volume
- Edge color = error rate (green=ok, amber=degraded, red=high errors)
- Node size = request rate
- Legend: node size = traffic, edge color = health

Service detail popover on node click:
```
┌─────────────────────────┐
│ search-service    [×]   │
│ ─────────────────────── │
│ Req/s      443          │
│ Error%     8.4%  🔴     │
│ P99        5.0s  🔴     │
│ Instances  3            │
│                         │
│ [View Service →]        │
└─────────────────────────┘
```

---

## Span Waterfall Component

This is the most complex component. Build it carefully.

### Layout
```
TRACE: a3f9c2b1  •  POST /checkout  •  892ms  •  12 spans  •  2 errors

Timeline header:  0ms    200ms    400ms    600ms    800ms
─────────────────────────────────────────────────────────
▼ api-gateway          POST /checkout          ████████████  892ms
  ▼ auth-service         validateToken           ██  45ms
  ▼ user-service         getUserById           ████  312ms
      cache-service        cache.get             █  8ms
      storage-service      db.query           ████  287ms  ❌
  ▼ payment-service      processPayment          ██████  445ms
      storage-service      db.query           █████  398ms
  ▼ search-service       search/query                ████████  5012ms  ❌
      cache-service        cache.get             █  4ms
```

### Span row anatomy
```
[indent] [▼/▶] [service name]  [operation]    [          bar          ]  [duration]  [error?]
```

- Indent: 16px per nesting level
- Expand/collapse: spans with children have chevron
- Service name: 140px, mono, `text.secondary`
- Operation: 180px, mono, `text.primary`
- Bar: fills remaining width based on start offset + duration relative to trace total
- Bar color: service color from chart palette (each service gets consistent color)
- Error spans: bar color `#ef4444`, error icon at end
- Duration: 72px right-aligned, mono, red if > 1s
- Row height: 32px
- Hover: `bg.hover`

### Timeline header
- Shows time markers at 0%, 25%, 50%, 75%, 100% of trace duration
- Color: `text.tertiary`
- Vertical grid lines: `border.subtle`

### Mock span tree for trace a3f9c2b1
```typescript
export const mockSpanTree = {
  traceId: 'a3f9c2b1',
  totalDuration: 892,
  rootSpan: {
    id: 's1', service: 'api-gateway', operation: 'POST /checkout',
    startOffset: 0, duration: 892, status: 'error', depth: 0,
    children: [
      {
        id: 's2', service: 'auth-service', operation: 'validateToken',
        startOffset: 12, duration: 45, status: 'ok', depth: 1,
        children: []
      },
      {
        id: 's3', service: 'user-service', operation: 'getUserById',
        startOffset: 58, duration: 312, status: 'ok', depth: 1,
        children: [
          {
            id: 's4', service: 'cache-service', operation: 'cache.get',
            startOffset: 62, duration: 8, status: 'ok', depth: 2, children: []
          },
          {
            id: 's5', service: 'storage-service', operation: 'db.query',
            startOffset: 72, duration: 287, status: 'error', depth: 2, children: []
          },
        ]
      },
      {
        id: 's6', service: 'payment-service', operation: 'processPayment',
        startOffset: 380, duration: 445, status: 'ok', depth: 1,
        children: [
          {
            id: 's7', service: 'storage-service', operation: 'db.query',
            startOffset: 390, duration: 398, status: 'ok', depth: 2, children: []
          },
        ]
      },
      {
        id: 's8', service: 'search-service', operation: 'search/query',
        startOffset: 124, duration: 512, status: 'error', depth: 1,
        children: [
          {
            id: 's9', service: 'cache-service', operation: 'cache.get',
            startOffset: 128, duration: 4, status: 'ok', depth: 2, children: []
          },
        ]
      },
    ]
  }
};
```

### Service colors (consistent across waterfall)
```typescript
export const serviceColors: Record<string, string> = {
  'api-gateway':      '#06b6d4',
  'auth-service':     '#8b5cf6',
  'user-service':     '#10b981',
  'payment-service':  '#f59e0b',
  'search-service':   '#f97316',
  'cache-service':    '#ec4899',
  'storage-service':  '#a78bfa',
  'notification-svc': '#34d399',
  'queue-service':    '#fbbf24',
  'analytics-svc':    '#60a5fa',
};
```

---

## Trace Detail Drawer

Width: `640px`, MUI `<Drawer anchor="right">`

### Header
```
Trace: a3f9c2b1                    [Open full page →]  [×]
POST /checkout  •  892ms  •  12 spans  •  2 errors
api-gateway  •  14:23:01.234
```

### Tabs: Waterfall | Spans | Logs | Summary

#### Waterfall tab
Full SpanWaterfall component inside drawer, scrollable

#### Spans tab
Flat list of all spans as table:
| Span ID | Service | Operation | Duration | Start | Status |
Each row expandable to show span tags/attributes

#### Logs tab
Log lines correlated to this trace — filter by traceId
Reuse LogsTable with traceId pre-filter

#### Summary tab
```
TRACE INFO
Trace ID        a3f9c2b1
Root Service    api-gateway
Root Operation  POST /checkout
Start Time      2024-01-15 14:23:01.234
Duration        892ms
Total Spans     12
Error Spans     2
Status          Error

SPAN BREAKDOWN
api-gateway      1 span    892ms
auth-service     1 span    45ms
user-service     1 span    312ms
cache-service    2 spans   12ms
storage-service  2 spans   685ms  ← slowest
payment-service  1 span    445ms
search-service   1 span    512ms
```

---

## Trace Detail Full Page

Route: `/traces/:id`

### Layout
```
← Back to Traces

Trace: a3f9c2b1                              [Copy ID]  [Share]
POST /checkout  •  api-gateway  •  892ms  •  14:23:01

┌──────────────┬──────────────┬──────────────┬──────────────┐
│ Duration     │ Spans        │ Errors       │ Services     │
│ 892ms        │ 12           │ 2            │ 7            │
└──────────────┴──────────────┴──────────────┴──────────────┘

[Waterfall]  [Spans]  [Logs]  [Summary]       ← tabs

[Full SpanWaterfall — fills remaining height]
```

Selected span detail panel (appears below waterfall when span clicked):
```
┌─────────────────────────────────────────────────────┐
│ storage-service  •  db.query  •  287ms  •  ERROR    │
├─────────────────────────────────────────────────────┤
│ ATTRIBUTES                    EVENTS                │
│ db.system    postgresql        error  Connection... │
│ db.name      users_db          ...                  │
│ db.operation SELECT                                 │
│ net.peer     postgres:5432                          │
│ span.kind    client                                 │
└─────────────────────────────────────────────────────┘
```

---

## Page 2 — APM Overview `/apm`

### Page header
```
APM — Application Performance          Environment: production ▼
```

### Section 1 — Service Overview Cards

Grid of service cards, 3 per row:

```
┌────────────────────────────────┐
│ ● api-gateway         healthy  │
│                                │
│ 4,821 req/s   0.4% errors      │
│                                │
│ P50  45ms  P95  124ms          │
│ [sparkline request rate]       │
│                                │
│ [View Service →]               │
└────────────────────────────────┘
```

```typescript
export const mockServices = [
  { name: 'api-gateway',      status: 'healthy',  reqRate: 4821, errorRate: 0.4, p50: 45,   p95: 124,  p99: 312,  instances: 3 },
  { name: 'auth-service',     status: 'healthy',  reqRate: 2043, errorRate: 0.1, p50: 23,   p95: 67,   p99: 98,   instances: 2 },
  { name: 'user-service',     status: 'degraded', reqRate: 1932, errorRate: 1.8, p50: 312,  p95: 891,  p99: 1200, instances: 2 },
  { name: 'payment-service',  status: 'healthy',  reqRate: 721,  errorRate: 0.2, p50: 67,   p95: 198,  p99: 445,  instances: 2 },
  { name: 'search-service',   status: 'down',     reqRate: 443,  errorRate: 8.4, p50: 5012, p95: null, p99: null, instances: 1 },
  { name: 'cache-service',    status: 'healthy',  reqRate: 8211, errorRate: 0.0, p50: 4,    p95: 12,   p99: 18,   instances: 3 },
  { name: 'queue-service',    status: 'degraded', reqRate: 987,  errorRate: 0.3, p50: 445,  p95: 892,  p99: 1400, instances: 2 },
  { name: 'storage-service',  status: 'healthy',  reqRate: 3421, errorRate: 0.1, p50: 89,   p95: 234,  p99: 445,  instances: 4 },
  { name: 'notification-svc', status: 'healthy',  reqRate: 234,  errorRate: 0.8, p50: 18,   p95: 45,   p99: 98,   instances: 1 },
];
```

Card status border-left:
- healthy: `#10b981`
- degraded: `#f59e0b`
- down: `#ef4444`

### Section 2 — Service Map (full width)
Same as traces tab service map, height `360px`

### Section 3 — Error Tracking

Table of error groups:
```
ERROR                              SERVICE           COUNT   USERS   FIRST SEEN   LAST SEEN
Connection refused: redis:6379     search-service    1,241   —       2d ago       2m ago
Response timeout after 5000ms      user-service      312     89      5h ago       8m ago
Payment gateway rejected charge    payment-service   89      67      1d ago       34m ago
JWT token signature invalid        auth-service      34      34      3h ago       1h ago
```

Columns: error message (truncated 60 chars), service chip, count, affected users, first seen, last seen
Row click → opens error detail drawer showing stack trace + affected traces

### Error detail drawer (420px)
```
Connection refused: redis:6379         [×]
search-service  •  1,241 occurrences

STACK TRACE
Error: connect ECONNREFUSED 127.0.0.1:6379
  at TCPConnectWrap.afterConnect [as oncomplete]
  at net.js:1141:16

AFFECTED TRACES (showing 5)
[trace list — reuse TracesTable mini version]

TAGS
error.type    ConnectionRefusedError
db.system     redis
net.peer      redis:6379
```

Stack trace: Monaco editor read-only, language `plaintext`, height `200px`

---

## APM Tabs inside `/traces`

The Overview tab at `/traces` shows a condensed version of the APM data:
- 4 stat cards
- Latency chart
- Top erroring services list

This shares the same mock data as `/apm` — import from apm/mockData.

---

## Prompt for Deepseek

```
Read all files in docs/design-system/.
Now read docs/design-system/phase5-traces-apm.md.

This is the most complex phase. Build in this order:

PART A — Traces page
1. src/pages/traces/mockData.ts — all mock data as specified
2. src/pages/traces/components/TracesToolbar.tsx
3. src/pages/traces/components/TracesTable.tsx
4. src/pages/traces/components/SpanRow.tsx — single span row with bar
5. src/pages/traces/components/SpanWaterfall.tsx — recursive tree of SpanRows
6. src/pages/traces/components/TraceDetailDrawer.tsx — 640px, 4 tabs
7. src/pages/traces/components/TraceDetailPage.tsx — full page at /traces/:id
8. src/pages/traces/index.tsx — 3 tabs: Overview, Traces, Service Map

PART B — APM page
9. src/pages/apm/mockData.ts
10. src/pages/apm/components/ServiceOverviewCards.tsx — 3-col grid
11. src/pages/apm/components/ApmServiceMap.tsx — enhanced with click popover
12. src/pages/apm/components/ErrorTracking.tsx — error groups table + drawer
13. src/pages/apm/components/LatencyChart.tsx — p50/p95/p99 lines
14. src/pages/apm/index.tsx — full APM page

PART C — Router
15. Add /traces/:id and /apm routes to App.tsx

CRITICAL RULES for SpanWaterfall:
- Bar width = (span.duration / trace.totalDuration) * 100%
- Bar left offset = (span.startOffset / trace.totalDuration) * 100%
- Each service gets a consistent color from serviceColors map
- Error spans: bar color #ef4444
- Recursive rendering for nested children
- Collapse/expand works per span node
- Timeline header shows 5 time markers

Do not skip any component. Do not invent features not in spec.
```
