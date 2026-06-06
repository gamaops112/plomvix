# Phase 6 — Alerts & Incidents Spec
## obsAdmin

---

## File Structure

```
src/
├── pages/
│   ├── alerts/
│   │   ├── index.tsx                      ← alerts page (tabs)
│   │   ├── components/
│   │   │   ├── AlertsSummaryBar.tsx        ← top stat row
│   │   │   ├── FiringAlertsTable.tsx       ← active alerts table
│   │   │   ├── AlertRulesTable.tsx         ← all alert rules
│   │   │   ├── AlertDetailDrawer.tsx       ← slide-out alert detail
│   │   │   ├── AlertDetailPage.tsx         ← full page alert detail
│   │   │   ├── CreateAlertModal.tsx        ← new alert rule modal
│   │   │   └── NotificationChannels.tsx    ← channels config tab
│   │   └── mockData.ts
│   └── incidents/
│       ├── index.tsx                       ← incidents list page
│       ├── components/
│       │   ├── IncidentsList.tsx           ← incidents table
│       │   ├── IncidentDetailDrawer.tsx    ← slide-out incident detail
│       │   ├── IncidentDetailPage.tsx      ← full page /incidents/:id
│       │   └── IncidentTimeline.tsx        ← timeline of events
│       └── mockData.ts
```

---

## Page 1 — Alerts `/alerts`

### Tabs
```
[Firing Alerts]  [Alert Rules]  [Notification Channels]
```

### Summary Bar (above tabs)

5 stat chips in a row:
```
🔴 Critical: 2   🟠 High: 3   🟡 Warning: 5   🔵 Info: 4   ✅ Resolved today: 12
```

| Severity | Color |
|---|---|
| Critical | `#ef4444` |
| High | `#f97316` |
| Warning | `#f59e0b` |
| Info | `#06b6d4` |
| Resolved | `#10b981` |

Each chip: background tinted, border matching color, count bold.

---

### Tab 1 — Firing Alerts

#### Toolbar
```
[Search alerts...]  [Severity ▼]  [Service ▼]  [Status ▼]  [+ Create Alert Rule]
```

#### Firing Alerts Table

Columns:
| Column | Width | Content |
|---|---|---|
| Severity | 90px | colored chip |
| Alert Name | 220px | alert rule name |
| Service | 140px | service name chip |
| Condition | 200px | what triggered it |
| Value | 100px | current metric value |
| Duration | 100px | how long firing |
| Started | 120px | timestamp |
| Actions | 80px | drawer + page + silence |

Row height: 36px
Click row → AlertDetailDrawer
Click name → `/alerts/:id`

```typescript
export const mockFiringAlerts = [
  {
    id: 'al1', severity: 'critical', name: 'search-service Down',
    service: 'search-service', condition: 'Error rate > 5%',
    value: '8.4%', duration: '12m', started: '14:11:00', status: 'firing',
    assignee: null, silenced: false,
  },
  {
    id: 'al2', severity: 'critical', name: 'High DB Latency',
    service: 'storage-service', condition: 'P99 latency > 2s',
    value: '4.2s', duration: '8m', started: '14:15:00', status: 'firing',
    assignee: 'John D.', silenced: false,
  },
  {
    id: 'al3', severity: 'high', name: 'User Service Slow',
    service: 'user-service', condition: 'P95 latency > 500ms',
    value: '891ms', duration: '22m', started: '14:01:00', status: 'firing',
    assignee: null, silenced: false,
  },
  {
    id: 'al4', severity: 'high', name: 'Queue Depth Threshold',
    service: 'queue-service', condition: 'Queue depth > 1000',
    value: '1,847', duration: '15m', started: '14:08:00', status: 'firing',
    assignee: null, silenced: false,
  },
  {
    id: 'al5', severity: 'high', name: 'Payment Timeout Spike',
    service: 'payment-service', condition: 'Timeout rate > 1%',
    value: '2.3%', duration: '5m', started: '14:18:00', status: 'firing',
    assignee: 'Jane S.', silenced: false,
  },
  {
    id: 'al6', severity: 'warning', name: 'High Memory Usage',
    service: 'api-gateway', condition: 'Memory usage > 80%',
    value: '84%', duration: '34m', started: '13:49:00', status: 'firing',
    assignee: null, silenced: false,
  },
  {
    id: 'al7', severity: 'warning', name: 'CPU Spike Detected',
    service: 'user-service', condition: 'CPU usage > 70%',
    value: '78%', duration: '18m', started: '14:05:00', status: 'firing',
    assignee: null, silenced: true,
  },
  {
    id: 'al8', severity: 'info', name: 'Auto-scaling Triggered',
    service: 'api-gateway', condition: 'Instance count changed',
    value: '+2', duration: '2m', started: '14:21:00', status: 'firing',
    assignee: null, silenced: false,
  },
];
```

