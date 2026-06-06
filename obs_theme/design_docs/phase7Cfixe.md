# Phase 7C Fixes — Synthetics Monitors
## obsAdmin

---

## Fix 1 — Monitors Toolbar

Add above the table in `src/pages/synthetics/index.tsx`:

```tsx
// Toolbar row
<Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center' }}>
  <TextField
    size="small"
    placeholder="Search monitors..."
    InputProps={{ startAdornment: <Search size={14} /> }}
    sx={{ width: 280 }}
    value={search}
    onChange={(e) => setSearch(e.target.value)}
  />
  <Select size="small" value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)} sx={{ width: 120 }}>
    <MenuItem value="all">All Types</MenuItem>
    <MenuItem value="HTTP">HTTP</MenuItem>
    <MenuItem value="TCP">TCP</MenuItem>
    <MenuItem value="SSL">SSL</MenuItem>
    <MenuItem value="DNS">DNS</MenuItem>
    <MenuItem value="Journey">Journey</MenuItem>
  </Select>
  <Select size="small" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} sx={{ width: 120 }}>
    <MenuItem value="all">All Status</MenuItem>
    <MenuItem value="up">Up</MenuItem>
    <MenuItem value="down">Down</MenuItem>
    <MenuItem value="degraded">Degraded</MenuItem>
  </Select>
  <Box sx={{ flex: 1 }} />
  <Button
    variant="contained"
    size="small"
    startIcon={<Plus size={14} />}
    onClick={() => setCreateModalOpen(true)}
  >
    Create Monitor
  </Button>
</Box>
```

Filter logic:
```typescript
const filteredMonitors = mockMonitors.filter(m => {
  const matchSearch = m.name.toLowerCase().includes(search.toLowerCase()) ||
                      m.url.toLowerCase().includes(search.toLowerCase());
  const matchType   = typeFilter === 'all' || m.type === typeFilter;
  const matchStatus = statusFilter === 'all' || m.status === statusFilter;
  return matchSearch && matchType && matchStatus;
});
```

---

## Fix 2 — Actions Column + Row Click

### Add Actions column to monitors table

```tsx
// Last column in table
<TableCell align="right" sx={{ width: 120 }}>
  <Box sx={{ display: 'flex', gap: 0.5, justifyContent: 'flex-end', opacity: 0, '.MuiTableRow-root:hover &': { opacity: 1 } }}>
    <Tooltip title="Pause monitor">
      <IconButton size="small" onClick={(e) => { e.stopPropagation(); handlePause(monitor.id); }}>
        <Pause size={14} />
      </IconButton>
    </Tooltip>
    <Tooltip title="Edit monitor">
      <IconButton size="small" onClick={(e) => { e.stopPropagation(); handleEdit(monitor); }}>
        <Pencil size={14} />
      </IconButton>
    </Tooltip>
    <Tooltip title="Delete monitor">
      <IconButton size="small" onClick={(e) => { e.stopPropagation(); handleDelete(monitor.id); }}>
        <Trash2 size={14} color="#ef4444" />
      </IconButton>
    </Tooltip>
  </Box>
</TableCell>
```

Actions:
- Pause → `notify.success('Monitor paused')`
- Edit → opens CreateMonitorModal pre-filled with monitor data
- Delete → confirmation dialog → `notify.success('Monitor deleted')`

### Row click opens detail drawer

```tsx
<TableRow
  key={monitor.id}
  hover
  onClick={() => { setSelectedMonitor(monitor); setDrawerOpen(true); }}
  sx={{ cursor: 'pointer' }}
>
```

---

## Fix 3 — URL Column Width

```tsx
// URL column — constrain width
<TableCell sx={{ width: 240, maxWidth: 240 }}>
  <Tooltip title={monitor.url}>
    <Typography
      variant="caption"
      sx={{
        fontFamily: 'monospace',
        color: 'text.secondary',
        display: 'block',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        maxWidth: 220,
      }}
    >
      {monitor.url}
    </Typography>
  </Tooltip>
</TableCell>
```

