# Phase 7 — Complete Polish & Remaining Pages
## obsAdmin

---

## PROMPT 7A — Dark/Light Mode + Toasts

### Theme Mode System

Add to `src/store/uiStore.ts`:
```typescript
themeMode: 'dark' | 'light';
toggleThemeMode: () => void;
```
Persist in localStorage key `obsadmin-ui`.

### Light Theme Palette

Add to `src/theme/index.ts` — export two themes:

```typescript
export const darkTheme = createTheme({ /* existing dark theme */ });

export const lightTheme = createTheme({
  palette: {
    mode: 'light',
    background: {
      default: '#f4f5f7',
      paper:   '#ffffff',
      surface:  '#ffffff',
      elevated: '#f9fafb',
      hover:    '#f3f4f6',
      selected: '#eff6ff',
    },
    primary: {
      main:  '#0891b2',
      dark:  '#0e7490',
      light: '#06b6d4',
      contrastText: '#ffffff',
    },
    text: {
      primary:   '#111827',
      secondary: '#6b7280',
      disabled:  '#9ca3af',
    },
    divider: '#e5e7eb',
    // keep same success/warning/error/accent/chart colors
    success: { main: '#10b981' },
    warning: { main: '#f59e0b' },
    error:   { main: '#ef4444' },
    accent: {
      default: '#0891b2',
      hover:   '#0e7490',
      subtle:  'rgba(8,145,178,0.08)',
      border:  'rgba(8,145,178,0.25)',
    },
    chart: {
      colors: ['#0891b2','#8b5cf6','#f59e0b','#10b981','#f97316','#ec4899'],
      grid:   '#e5e7eb',
      axis:   '#9ca3af',
    },
  },
  // same typography, shape, spacing, shadows as dark theme
  // same component overrides but with light-appropriate colors
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        body: { background: '#f4f5f7' },
        '::-webkit-scrollbar-thumb': { background: '#d1d5db' },
      },
    },
    MuiCard: {
      defaultProps: { elevation: 0 },
      styleOverrides: {
        root: {
          background: '#ffffff',
          border: '1px solid #e5e7eb',
          borderRadius: '4px',
        },
      },
    },
    MuiDrawer: {
      styleOverrides: {
        paper: {
          background: '#ffffff',
          borderRight: '1px solid #e5e7eb',
        },
      },
    },
    MuiAppBar: {
      defaultProps: { elevation: 0 },
      styleOverrides: {
        root: {
          background: '#ffffff',
          borderBottom: '1px solid #e5e7eb',
          color: '#111827',
        },
      },
    },
    MuiTableHead: {
      styleOverrides: {
        root: {
          '& .MuiTableCell-root': {
            background: '#f9fafb',
            borderBottom: '1px solid #e5e7eb',
            color: '#6b7280',
          },
        },
      },
    },
    MuiOutlinedInput: {
      styleOverrides: {
        root: {
          background: '#ffffff',
          '& .MuiOutlinedInput-notchedOutline': { borderColor: '#d1d5db' },
          '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: '#9ca3af' },
          '&.Mui-focused .MuiOutlinedInput-notchedOutline': {
            borderColor: '#0891b2',
            boxShadow: '0 0 0 2px rgba(8,145,178,0.12)',
          },
        },
      },
    },
  },
});
```

### Apply in `main.tsx`
```tsx
import { darkTheme, lightTheme } from './theme';
import { useUIStore } from './store/uiStore';

function ThemedApp() {
  const { themeMode } = useUIStore();
  const theme = themeMode === 'dark' ? darkTheme : lightTheme;
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <App />
    </ThemeProvider>
  );
}
```

### Theme Toggle Button Component
`src/components/common/ThemeToggle.tsx`

```tsx
// Sun icon for light mode, Moon icon for dark mode
// Use Lucide: Sun, Moon
// IconButton, small, tooltip "Switch to light/dark mode"
// On click: toggleThemeMode() from useUIStore
```

Place in THREE locations:
1. **Topbar** — between Refresh button and notification bell
2. **User avatar menu** — as a menu item with toggle switch
3. **Settings page** → Appearance tab

---

### Sonner Toasts — Global Setup

Install: `sonner` (already in dependencies)