Silenced rows: opacity 0.5, "SILENCED" chip instead of duration

---

### Alert Detail Drawer (480px)

#### Header
```
🔴 CRITICAL                              [Open full page →]  [×]
search-service Down
search-service  •  firing for 12m  •  started 14:11:00
```

#### Tabs: Overview | History | Related Traces | Runbook

**Overview tab**
```
CONDITION
Error rate > 5% for 5 minutes

CURRENT VALUE
8.4%  (threshold: 5%)

[Mini ECharts line chart showing metric value over time
 with threshold line as dashed red horizontal line]

DETAILS
Service         search-service
Triggered by    Error rate
Threshold       > 5%
Duration        12 minutes
Notification    Slack #alerts, PagerDuty
Assignee        Unassigned  [Assign →]

ACTIONS
[Silence 1h]  [Silence 4h]  [Acknowledge]  [Resolve]
```

Threshold line on chart: dashed `#ef4444`, label "Threshold: 5%"
Current value line: solid `#ef4444`

**History tab**
Timeline of state changes:
```
● 14:11:00  Alert fired — error rate reached 8.4%
● 14:06:00  Warning — error rate exceeded 3%
● 13:58:00  Resolved — previous firing resolved
● 13:45:00  Alert fired — error rate reached 6.1%
```
Each event: colored dot + timestamp + description
Dot colors match severity

**Related Traces tab**
Reuse TracesTable mini, pre-filtered to service + time range of alert

**Runbook tab**
Monaco editor read-only, markdown content:
```markdown
# search-service Down Runbook

## Symptoms
- Error rate above 5%
- Possible Redis connection failures
- Increased response times

## Investigation Steps
1. Check search-service logs for connection errors
2. Verify Redis cluster health
3. Check network connectivity between pods

## Resolution
- Restart search-service pods if Redis is healthy
- Scale Redis if memory pressure detected
```

---

### Alert Detail Full Page `/alerts/:id`

```
← Back to Alerts

🔴 CRITICAL  search-service Down         [Silence]  [Acknowledge]  [Resolve]
search-service  •  firing for 12m

┌──────────┬──────────┬──────────┬──────────┐
│ Status   │ Duration │ Value    │ Threshold│
│ FIRING   │ 12m      │ 8.4%     │ 5%       │
└──────────┴──────────┴──────────┴──────────┘

[Full metric chart with threshold line — 320px height]

[Tabs: Overview | History | Related Traces | Runbook]
```

---

### Tab 2 — Alert Rules

#### Toolbar
```
[Search rules...]  [Status ▼]  [Severity ▼]  [+ Create Rule]
```

#### Alert Rules Table

```typescript
export const mockAlertRules = [
  { id: 'r1', name: 'High Error Rate',      service: 'any',             condition: 'error_rate > 5%',        severity: 'critical', status: 'enabled',  lastFired: '12m ago',  notifications: ['slack', 'pagerduty'] },
  { id: 'r2', name: 'High Latency P99',     service: 'any',             condition: 'latency_p99 > 2s',       severity: 'critical', status: 'enabled',  lastFired: '8m ago',   notifications: ['slack', 'pagerduty'] },
  { id: 'r3', name: 'Service Down',         service: 'any',             condition: 'availability < 99%',     severity: 'critical', status: 'enabled',  lastFired: '2h ago',   notifications: ['slack', 'pagerduty', 'email'] },
  { id: 'r4', name: 'High Memory Usage',    service: 'any',             condition: 'memory_usage > 80%',     severity: 'warning',  status: 'enabled',  lastFired: '34m ago',  notifications: ['slack'] },
  { id: 'r5', name: 'CPU Spike',            service: 'any',             condition: 'cpu_usage > 70%',        severity: 'warning',  status: 'enabled',  lastFired: '18m ago',  notifications: ['slack'] },
  { id: 'r6', name: 'Queue Depth',          service: 'queue-service',   condition: 'queue_depth > 1000',     severity: 'high',     status: 'enabled',  lastFired: '15m ago',  notifications: ['slack'] },
  { id: 'r7', name: 'Payment Timeout',      service: 'payment-service', condition: 'timeout_rate > 1%',      severity: 'high',     status: 'enabled',  lastFired: '5m ago',   notifications: ['slack', 'pagerduty'] },
  { id: 'r8', name: 'Disk Space Low',       service: 'any',             condition: 'disk_usage > 85%',       severity: 'warning',  status: 'disabled', lastFired: '3d ago',   notifications: ['email'] },
  { id: 'r9', name: 'SSL Cert Expiry',      service: 'any',             condition: 'cert_expiry < 30d',      severity: 'info',     status: 'enabled',  lastFired: 'never',    notifications: ['email'] },
  { id: 'r10',name: 'Auto-scale Event',     service: 'any',             condition: 'instance_count_changed', severity: 'info',     status: 'enabled',  lastFired: '2m ago',   notifications: ['slack'] },
];
```

