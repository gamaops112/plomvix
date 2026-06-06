# Phase 4 — Metrics & Infrastructure Spec
## obsAdmin

---

## File Structure

```
src/
├── pages/
│   └── metrics/
│       ├── index.tsx                      ← main metrics page (tabs)
│       ├── components/
│       │   ├── InfrastructureTab.tsx      ← hosts view tab
│       │   ├── MetricsExplorerTab.tsx     ← query builder tab
│       │   ├── HostsColorCards.tsx        ← colored summary cards (Elastic style)
│       │   ├── HostsTable.tsx             ← host inventory table with sparklines
│       │   ├── HostDetailDrawer.tsx       ← slide-out host detail (drawer)
│       │   ├── HostDetailPage.tsx         ← full host detail page
│       │   ├── MetricChart.tsx            ← reusable single metric chart
│       │   ├── MetricsExplorerChart.tsx   ← main explorer chart
│       │   └── MetricsQueryBar.tsx        ← metric query builder controls
│       └── mockData.ts
```

---

## Page Header + Tabs

```
Metrics & Infrastructure

[Infrastructure]  [Metrics Explorer]          ← MUI Tabs
```

Tab indicator: `#06b6d4`, 2px
Active tab: `text.primary`, weight 500
Inactive tab: `text.secondary`

---

## Tab 1 — Infrastructure (Hosts View)

Exact reference: Elastic image 1 from your screenshots.

### Section 1 — Colored Summary Cards

5 cards in a row, equal width.

| Card | Label | Value | Color |
|---|---|---|---|
| 1 | Hosts | 11 | `#06b6d4` |
| 2 | CPU Usage (avg) | 12.28% | `#f59e0b` |
| 3 | Memory Usage (avg) | 36.03% | `#8b5cf6` |
| 4 | Network Inbound (RX) | 8.19 Mbit | `#06b6d4` teal |
| 5 | Network Outbound (TX) | 15.41 Mbit | `#f97316` |

Each card:
```
background: linear-gradient(135deg, {color}18 0%, {color}08 100%)
border: 1px solid {color}30
border-radius: 4px
padding: 16px
height: 100px
```

Bottom of card: large metric value + unit
Top right: mini sparkline (ECharts, no axes, 40px height)
Top left: label in `caption` style

### Section 2 — Filters Row

```
[Operating System: Any ▼]  [Cloud Provider: Any ▼]  [Host Limit: 10 | 20 | 50 | 100* | 500]
```

- Selects: MUI `<Select>` small
- Host limit: MUI `<ToggleButtonGroup>` — 100 selected by default, accent color

### Section 3 — Hosts Table

Columns:
| Column | Width | Notes |
|---|---|---|
| — | 32px | expand/link icon |
| Name | 200px | hostname, mono, cyan link |
| Operating System | 140px | OS name |
| CPU usage (avg) | 130px | percentage + mini bar |
| Disk Latency (avg) | 130px | ms value |
| RX (avg) | 110px | Mbit/s |
| TX (avg) | 110px | Mbit/s |
| Memory total (avg) | 140px | GB |
| Memory usage (avg) | 140px | percentage + mini bar |

Row height: 36px
Click row → opens HostDetailDrawer
Click name link → navigates to `/metrics/hosts/:id` (HostDetailPage)