In `src/layout/AppShell.tsx` add:
```tsx
import { Toaster } from 'sonner';
// inside return, after children:
<Toaster
  position="bottom-right"
  theme={themeMode}
  toastOptions={{
    style: {
      background: themeMode === 'dark' ? '#1c2333' : '#ffffff',
      border: `1px solid ${themeMode === 'dark' ? '#2a3147' : '#e5e7eb'}`,
      color: themeMode === 'dark' ? '#e8eaf0' : '#111827',
      fontSize: '13px',
      borderRadius: '4px',
    },
  }}
/>
```

Create `src/lib/toast.ts`:
```typescript
import { toast } from 'sonner';

export const notify = {
  success: (msg: string) => toast.success(msg),
  error:   (msg: string) => toast.error(msg),
  warning: (msg: string) => toast.warning(msg),
  info:    (msg: string) => toast.info(msg),
  loading: (msg: string) => toast.loading(msg),
};
```

Wire toasts to these actions across the app:
- Alert silenced → `notify.success('Alert silenced for 1 hour')`
- Alert rule created → `notify.success('Alert rule created')`
- Alert rule deleted → `notify.error('Alert rule deleted')`
- Incident note added → `notify.success('Note added')`
- Settings saved → `notify.success('Settings saved')`
- Copy to clipboard → `notify.success('Copied to clipboard')`
- Live tail enabled → `notify.info('Live tail enabled')`
- Data source connected → `notify.success('Data source connected')`

---

## PROMPT 7B — Settings Page + User Avatar Menu

### Settings Page `/settings`

Tabs:
```
[General]  [Appearance]  [Data Sources]  [Notifications]  [Team]  [API Keys]
```

#### General Tab
```
ORGANIZATION
Organization name:  [obsAdmin Demo_______]
Timezone:          [UTC ▼]
Date format:       [YYYY-MM-DD ▼]

DEFAULT VIEWS
Default time range:    [Last 15 minutes ▼]
Default environment:   [production ▼]
Rows per page:         [25 ▼]

TELEMETRY
[ ] Send anonymous usage data to help improve obsAdmin

[Save Changes]
```

#### Appearance Tab
```
THEME
● Dark    ○ Light

DENSITY
○ Compact    ● Default    ○ Comfortable

SIDEBAR
[✓] Show section labels
[✓] Show icons
[ ] Collapse on mobile

[Save Changes]
```
Theme radio connects to `useUIStore().themeMode`
Density setting: store in uiStore, apply `theme.spacing` multiplier

#### Data Sources Tab
Cards for each data source type:

```
┌──────────────────────────────┐
│ [icon] Prometheus            │
│ ● Connected                  │
│ http://prometheus:9090        │
│                              │
│ [Test Connection]  [Edit]    │
└──────────────────────────────┘
```

```typescript
const dataSources = [
  { type: 'Prometheus',   status: 'connected', url: 'http://prometheus:9090',  icon: 'Activity' },
  { type: 'Loki',         status: 'connected', url: 'http://loki:3100',        icon: 'FileText' },
  { type: 'Tempo',        status: 'connected', url: 'http://tempo:3200',       icon: 'GitBranch' },
  { type: 'Elasticsearch',status: 'error',     url: 'http://elastic:9200',     icon: 'Search' },
  { type: 'ClickHouse',   status: 'disconnected', url: '',                     icon: 'Database' },
  { type: 'Kafka',        status: 'disconnected', url: '',                     icon: 'Radio' },
];
```

[+ Add Data Source] button → modal with type selector + URL + auth fields

#### Notifications Tab
```
GLOBAL NOTIFICATION SETTINGS

Default severity threshold:  [Warning ▼]  (only alert on this severity+)

CHANNELS
[Links to Notification Channels in /alerts — same component reused]
```

#### Team Tab
Condensed version of Users page — list of team members with roles.
[Invite Member] button → email input modal

#### API Keys Tab
```
API KEYS

[+ Generate New Key]

NAME              CREATED      LAST USED    SCOPES        ACTIONS
Admin Key         2d ago       1h ago       read, write   [Copy] [Revoke]
Read-only Key     5d ago       3h ago       read          [Copy] [Revoke]
CI Pipeline Key   1d ago       just now     write         [Copy] [Revoke]
```