Columns: Severity chip, Name, Service, Condition (mono), Status toggle, Last Fired, Notifications (icon chips), Actions (edit/delete)

Notification icons:
- Slack: `#` icon chip, green
- PagerDuty: `P` icon chip, green  
- Email: `@` icon chip, cyan

Status toggle: MUI `<Switch>` inline in table

---

### Create Alert Rule Modal

Width: `560px`

```
Create Alert Rule                          [×]
─────────────────────────────────────────────
BASIC INFO
Name:          [________________]
Severity:      [Critical ▼]
Service:       [Any ▼]

CONDITION
Metric:        [error_rate ▼]
Operator:      [> ▼]
Threshold:     [____]  %
For:           [5 ▼] minutes

NOTIFICATIONS
[✓] Slack   Channel: [#alerts____]
[✓] PagerDuty
[ ] Email

RUNBOOK URL
[________________________________]

              [Cancel]  [Create Rule]
```

Use React Hook Form + Zod validation.
Required fields: Name, Metric, Threshold.

---

### Tab 3 — Notification Channels

```typescript
export const mockChannels = [
  { id: 'c1', type: 'slack',     name: 'Slack #alerts',      status: 'connected', lastTest: '2h ago',  config: { webhook: 'https://hooks.slack.com/...' } },
  { id: 'c2', type: 'slack',     name: 'Slack #incidents',   status: 'connected', lastTest: '1d ago',  config: { webhook: 'https://hooks.slack.com/...' } },
  { id: 'c3', type: 'pagerduty', name: 'PagerDuty On-call',  status: 'connected', lastTest: '6h ago',  config: { apiKey: 'pd_****' } },
  { id: 'c4', type: 'email',     name: 'Ops Team Email',     status: 'connected', lastTest: '3d ago',  config: { to: 'ops@company.com' } },
  { id: 'c5', type: 'webhook',   name: 'Custom Webhook',     status: 'error',     lastTest: '1h ago',  config: { url: 'https://...' } },
];
```

Channel cards in a grid (3 per row):
```
┌──────────────────────────┐
│ [Slack icon]  Slack      │
│ #alerts                  │
│ ● Connected  2h ago test │
│                          │
│ [Test]  [Edit]  [Delete] │
└──────────────────────────┘
```

Error channel: border `#ef444440`, status dot red
`[+ Add Channel]` button opens modal with type selector

---

## Page 2 — Incidents `/incidents`

### Page header
```
Incidents                              [+ Create Incident]
```

### Summary chips
```
🔴 Open: 1   🟡 Investigating: 2   🟢 Resolved today: 4
```

### Incidents Table

```typescript
export const mockIncidents = [
  {
    id: 'inc1', title: 'search-service outage',
    severity: 'critical', status: 'open',
    affectedServices: ['search-service', 'api-gateway'],
    startedAt: '14:11:00', duration: '12m',
    assignee: 'John D.', alerts: 2, updates: 3,
  },
  {
    id: 'inc2', title: 'Elevated latency across user-service',
    severity: 'high', status: 'investigating',
    affectedServices: ['user-service', 'storage-service'],
    startedAt: '14:01:00', duration: '22m',
    assignee: 'Jane S.', alerts: 3, updates: 5,
  },
  {
    id: 'inc3', title: 'Payment service timeout spike',
    severity: 'high', status: 'investigating',
    affectedServices: ['payment-service'],
    startedAt: '13:55:00', duration: '28m',
    assignee: null, alerts: 1, updates: 2,
  },
  {
    id: 'inc4', title: 'API gateway memory pressure',
    severity: 'warning', status: 'resolved',
    affectedServices: ['api-gateway'],
    startedAt: '12:30:00', duration: '45m',
    assignee: 'John D.', alerts: 1, updates: 4,
  },
];
```