### Mock hosts data
```typescript
export const mockHosts = [
  { id: 'h1', name: '041ecb195a9f', os: null,   cpu: 0,    diskLatency: 0,    rx: 0,    tx: 0,    memTotal: 0,   memUsage: 0    },
  { id: 'h2', name: '69e2aeee9842', os: null,   cpu: 0,    diskLatency: 0,    rx: 0,    tx: 0,    memTotal: 0,   memUsage: 0    },
  { id: 'h3', name: '94ebaa02dec7', os: null,   cpu: 0,    diskLatency: 0,    rx: 0,    tx: 0,    memTotal: 0,   memUsage: 0    },
  { id: 'h4', name: 'gke-demo-co-1', os: 'Ubuntu', cpu: 23.7, diskLatency: 4.9,  rx: 11.1, tx: 14.5, memTotal: 16.8, memUsage: 49.9 },
  { id: 'h5', name: 'gke-demo-co-2', os: 'Ubuntu', cpu: 13.1, diskLatency: 1.4,  rx: 16.0, tx: 20.4, memTotal: 16.8, memUsage: 63.6 },
  { id: 'h6', name: 'gke-demo-co-3', os: 'Ubuntu', cpu: 20.6, diskLatency: 3.7,  rx: 13.5, tx: 41.9, memTotal: 16.8, memUsage: 49.0 },
  { id: 'h7', name: 'ip-192-168-1',  os: 'Ubuntu', cpu: 3.6,  diskLatency: 5.2,  rx: 121.3,tx: 172.2,memTotal: 16.8, memUsage: 8.0  },
  { id: 'h8', name: 'ip-192-168-2',  os: 'Ubuntu', cpu: 67.2, diskLatency: 12.1, rx: 45.2, tx: 38.7, memTotal: 32.0, memUsage: 78.4 },
  { id: 'h9', name: 'ip-192-168-3',  os: 'CentOS', cpu: 45.8, diskLatency: 8.3,  rx: 22.4, tx: 19.8, memTotal: 8.0,  memUsage: 54.2 },
  { id: 'h10',name: 'ip-192-168-4',  os: 'Debian', cpu: 8.2,  diskLatency: 2.1,  rx: 5.4,  tx: 4.2,  memTotal: 16.0, memUsage: 31.7 },
  { id: 'h11',name: 'ip-192-168-5',  os: 'Ubuntu', cpu: 91.4, diskLatency: 22.7, rx: 87.3, tx: 92.1, memTotal: 64.0, memUsage: 88.9 },
];
```

CPU and Memory usage columns:
- Value as percentage text
- Thin progress bar below: `height: 3px`, color based on value:
  - < 50%: `#10b981`
  - 50-80%: `#f59e0b`
  - > 80%: `#ef4444`

---

## Host Detail Drawer

Width: `520px`, MUI `<Drawer anchor="right">`

### Header
```
gke-demo-co-1                          [Open full page →]  [×]
Ubuntu  •  host-prod-02  •  Last seen: just now
```

### Tabs: Overview | Metrics | Logs | Processes

#### Overview tab
4 stat tiles in 2x2 grid:
- CPU: `23.7%` with trend chart
- Memory: `63.6%` with trend chart
- Network RX: `16.0 Mbit/s`
- Network TX: `20.4 Mbit/s`

Below: system info table
```
FIELD            VALUE
Hostname         gke-demo-co-1
OS               Ubuntu 22.04
Kernel           5.15.0-1034-gke
IP Address       192.168.1.42
Uptime           12d 4h 22m
CPU Cores        8
Total Memory     16.8 GB
Cloud Provider   GCP
Region           us-central1-a
```

#### Metrics tab
4 ECharts line charts stacked vertically, each 140px tall:
- CPU Usage % over time
- Memory Usage % over time
- Network RX/TX Mbit/s (dual series)
- Disk Latency ms over time

Each chart: no card border, just label above + chart below

#### Logs tab
Last 20 log lines for this host — reuse LogsTable component with host filter pre-applied

#### Processes tab
Table of top processes:
```typescript
[
  { pid: 1842, name: 'node',    cpu: 18.2, mem: 4.2,  status: 'running' },
  { pid: 2341, name: 'python3', cpu: 12.4, mem: 2.8,  status: 'running' },
  { pid: 891,  name: 'nginx',   cpu: 2.1,  mem: 0.4,  status: 'running' },
  { pid: 3421, name: 'redis',   cpu: 1.8,  mem: 1.2,  status: 'running' },
  { pid: 4892, name: 'postgres',cpu: 8.7,  mem: 6.4,  status: 'running' },
]
```

---

## Host Detail Page

Route: `/metrics/hosts/:id`
Full page view of a single host — same content as drawer but full width.

### Layout
```
← Back to Infrastructure

gke-demo-co-1    [Ubuntu]  [● Healthy]        Last 1h ▼  Refresh

┌─────────┬─────────┬─────────┬─────────┐
│  CPU    │ Memory  │  RX     │  TX     │  ← stat tiles row
└─────────┴─────────┴─────────┴─────────┘

┌──────────────────────┬────────────────┐
│  CPU Usage chart     │  System Info   │
├──────────────────────┤                │
│  Memory Usage chart  │                │
├──────────────────────┤                │
│  Network chart       │                │
├──────────────────────┤                │
│  Disk Latency chart  │                │
└──────────────────────┴────────────────┘

[Logs tab preview]
[Processes tab preview]
```

---

## Tab 2 — Metrics Explorer

Like Datadog Metrics Explorer — build a custom metric query visually.