---

## Fix 4 — Create Monitor Modal

`src/pages/synthetics/components/CreateMonitorModal.tsx`

Width: `520px`

```tsx
// Form fields with React Hook Form + Zod

const monitorSchema = z.object({
  name:      z.string().min(2, 'Name is required'),
  type:      z.enum(['HTTP', 'TCP', 'SSL', 'DNS', 'Journey']),
  url:       z.string().min(1, 'URL is required'),
  frequency: z.string(),
  locations: z.array(z.string()).min(1, 'Select at least one location'),
  notifyOnFailure: z.boolean(),
  notifyChannel:   z.string().optional(),
});
```

### Modal layout

```
Create Monitor                              [×]
──────────────────────────────────────────────

BASIC INFO

Name
[_______________________________________]

Type
[HTTP ▼]   (HTTP, TCP, SSL, DNS, Journey)

URL / Host
[_______________________________________]
Helper text changes by type:
  HTTP    → "https://example.com/health"
  TCP     → "hostname:port"
  SSL     → "https://example.com"
  DNS     → "example.com"
  Journey → "https://example.com/login"

──────────────────────────────────────────────

SCHEDULE

Check Frequency
[1 minute ▼]
Options: 30s, 1m, 2m, 5m, 10m, 15m, 30m, 1h

──────────────────────────────────────────────

LOCATIONS

[✓] US East (N. Virginia)
[✓] EU West (Ireland)
[ ] Asia Pacific (Singapore)
[ ] US West (Oregon)
[ ] South America (São Paulo)

──────────────────────────────────────────────

HTTP OPTIONS (only show when type = HTTP)

Method:  [GET ▼]   (GET, POST, HEAD)

Expected status: [200___]

Follow redirects: [✓]

Request headers: [+ Add Header]
  Key [____________]  Value [____________]  [×]

──────────────────────────────────────────────

NOTIFICATIONS

[✓] Notify on failure
    Channel: [Slack #alerts ▼]

[✓] Notify on recovery

──────────────────────────────────────────────

              [Cancel]    [Create Monitor]
```

On submit:
- Add to monitors list (local state)
- `notify.success('Monitor created successfully')`
- Close modal

Edit mode (opened from row edit button):
- Pre-fill all fields with existing monitor data
- Submit button says "Update Monitor"
- `notify.success('Monitor updated')`

---

## Fix 5 — Monitor Detail Drawer

`src/pages/synthetics/components/MonitorDetailDrawer.tsx`

Width: `480px`, anchor right

### Header
```
API Gateway Health                    [Edit]  [×]
● Up  •  https://api.obsadmin.io/health  •  HTTP
Last checked: 30s ago
```

### Tabs: Overview | History | Errors

#### Overview Tab
```
┌──────────┬──────────┬──────────┐
│Availability│Avg Duration│Checks/day│
│  99.9%   │   45ms   │  1,440   │
└──────────┴──────────┴──────────┘

DURATION TREND (last 24h)
[ECharts line chart — p50/p75/p95/max, height 160px]
Colors: #10b981, #f59e0b, #ef4444, #8b5cf6

STATUS HISTORY (last 24h)
[Row of 288 × 5-min colored squares]
● green = up  ● red = down  ● amber = degraded
Hover tooltip: "14:35 — Up — 43ms"

MONITOR DETAILS
Type            HTTP
Method          GET
URL             https://api.obsadmin.io/health
Expected Status 200
Frequency       Every 1 minute
Locations       US East, EU West
Created         2 days ago
```

#### History Tab
Table of recent check results:
```
TIME          STATUS   DURATION   LOCATION
14:23:01      ● Up     43ms       US East
14:22:01      ● Up     41ms       US East
14:21:01      ● Up     47ms       EU West
14:20:01      ● Up     44ms       US East
...
```