Keys masked: `sk_live_**********************abcd`
Copy button → copies to clipboard + toast

---

### User Avatar Menu

Click `AD` avatar circle in topbar → MUI `<Menu>` popover:

```
┌──────────────────────────┐
│ [Avatar]                 │
│ Admin User               │
│ admin@obsadmin.io        │
├──────────────────────────┤
│ ☀ / 🌙  Dark mode  [toggle]│
├──────────────────────────┤
│ 👤  Profile              │
│ ⚙️  Settings             │
│ 🔑  API Keys             │
├──────────────────────────┤
│ 📖  Documentation        │
│ 💬  Community            │
├──────────────────────────┤
│ 🚪  Sign out             │
└──────────────────────────┘
```

Theme toggle inside menu: inline Switch component
Settings → navigates to `/settings`
API Keys → navigates to `/settings` with API Keys tab active

---

## PROMPT 7C — Synthetics Monitors + Demo Data Modal

### Synthetics Monitors `/synthetics`

Tabs:
```
[Monitors]  [Status Page]
```

#### Monitors Tab

Summary bar:
```
● Up: 8   ● Down: 1   ● Degraded: 2   Avg availability: 98.7%
```

Monitors Table:
```typescript
export const mockMonitors = [
  { id: 'm1', name: 'API Gateway Health',    url: 'https://api.obsadmin.io/health', type: 'HTTP',    status: 'up',       availability: 99.9, avgDuration: 45,   lastCheck: '30s ago',  frequency: '1m'  },
  { id: 'm2', name: 'Auth Service Check',    url: 'https://auth.obsadmin.io/ping',  type: 'HTTP',    status: 'up',       availability: 100,  avgDuration: 23,   lastCheck: '45s ago',  frequency: '1m'  },
  { id: 'm3', name: 'Search Endpoint',       url: 'https://api.obsadmin.io/search', type: 'HTTP',    status: 'down',     availability: 97.2, avgDuration: null, lastCheck: '1m ago',   frequency: '1m'  },
  { id: 'm4', name: 'Payment API',           url: 'https://pay.obsadmin.io',        type: 'HTTP',    status: 'up',       availability: 99.8, avgDuration: 67,   lastCheck: '20s ago',  frequency: '2m'  },
  { id: 'm5', name: 'Database TCP Check',    url: 'postgres:5432',                  type: 'TCP',     status: 'up',       availability: 99.9, avgDuration: 4,    lastCheck: '1m ago',   frequency: '1m'  },
  { id: 'm6', name: 'Redis TCP Check',       url: 'redis:6379',                     type: 'TCP',     status: 'degraded', availability: 98.1, avgDuration: 892,  lastCheck: '30s ago',  frequency: '1m'  },
  { id: 'm7', name: 'SSL Certificate',       url: 'https://obsadmin.io',            type: 'SSL',     status: 'up',       availability: 100,  avgDuration: null, lastCheck: '1h ago',   frequency: '1h'  },
  { id: 'm8', name: 'DNS Resolution',        url: 'obsadmin.io',                    type: 'DNS',     status: 'up',       availability: 100,  avgDuration: 12,   lastCheck: '5m ago',   frequency: '5m'  },
  { id: 'm9', name: 'Full Journey Check',    url: 'https://obsadmin.io/login',      type: 'Journey', status: 'degraded', availability: 96.4, avgDuration: 3421, lastCheck: '10m ago',  frequency: '10m' },
  { id: 'm10',name: 'CDN Edge Check',        url: 'https://cdn.obsadmin.io',        type: 'HTTP',    status: 'up',       availability: 99.7, avgDuration: 18,   lastCheck: '1m ago',   frequency: '1m'  },
];
```

Columns: Status dot, Name, URL (truncated mono), Type chip, Availability %, Avg Duration, Last Check, Frequency, Actions

Click row → Monitor Detail Drawer (380px):
- Header: name + status + URL
- Tabs: Overview | History | Errors
- Overview: availability % big number + mini duration trend chart
- Duration trends ECharts: p50/p75/p95/max lines over 24h
- Status bar at bottom: 24h of 5-min buckets, green=up, red=down, amber=degraded