Columns: Severity, Title, Status chip, Affected Services (chips), Started, Duration, Assignee, Alerts count, Actions

Status chips:
- open: red
- investigating: amber
- resolved: green
- postmortem: cyan

---

## Incident Detail Drawer (560px)

#### Header
```
🔴 CRITICAL  INC-001                    [Open full page →]  [×]
search-service outage
Open  •  started 14:11:00  •  12 minutes

[Acknowledge]  [Resolve]  [Create Postmortem]
```

#### Tabs: Overview | Timeline | Alerts | Runbook

**Overview tab**
```
AFFECTED SERVICES
[search-service]  [api-gateway]

SUMMARY
Service became unavailable at 14:11. Redis connection
failures detected. Team investigating.

ASSIGNEE
[Avatar] John D.  [Reassign]

RESPONDERS
[+ Add Responder]

LINKS
[+ Add Link]  (runbook, dashboard, etc)
```

**Timeline tab** — IncidentTimeline component
```
● 14:23  John D. added a note: "Checking Redis cluster"
● 14:19  Alert acknowledged
● 14:15  John D. assigned as incident commander
● 14:13  Incident severity updated: warning → critical
● 14:11  Incident created automatically from alert
● 14:11  Alert fired: search-service Down (error rate 8.4%)
```

Each event:
- Colored dot by event type (alert=red, note=cyan, assignment=purple, status change=amber)
- Timestamp: mono, `caption2`
- Actor + description: `body2`
- [Add note] input at bottom: TextField + Send button

**Alerts tab**
List of linked alerts — reuse FiringAlertsTable mini version

---

## Incident Detail Full Page `/incidents/:id`

```
← Back to Incidents

🔴 CRITICAL  INC-001                    [Acknowledge]  [Resolve]  [Create Postmortem]
search-service outage  •  Open  •  12 minutes

┌────────────┬────────────┬────────────┬────────────┐
│ Status     │ Duration   │ Alerts     │ Responders │
│ OPEN       │ 12m        │ 2          │ 1          │
└────────────┴────────────┴────────────┴────────────┘

┌──────────────────────────┬──────────────────────────┐
│  TIMELINE (left, xs=5)   │  ALERTS (right, xs=7)    │
│  IncidentTimeline        │  FiringAlertsTable mini  │
│  with note input         │                          │
│                          │                          │
│                          │                          │
└──────────────────────────┴──────────────────────────┘
```

---

## Prompt for Deepseek

```
Read all files in docs/design-system/.
Now read docs/design-system/phase6-alerts-incidents.md.

Build in this order:

PART A — Alerts page
1. src/pages/alerts/mockData.ts
2. src/pages/alerts/components/AlertsSummaryBar.tsx
3. src/pages/alerts/components/FiringAlertsTable.tsx
4. src/pages/alerts/components/AlertRulesTable.tsx — with inline Switch toggle
5. src/pages/alerts/components/AlertDetailDrawer.tsx — 480px, 4 tabs, metric chart with threshold line
6. src/pages/alerts/components/AlertDetailPage.tsx — full page /alerts/:id
7. src/pages/alerts/components/CreateAlertModal.tsx — React Hook Form + Zod
8. src/pages/alerts/components/NotificationChannels.tsx — channel cards grid
9. src/pages/alerts/index.tsx — 3 tabs

PART B — Incidents page
10. src/pages/incidents/mockData.ts
11. src/pages/incidents/components/IncidentsList.tsx
12. src/pages/incidents/components/IncidentTimeline.tsx — with add note input
13. src/pages/incidents/components/IncidentDetailDrawer.tsx — 560px, 4 tabs
14. src/pages/incidents/components/IncidentDetailPage.tsx — full page /incidents/:id
15. src/pages/incidents/index.tsx

PART C — Router
16. Add /alerts/:id and /incidents/:id routes to App.tsx

RULES:
- Metric chart in AlertDetailDrawer must show threshold as dashed red horizontal line
- Silenced alert rows: opacity 0.5
- Status toggle in AlertRulesTable: inline MUI Switch
- IncidentTimeline: colored dots by event type
- CreateAlertModal: validate with Zod — name required, threshold must be a number
- All severity colors from theme tokens only

Do not skip any component. Do not invent features not in spec.
```