Row height: 28px, monospace timestamps

#### Errors Tab
If no errors:
```
[EmptyState icon="CheckCircle" title="No errors" 
 description="This monitor has been running without errors"]
```

If errors exist:
```
TIME          ERROR                    DURATION
14:15:00      Connection timeout       —
13:58:00      HTTP 503 returned        —
```

---

## Fix 6 — Status Page: Fix "All Systems Operational" Logic

`src/pages/synthetics/components/StatusPage.tsx`

```typescript
// Derive overall status from services dynamically
const overallStatus = useMemo(() => {
  const hasOutage    = statusServices.some(s => s.status === 'outage');
  const hasDegraded  = statusServices.some(s => s.status === 'degraded');
  if (hasOutage)   return { label: 'Major Outage',          color: '#ef4444' };
  if (hasDegraded) return { label: 'Partial Degradation',   color: '#f59e0b' };
  return             { label: 'All Systems Operational',    color: '#10b981' };
}, [statusServices]);

// Header reflects actual status:
<Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 3 }}>
  <Box sx={{ width: 10, height: 10, borderRadius: '50%', background: overallStatus.color }} />
  <Typography variant="h3" sx={{ color: overallStatus.color }}>
    {overallStatus.label}
  </Typography>
</Box>
```

Status services data with correct statuses:
```typescript
const statusServices = [
  { name: 'API Gateway',    status: 'operational' },
  { name: 'Authentication', status: 'operational' },
  { name: 'Search',         status: 'outage'      }, // ← matches monitors
  { name: 'Payments',       status: 'operational' },
  { name: 'Notifications',  status: 'operational' },
];
```

---

## Fix 7 — Status Page: Add Recent Incidents Section

Below the 90-day uptime grid:

```tsx
import { mockIncidents } from '../../incidents/mockData';

// Show last 3 incidents
const recentIncidents = mockIncidents.slice(0, 3);

<Box sx={{ mt: 4 }}>
  <Typography variant="h4" sx={{ mb: 2 }}>Recent Incidents</Typography>
  {recentIncidents.map(incident => (
    <Box
      key={incident.id}
      sx={{
        borderLeft: `3px solid ${
          incident.severity === 'critical' ? '#ef4444' :
          incident.severity === 'high'     ? '#f97316' : '#f59e0b'
        }`,
        pl: 2, py: 1, mb: 1,
        background: 'background.paper',
        borderRadius: '0 4px 4px 0',
      }}
    >
      <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
        <Typography variant="body2" fontWeight={500}>
          {incident.title}
        </Typography>
        <Typography variant="caption" sx={{ color: 'text.secondary' }}>
          {incident.startedAt}
        </Typography>
      </Box>
      <Typography variant="caption" sx={{ color: 'text.secondary' }}>
        {incident.status === 'resolved' ? '✓ Resolved' : '⚠ Ongoing'}
        {' • '}
        {incident.affectedServices.join(', ')}
      </Typography>
    </Box>
  ))}
</Box>
```

---

## Fix 8 — 90-Day Uptime Grid: Larger Squares + Tooltips

```tsx
// Each square: 12x12px with gap, tooltip on hover
{uptimeData.map((day, i) => (
  <Tooltip
    key={i}
    title={
      <Box>
        <Typography variant="caption" display="block">
          {day.date}
        </Typography>
        <Typography variant="caption" display="block">
          Uptime: {day.uptime}%
        </Typography>
        {day.incidents > 0 && (
          <Typography variant="caption" display="block" sx={{ color: '#ef4444' }}>
            {day.incidents} incident{day.incidents > 1 ? 's' : ''}
          </Typography>
        )}
      </Box>
    }
    arrow
  >
    <Box
      sx={{
        width: 14,
        height: 14,
        borderRadius: '2px',
        background:
          day.uptime === 100  ? '#10b981' :
          day.uptime >= 99    ? '#10b981' :
          day.uptime >= 95    ? '#f59e0b' : '#ef4444',
        cursor: 'pointer',
        '&:hover': { opacity: 0.8, transform: 'scale(1.2)' },
        transition: 'transform 0.1s',
      }}
    />
  </Tooltip>
))}
```