[+ Create Monitor] button → modal:
```
Name: [_____________]
Type: [HTTP ▼]  (HTTP, TCP, SSL, DNS, Journey)
URL:  [_____________]
Frequency: [1 minute ▼]
Locations: [✓] US East  [✓] EU West  [ ] Asia Pacific
[Notify on failure]: [✓] Slack #alerts
```

#### Status Page Tab
Public-facing style status display:
```
obsAdmin Status                    All Systems Operational ✓

SERVICES
● API Gateway          Operational
● Authentication       Operational
● Search               Major Outage
● Payments             Operational
● Notifications        Operational

RECENT INCIDENTS
[links to incidents from /incidents]

90-DAY UPTIME
[colored grid of daily dots — green/amber/red]
```

---

### Demo Data Modal

Route: `/demo` → full page

```
Demo Data & Sandbox

Load mock data to explore obsAdmin features.
Data is generated locally and does not affect any real systems.

┌─────────────────────────────────────────────────────┐
│ [Activity] Infrastructure Metrics        [● Loaded] │
│ 11 hosts, 847 containers, 60min of metrics data     │
│                                     [Reload] [Clear]│
├─────────────────────────────────────────────────────┤
│ [FileText] Log Stream                [● Loaded]     │
│ 200 log entries across 10 services                  │
│                                     [Reload] [Clear]│
├─────────────────────────────────────────────────────┤
│ [GitBranch] Traces & Spans           [● Loaded]     │
│ 12 traces, 9 spans per trace, service map           │
│                                     [Reload] [Clear]│
├─────────────────────────────────────────────────────┤
│ [Bell] Alerts & Incidents            [● Loaded]     │
│ 8 firing alerts, 4 incidents, 10 alert rules        │
│                                     [Reload] [Clear]│
├─────────────────────────────────────────────────────┤
│ [Radio] Synthetics Monitors          [● Loaded]     │
│ 10 monitors across HTTP/TCP/SSL/DNS types           │
│                                     [Reload] [Clear]│
└─────────────────────────────────────────────────────┘

[Load All Demo Data]          [Clear All Data]
```

Each row: icon, dataset name, description, status badge, Reload + Clear buttons.
"Load All" button fires all loaders + `notify.success('Demo data loaded successfully')`

---

## PROMPT 7D — Users & Integrations

### Users Page `/users`

Tabs:
```
[Members]  [Roles]  [Audit Log]
```

#### Members Tab

Summary: `12 members  •  3 pending invites`

[+ Invite Member] button → modal:
```
Invite Team Member
Email:  [_______________]
Role:   [Viewer ▼]
[Send Invite]
```

Members Table:
```typescript
export const mockUsers = [
  { id: 'u1', name: 'Admin User',    email: 'admin@obsadmin.io',   role: 'Admin',   status: 'active',  lastSeen: 'just now',  avatar: 'AU' },
  { id: 'u2', name: 'John Doe',      email: 'john@obsadmin.io',    role: 'Editor',  status: 'active',  lastSeen: '2h ago',    avatar: 'JD' },
  { id: 'u3', name: 'Jane Smith',    email: 'jane@obsadmin.io',    role: 'Editor',  status: 'active',  lastSeen: '1d ago',    avatar: 'JS' },
  { id: 'u4', name: 'Bob Chen',      email: 'bob@obsadmin.io',     role: 'Viewer',  status: 'active',  lastSeen: '3d ago',    avatar: 'BC' },
  { id: 'u5', name: 'Alice Kumar',   email: 'alice@obsadmin.io',   role: 'Viewer',  status: 'active',  lastSeen: '1w ago',    avatar: 'AK' },
  { id: 'u6', name: 'Pending User',  email: 'pending@company.com', role: 'Viewer',  status: 'pending', lastSeen: '—',         avatar: 'PU' },
];
```

Columns: Avatar + Name, Email, Role chip, Status, Last Seen, Actions (edit role, remove)

Role chips:
- Admin: cyan
- Editor: purple
- Viewer: default gray

#### Roles Tab
3 role cards:
```
┌─────────────────────────────┐
│ Admin                       │
│ Full access to all features │
│                             │
│ ✓ View all data             │
│ ✓ Create/edit alert rules   │
│ ✓ Manage users              │
│ ✓ Configure data sources    │
│ ✓ Delete data               │
│                  2 members  │
└─────────────────────────────┘
```