### Layout
```
┌─────────────────────────────────────────────────────┐
│  QUERY BAR                                          │
│  Metric: [system.cpu.usage ▼]                       │
│  From:   [host ▼]  Filter: [+ Add filter]          │
│  Group by: [host ▼]  Aggregation: [avg ▼]          │
├─────────────────────────────────────────────────────┤
│                                                     │
│  MAIN CHART (ECharts, multi-series line)            │
│  height: 320px                                      │
│                                                     │
├──────────┬──────────┬──────────┬────────────────────┤
│ CPU      │ Memory   │ Network  │  Disk              │  ← preset metric tiles
│ chart    │ chart    │ chart    │  chart             │
│ 160px    │ 160px    │ 160px    │  160px             │
└──────────┴──────────┴──────────┴────────────────────┘
```

### Query bar controls
```typescript
export const metricOptions = [
  'system.cpu.usage',
  'system.memory.usage',
  'system.network.in.bytes',
  'system.network.out.bytes',
  'system.disk.io.read',
  'system.disk.io.write',
  'system.load.1',
  'system.load.5',
  'system.load.15',
  'service.request.rate',
  'service.error.rate',
  'service.latency.p50',
  'service.latency.p95',
  'service.latency.p99',
];
```

Aggregation options: avg, sum, min, max, count, p50, p95, p99
Group by options: host, service, region, os, cloud_provider

### Main chart
- Multi-series: one line per host/service when grouped
- Uses chart color palette in order
- Time range: matches page time range selector
- Legend below chart, scrollable if many series
- Tooltip: shared crosshair, dark style

### Mock metrics timeseries
```typescript
export const generateTimeSeries = (base: number, variance: number, points = 60) =>
  Array.from({length: points}, () =>
    +(base + (Math.random() - 0.5) * variance * 2).toFixed(2)
  );

export const metricsExplorerData = {
  timestamps: Array.from({length:60}, (_,i) => {
    const d = new Date(); d.setMinutes(d.getMinutes() - (59-i));
    return d.toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'});
  }),
  series: {
    'host-prod-01': generateTimeSeries(23.7, 8),
    'host-prod-02': generateTimeSeries(67.2, 12),
    'host-prod-03': generateTimeSeries(45.8, 10),
    'host-staging':  generateTimeSeries(8.2,  5),
  }
};
```

### Preset metric tiles (bottom row)
4 small charts, click to load that metric into main chart:
- CPU Usage — `system.cpu.usage`
- Memory Usage — `system.memory.usage`
- Network In — `system.network.in.bytes`
- Disk IO — `system.disk.io.read`

Each tile:
```
CPU Usage          avg 34.2%
[small line chart, 80px height]
```
Border: `1px solid #1f2535`
Active (selected): border `#06b6d440`, background `#06b6d408`

---

## Service Metrics Section

Below the host metrics, a second section for service-level metrics.

### Service metrics summary table
```
SERVICE           REQ/S    ERROR%   P50     P95     P99
api-gateway       4,821    0.4%     45ms    124ms   312ms
auth-service      2,043    0.1%     23ms    67ms    98ms
user-service      1,932    1.8%     312ms   891ms   1.2s
payment-service   721      0.2%     67ms    198ms   445ms
search-service    443      8.4%     5012ms  —       —
cache-service     8,211    0.0%     4ms     12ms    18ms
queue-service     987      0.3%     445ms   892ms   1.4s
```

Color rules:
- Error% > 5%: red
- Error% 1-5%: amber
- Error% < 1%: default
- P99 > 1s: red
- P99 500ms-1s: amber

---

## Prompt for Deepseek

```
Read all files in docs/design-system/.
Now read docs/design-system/phase4-metrics.md.

Build the complete Metrics & Infrastructure page at src/pages/metrics/index.tsx
with all sub-components as specified.

Rules:
1. mockData.ts — all mock data as specified including generateTimeSeries helper
2. InfrastructureTab.tsx — colored cards + filters row + hosts table
3. HostsTable.tsx — with inline CPU/Memory progress bars, colored by threshold
4. HostDetailDrawer.tsx — 520px right drawer, 4 tabs
5. HostDetailPage.tsx — full page at /metrics/hosts/:id, add route to App.tsx
6. MetricsExplorerTab.tsx — query bar + main chart + 4 preset tiles
7. MetricsQueryBar.tsx — metric select + from + filter + group by + aggregation
8. Service metrics table at bottom of explorer tab
9. All ECharts charts: transparent background, theme colors only
10. CPU/Memory bars: green < 50%, amber 50-80%, red > 80%
11. Add /metrics/hosts/:id route to App.tsx router

Do not add features not in the spec.
```