Generate 90 days of mock uptime data:
```typescript
export const uptimeData = Array.from({ length: 90 }, (_, i) => {
  const date = new Date();
  date.setDate(date.getDate() - (89 - i));
  const rand = Math.random();
  return {
    date: date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }),
    uptime: rand > 0.95 ? +(90 + Math.random() * 5).toFixed(1) :
            rand > 0.85 ? +(95 + Math.random() * 4).toFixed(1) :
                          100,
    incidents: rand > 0.95 ? 1 : rand > 0.92 ? 2 : 0,
  };
});
```

---

## Fix 9 — Status Page Visual Treatment

```tsx
// Add a proper header to status page
<Box sx={{
  background: 'linear-gradient(135deg, background.paper 0%, background.elevated 100%)',
  border: '1px solid divider',
  borderRadius: '4px',
  p: 3,
  mb: 3,
  textAlign: 'center',
}}>
  <Typography variant="h2" sx={{ mb: 1 }}>obsAdmin Status</Typography>
  <Typography variant="body2" sx={{ color: 'text.secondary', mb: 2 }}>
    Real-time status of obsAdmin services
  </Typography>
  {/* Overall status badge */}
  <Box sx={{
    display: 'inline-flex',
    alignItems: 'center',
    gap: 1,
    px: 2, py: 0.5,
    borderRadius: '20px',
    background: alpha(overallStatus.color, 0.12),
    border: `1px solid ${alpha(overallStatus.color, 0.3)}`,
  }}>
    <Box sx={{ width: 8, height: 8, borderRadius: '50%', background: overallStatus.color }} />
    <Typography variant="body2" fontWeight={500} sx={{ color: overallStatus.color }}>
      {overallStatus.label}
    </Typography>
  </Box>
  <Typography variant="caption" display="block" sx={{ color: 'text.disabled', mt: 1 }}>
    Last updated: just now
  </Typography>
</Box>
```

---

## Prompt for Deepseek

```
Read all files in docs/design-system/.
Now read docs/design-system/phase7c-fixes.md.

Apply ALL fixes in order:

1. Add search/filter toolbar above monitors table — Fix 1
   Wire search + type + status filters to filteredMonitors

2. Add Actions column to monitors table — Fix 2
   Pause/Edit/Delete buttons, show on row hover
   Row click opens MonitorDetailDrawer

3. Fix URL column max-width with truncation + tooltip — Fix 3

4. Build CreateMonitorModal.tsx — Fix 4
   Full form with React Hook Form + Zod
   HTTP Options section only shows when type = HTTP
   Edit mode pre-fills form

5. Build MonitorDetailDrawer.tsx — Fix 5
   480px drawer, 3 tabs: Overview | History | Errors
   Duration trend ECharts chart
   Status history squares row

6. Fix Status Page overall status logic — Fix 6
   Derive from actual service statuses dynamically
   Never hardcode "All Systems Operational"

7. Add Recent Incidents section to Status Page — Fix 7
   Import from incidents mockData
   Show last 3 incidents with severity border

8. Fix 90-day uptime squares — Fix 8
   14x14px squares, MUI Tooltip on each
   Generate 90 days mock data with incidents count

9. Add Status Page header treatment — Fix 9
   Centered header with gradient background
   Dynamic status badge

CRITICAL:
- CreateMonitorModal must use React Hook Form + Zod
- Edit mode must pre-fill all form fields
- Status page overall status must be DERIVED not hardcoded
- Uptime tooltip must show date + uptime % + incident count
- All colors through theme tokens — no hardcoded hex
```