Same for Editor (no manage users, no delete) and Viewer (view only).

#### Audit Log Tab
Table of recent actions:
```typescript
[
  { time: '14:23', user: 'Admin User', action: 'Created alert rule', target: 'High Error Rate', ip: '192.168.1.1' },
  { time: '14:15', user: 'John Doe',   action: 'Silenced alert',     target: 'CPU Spike',       ip: '192.168.1.4' },
  { time: '13:58', user: 'Admin User', action: 'Invited user',       target: 'pending@...',     ip: '192.168.1.1' },
  { time: '13:45', user: 'Jane Smith', action: 'Updated dashboard',  target: 'Main Dashboard',  ip: '192.168.1.7' },
]
```

---

### Integrations Page `/integrations`

Category tabs:
```
[All]  [Data Sources]  [Alerting]  [CI/CD]  [Cloud]
```

Integration cards grid (3 per row):

```typescript
export const mockIntegrations = [
  // Data Sources
  { name: 'Prometheus',    category: 'Data Sources', status: 'connected',    logo: 'Activity',   desc: 'Metrics collection and alerting' },
  { name: 'Loki',          category: 'Data Sources', status: 'connected',    logo: 'FileText',   desc: 'Log aggregation system' },
  { name: 'Tempo',         category: 'Data Sources', status: 'connected',    logo: 'GitBranch',  desc: 'Distributed tracing backend' },
  { name: 'Elasticsearch', category: 'Data Sources', status: 'error',        logo: 'Search',     desc: 'Search and analytics engine' },
  { name: 'ClickHouse',    category: 'Data Sources', status: 'available',    logo: 'Database',   desc: 'Column-oriented database' },
  { name: 'InfluxDB',      category: 'Data Sources', status: 'available',    logo: 'TrendingUp', desc: 'Time series database' },
  // Alerting
  { name: 'Slack',         category: 'Alerting',     status: 'connected',    logo: 'Hash',       desc: 'Team messaging and notifications' },
  { name: 'PagerDuty',     category: 'Alerting',     status: 'connected',    logo: 'Phone',      desc: 'Incident response platform' },
  { name: 'OpsGenie',      category: 'Alerting',     status: 'available',    logo: 'Bell',       desc: 'Alert management platform' },
  { name: 'Email',         category: 'Alerting',     status: 'connected',    logo: 'Mail',       desc: 'Email notifications' },
  // CI/CD
  { name: 'GitHub Actions',category: 'CI/CD',        status: 'available',    logo: 'Github',     desc: 'CI/CD and deployment tracking' },
  { name: 'GitLab CI',     category: 'CI/CD',        status: 'available',    logo: 'GitMerge',   desc: 'GitLab pipeline integration' },
  { name: 'Jenkins',       category: 'CI/CD',        status: 'available',    logo: 'Layers',     desc: 'Open source automation server' },
  // Cloud
  { name: 'AWS',           category: 'Cloud',        status: 'available',    logo: 'Cloud',      desc: 'Amazon Web Services metrics' },
  { name: 'GCP',           category: 'Cloud',        status: 'available',    logo: 'Cloud',      desc: 'Google Cloud Platform metrics' },
  { name: 'Azure',         category: 'Cloud',        status: 'available',    logo: 'Cloud',      desc: 'Microsoft Azure metrics' },
];
```

Card status:
- connected: green border-left + "● Connected" badge
- error: red border-left + "● Error" badge
- available: default + "Configure" button

Connected card:
```
┌─────────────────────────────┐
│ [icon]  Prometheus  ● Connected│
│ Metrics collection...       │
│                             │
│ [Configure]  [Disconnect]   │
└─────────────────────────────┘
```

---

## PROMPT 7E — Polish Pass

### 1. Breadcrumbs — fix dynamic routing

Update `Topbar.tsx` to generate breadcrumbs from `useLocation()`:

```typescript
const breadcrumbMap: Record<string, string> = {
  '/':              'Dashboard',
  '/logs':          'Logs',
  '/traces':        'Traces',
  '/metrics':       'Metrics',
  '/apm':           'APM',
  '/synthetics':    'Synthetics',
  '/alerts':        'Alerts',
  '/incidents':     'Incidents',
  '/users':         'Users',
  '/integrations':  'Integrations',
  '/settings':      'Settings',
  '/demo':          'Demo Data',
};
// Always prefix: Observability > Infrastructure > [page]
```

### 2. Notification Bell Dropdown

Click bell icon → MUI `<Popover>`, width `360px`:

```
Notifications                    [Mark all read]
──────────────────────────────────────────────
🔴 search-service is down          2m ago  ●
   Error rate exceeded 5% threshold
──────────────────────────────────────────────
🟡 High latency on user-service    8m ago  ●
   P95 latency: 891ms
──────────────────────────────────────────────
✅ Deployment completed            22m ago
   auth-service v2.4.1 deployed
──────────────────────────────────────────────
[View all alerts →]
```

Unread: bold title + blue dot + slightly lighter background
Bell icon: shows badge count of unread notifications

### 3. Global Search

Click search bar in topbar (or ⌘K) → MUI `<Modal>` fullscreen overlay:

```
┌──────────────────────────────────────────────┐
│ 🔍 Search services, logs, traces...    [Esc] │
├──────────────────────────────────────────────┤
│ RECENT                                       │
│ 📊  Dashboard                                │
│ 📋  Logs — search-service errors            │
│ 🔀  Trace a3f9c2b1                          │
├──────────────────────────────────────────────┤
│ SERVICES                                     │
│ ● api-gateway      healthy                   │
│ ● search-service   down                      │
├──────────────────────────────────────────────┤
│ QUICK ACTIONS                                │
│ ⚡ Create alert rule                         │
│ ⚡ Invite team member                        │
│ ⚡ Load demo data                            │
└──────────────────────────────────────────────┘
```

Keyboard: ↑↓ to navigate, Enter to select, Esc to close
Wire ⌘K / Ctrl+K keyboard shortcut globally in `AppShell.tsx`

### 4. Loading Skeletons

Create `src/components/common/PageSkeleton.tsx`:
```tsx
// Shows MUI Skeleton placeholders matching page layout
// Used while data loads (show for 1.5s on first mount via setTimeout)
// Variants: 'table' | 'cards' | 'chart' | 'dashboard'
```

Apply to: Dashboard, Logs, Metrics, Traces pages on initial load.

### 5. Empty States

Create `src/components/common/EmptyState.tsx`:
```tsx
interface EmptyStateProps {
  icon: LucideIcon;
  title: string;
  description: string;
  action?: { label: string; onClick: () => void };
}
```

Apply to:
- Logs table (no results): "No logs found — try adjusting your filters"
- Traces table (no results): "No traces found for this time range"
- Alerts (no firing): "All clear — no firing alerts"
- Monitors (none created): "No monitors yet — create your first monitor"

### 6. Docs Page `/docs`

Simple page linking to external resources:
```
Documentation

GETTING STARTED
→ Quick Start Guide
→ Installation
→ Configuration

FEATURES
→ Logs Explorer
→ Metrics & Infrastructure
→ Traces & APM
→ Alerts & Incidents
→ Synthetics

API REFERENCE
→ REST API
→ Query Language

COMMUNITY
→ GitHub (link: github.com/obsadmin)
→ Discord
→ Contributing Guide
```

Cards layout, 3 per row, each card is a link with external icon.

---

## Final Prompt for Deepseek (run after 7A-7D)

```
Read all files in docs/design-system/.
Now read docs/design-system/phase7-polish.md.

Run prompts in order: 7A → 7B → 7C → 7D → 7E.
Do not run the next prompt until the previous one is verified.

CRITICAL for 7A:
- Export both darkTheme and lightTheme from src/theme/index.ts
- ThemeToggle component must appear in topbar, user menu, AND settings
- Sonner Toaster must be in AppShell, not in individual pages
- notify.ts helper must be used everywhere — never import toast directly

CRITICAL for 7E breadcrumbs:
- Use useLocation() from react-router-dom
- Dynamic segments (/traces/:id) should show trace ID as last crumb
- Never hardcode "Dashboard" as the breadcrumb

Do not skip polish items. Do not invent features not in spec.
```
