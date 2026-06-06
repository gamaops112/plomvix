# Phase 7E — Complete Polish Pass
## obsAdmin

---

## SECTION 1 — Breadcrumbs (Dynamic)

File: `src/layout/Topbar.tsx`

```typescript
import { useLocation, useParams } from 'react-router-dom';

const breadcrumbMap: Record<string, string> = {
  '/':               'Dashboard',
  '/logs':           'Logs',
  '/traces':         'Traces',
  '/metrics':        'Metrics',
  '/apm':            'APM',
  '/synthetics':     'Monitors',
  '/alerts':         'Alerts',
  '/incidents':      'Incidents',
  '/users':          'Users',
  '/integrations':   'Integrations',
  '/settings':       'Settings',
  '/profile':        'Profile',
  '/demo':           'Demo Data',
  '/docs':           'Documentation',
};

const useBreadcrumbs = () => {
  const location = useLocation();
  const path = location.pathname;

  // Handle dynamic segments
  if (path.startsWith('/traces/'))
    return ['Traces', `Trace: ${path.split('/')[2].substring(0, 8)}`];
  if (path.startsWith('/alerts/'))
    return ['Alerts', `Alert: ${path.split('/')[2].substring(0, 8)}`];
  if (path.startsWith('/incidents/'))
    return ['Incidents', `INC-${path.split('/')[2]}`];
  if (path.startsWith('/metrics/hosts/'))
    return ['Metrics', 'Infrastructure', path.split('/')[3]];

  const label = breadcrumbMap[path] ?? path.split('/').pop() ?? 'Page';
  return [label];
};

// Always prefix: Observability > Infrastructure > [breadcrumbs]
const crumbs = ['Observability', 'Infrastructure', ...useBreadcrumbs()];
```

---

## SECTION 2 — Notification Bell Dropdown

File: `src/layout/NotificationBell.tsx`

```typescript
// State
const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
const [notifications, setNotifications] = useState(mockNotifications);
const unreadCount = notifications.filter(n => !n.read).length;

// Bell button with badge
<IconButton onClick={(e) => setAnchorEl(e.currentTarget)}>
  <Badge badgeContent={unreadCount} color="error" max={99}>
    <Bell size={18} />
  </Badge>
</IconButton>

// Popover
<Popover
  open={Boolean(anchorEl)}
  anchorEl={anchorEl}
  onClose={() => setAnchorEl(null)}
  anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
  transformOrigin={{ vertical: 'top', horizontal: 'right' }}
  PaperProps={{
    sx: {
      width: 380,
      maxHeight: 480,
      background: 'background.elevated',
      border: '1px solid divider',
      borderRadius: '4px',
      boxShadow: 'none',
    }
  }}
>
```

### Notification mock data
```typescript
export const mockNotifications = [
  {
    id: 'n1', read: false, severity: 'critical',
    title: 'search-service is down',
    description: 'Error rate exceeded 5% threshold',
    time: '2m ago', link: '/alerts',
  },
  {
    id: 'n2', read: false, severity: 'warning',
    title: 'High latency on user-service',
    description: 'P95 latency: 891ms (threshold: 500ms)',
    time: '8m ago', link: '/alerts',
  },
  {
    id: 'n3', read: false, severity: 'warning',
    title: 'Queue depth above threshold',
    description: 'queue-service depth: 1,847',
    time: '15m ago', link: '/alerts',
  },
  {
    id: 'n4', read: true, severity: 'success',
    title: 'Deployment completed',
    description: 'auth-service v2.4.1 deployed successfully',
    time: '22m ago', link: '/apm',
  },
  {
    id: 'n5', read: true, severity: 'info',
    title: 'Auto-scaled: added 3 instances',
    description: 'api-gateway scaled up due to load',
    time: '41m ago', link: '/metrics',
  },
  {
    id: 'n6', read: true, severity: 'critical',
    title: 'Payment timeout spike resolved',
    description: 'Alert resolved after 34 minutes',
    time: '1h ago', link: '/alerts',
  },
];
```

### Popover layout
```
┌─────────────────────────────────────────┐
│ Notifications (3)        [Mark all read]│
├─────────────────────────────────────────┤
│ 🔴 search-service is down        2m ●  │
│    Error rate exceeded 5%               │
├─────────────────────────────────────────┤
│ 🟡 High latency on user-service   8m ●  │
│    P95 latency: 891ms                   │
├─────────────────────────────────────────┤
│ 🟡 Queue depth above threshold   15m ●  │
│    queue-service depth: 1,847           │
├─────────────────────────────────────────┤
│ ✅ Deployment completed          22m    │
│    auth-service v2.4.1 deployed         │
├─────────────────────────────────────────┤
│           [View all alerts →]           │
└─────────────────────────────────────────┘
```

Unread rows: `background: alpha(primary.main, 0.04)`, bold title, blue dot
Click row: mark as read + navigate to link + close popover
Mark all read: sets all `read: true`, badge disappears

---

## SECTION 3 — Global Search ⌘K

File: `src/components/common/GlobalSearch.tsx`

### Keyboard shortcut wire in `AppShell.tsx`
```typescript
useEffect(() => {
  const handler = (e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      setSearchOpen(true);
    }
  };
  window.addEventListener('keydown', handler);
  return () => window.removeEventListener('keydown', handler);
}, []);
```

Also wire the topbar search bar `onClick` to open the modal.

### Modal
```typescript
<Modal open={searchOpen} onClose={() => setSearchOpen(false)}>
  <Box sx={{
    position: 'absolute',
    top: '15%',
    left: '50%',
    transform: 'translateX(-50%)',
    width: 600,
    maxHeight: '70vh',
    background: 'background.elevated',
    border: '1px solid divider',
    borderRadius: '6px',
    overflow: 'hidden',
    outline: 'none',
  }}>
```

### Search input
```tsx
<Box sx={{
  display: 'flex',
  alignItems: 'center',
  gap: 1.5,
  p: '12px 16px',
  borderBottom: '1px solid divider',
}}>
  <Search size={16} color="text.secondary" />
  <InputBase
    autoFocus
    fullWidth
    placeholder="Search services, logs, traces, pages..."
    value={query}
    onChange={(e) => setQuery(e.target.value)}
    onKeyDown={handleKeyNav}
    sx={{ fontSize: '14px' }}
  />
  <Chip label="Esc" size="small" sx={{ fontSize: '10px', height: 20 }} />
</Box>
```

### Search results structure
```typescript
interface SearchResult {
  id: string;
  type: 'page' | 'service' | 'trace' | 'alert' | 'action';
  label: string;
  description?: string;
  icon: LucideIcon;
  link?: string;
  action?: () => void;
}

const getResults = (query: string): SearchResult[] => {
  if (!query) return getDefaults(); // recent + quick actions
  return [
    ...searchPages(query),
    ...searchServices(query),
    ...searchTraces(query),
    ...searchAlerts(query),
  ];
};
```

### Default state (no query)
```
RECENT
📊  Dashboard
📋  Logs — last search
🔀  Trace a3f9c2b1

SERVICES
● api-gateway      healthy      → /apm
● search-service   down         → /apm
● user-service     degraded     → /apm

QUICK ACTIONS
⚡  Create alert rule            → opens CreateAlertModal
⚡  Invite team member           → opens InviteMemberModal
⚡  Load demo data               → /demo
⚡  Toggle theme                 → toggleThemeMode()
```

### With query
```typescript
const searchPages = (q: string) =>
  Object.entries(breadcrumbMap)
    .filter(([, label]) => label.toLowerCase().includes(q.toLowerCase()))
    .map(([path, label]) => ({
      id: path, type: 'page' as const,
      label, icon: getPageIcon(path), link: path,
    }));

const searchServices = (q: string) =>
  mockServices
    .filter(s => s.name.toLowerCase().includes(q.toLowerCase()))
    .map(s => ({
      id: s.name, type: 'service' as const,
      label: s.name,
      description: `${s.status} • ${s.reqRate} req/s`,
      icon: Activity,
      link: '/apm',
    }));

const searchTraces = (q: string) =>
  mockTraces
    .filter(t =>
      t.id.includes(q) ||
      t.rootOp.toLowerCase().includes(q.toLowerCase()) ||
      t.rootService.toLowerCase().includes(q.toLowerCase())
    )
    .slice(0, 3)
    .map(t => ({
      id: t.id, type: 'trace' as const,
      label: t.rootOp,
      description: `${t.rootService} • ${t.duration}ms`,
      icon: GitBranch,
      link: `/traces/${t.id}`,
    }));
```

### Keyboard navigation
```typescript
const handleKeyNav = (e: React.KeyboardEvent) => {
  if (e.key === 'ArrowDown') {
    e.preventDefault();
    setFocusedIndex(i => Math.min(i + 1, results.length - 1));
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault();
    setFocusedIndex(i => Math.max(i - 1, 0));
  }
  if (e.key === 'Enter' && results[focusedIndex]) {
    handleSelect(results[focusedIndex]);
  }
  if (e.key === 'Escape') {
    setSearchOpen(false);
  }
};
```

### Result row rendering
```tsx
{results.map((result, index) => (
  <Box
    key={result.id}
    onClick={() => handleSelect(result)}
    sx={{
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
      px: 2, py: 1,
      cursor: 'pointer',
      background: focusedIndex === index
        ? 'background.hover'
        : 'transparent',
      '&:hover': { background: 'background.hover' },
    }}
  >
    <Box sx={{
      p: 0.75,
      borderRadius: '4px',
      background: 'background.surface',
      display: 'flex',
    }}>
      <result.icon size={14} />
    </Box>
    <Box sx={{ flex: 1 }}>
      <Typography variant="body2">{result.label}</Typography>
      {result.description && (
        <Typography variant="caption" sx={{ color: 'text.secondary' }}>
          {result.description}
        </Typography>
      )}
    </Box>
    <Typography variant="caption" sx={{ color: 'text.disabled' }}>
      {result.type}
    </Typography>
  </Box>
))}
```

---

## SECTION 4 — Loading Skeletons

File: `src/components/common/PageSkeleton.tsx`

```typescript
type SkeletonVariant = 'table' | 'cards' | 'chart' | 'dashboard' | 'detail';

const PageSkeleton = ({ variant }: { variant: SkeletonVariant }) => {
  const theme = useTheme();

  if (variant === 'table') return (
    <Box sx={{ p: 3 }}>
      <Skeleton width={200} height={32} sx={{ mb: 2 }} />
      <Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
        <Skeleton width={260} height={36} />
        <Skeleton width={130} height={36} />
        <Skeleton width={130} height={36} />
      </Box>
      {Array.from({ length: 8 }).map((_, i) => (
        <Skeleton key={i} height={36} sx={{ mb: 0.5 }} />
      ))}
    </Box>
  );

  if (variant === 'cards') return (
    <Box sx={{ p: 3 }}>
      <Skeleton width={200} height={32} sx={{ mb: 2 }} />
      <Grid container spacing={2}>
        {Array.from({ length: 6 }).map((_, i) => (
          <Grid item xs={4} key={i}>
            <Skeleton height={160} variant="rectangular" sx={{ borderRadius: '4px' }} />
          </Grid>
        ))}
      </Grid>
    </Box>
  );

  if (variant === 'dashboard') return (
    <Box sx={{ p: 3 }}>
      <Grid container spacing={2} sx={{ mb: 2 }}>
        {Array.from({ length: 4 }).map((_, i) => (
          <Grid item xs={3} key={i}>
            <Skeleton height={100} variant="rectangular" sx={{ borderRadius: '4px' }} />
          </Grid>
        ))}
      </Grid>
      <Grid container spacing={2} sx={{ mb: 2 }}>
        {Array.from({ length: 7 }).map((_, i) => (
          <Grid item xs key={i}>
            <Skeleton height={72} variant="rectangular" sx={{ borderRadius: '4px' }} />
          </Grid>
        ))}
      </Grid>
      <Grid container spacing={2}>
        <Grid item xs={8}>
          <Skeleton height={280} variant="rectangular" sx={{ borderRadius: '4px' }} />
        </Grid>
        <Grid item xs={4}>
          <Skeleton height={280} variant="rectangular" sx={{ borderRadius: '4px' }} />
        </Grid>
      </Grid>
    </Box>
  );

  if (variant === 'chart') return (
    <Box sx={{ p: 3 }}>
      <Skeleton width={200} height={32} sx={{ mb: 2 }} />
      <Skeleton height={320} variant="rectangular" sx={{ borderRadius: '4px' }} />
    </Box>
  );

  if (variant === 'detail') return (
    <Box sx={{ p: 3 }}>
      <Skeleton width={100} height={20} sx={{ mb: 2 }} />
      <Skeleton width={300} height={36} sx={{ mb: 1 }} />
      <Skeleton width={200} height={20} sx={{ mb: 3 }} />
      <Grid container spacing={2} sx={{ mb: 3 }}>
        {Array.from({ length: 4 }).map((_, i) => (
          <Grid item xs={3} key={i}>
            <Skeleton height={80} variant="rectangular" sx={{ borderRadius: '4px' }} />
          </Grid>
        ))}
      </Grid>
      <Skeleton height={240} variant="rectangular" sx={{ borderRadius: '4px' }} />
    </Box>
  );

  return null;
};
```

### Apply skeletons on initial page load

Add to these pages — show skeleton for 1200ms on first mount:

```typescript
// Pattern to use in every page component:
const [loading, setLoading] = useState(true);

useEffect(() => {
  const timer = setTimeout(() => setLoading(false), 1200);
  return () => clearTimeout(timer);
}, []);

if (loading) return <PageSkeleton variant="dashboard" />;
```

Apply to:
- `pages/dashboard/index.tsx` → variant `dashboard`
- `pages/logs/index.tsx` → variant `table`
- `pages/metrics/index.tsx` → variant `dashboard`
- `pages/traces/index.tsx` → variant `table`
- `pages/apm/index.tsx` → variant `cards`
- `pages/alerts/index.tsx` → variant `table`
- `pages/incidents/index.tsx` → variant `table`
- `pages/synthetics/index.tsx` → variant `table`

---

## SECTION 5 — Empty States

File: `src/components/common/EmptyState.tsx`

```typescript
interface EmptyStateProps {
  icon: React.ElementType;
  title: string;
  description: string;
  action?: {
    label: string;
    onClick: () => void;
    icon?: React.ElementType;
  };
}

const EmptyState = ({ icon: Icon, title, description, action }: EmptyStateProps) => (
  <Box sx={{
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    py: 8,
    px: 4,
    textAlign: 'center',
  }}>
    <Box sx={{
      p: 2,
      borderRadius: '50%',
      background: alpha(theme.palette.primary.main, 0.08),
      mb: 2,
    }}>
      <Icon size={28} color={theme.palette.text.disabled} />
    </Box>
    <Typography variant="h4" sx={{ mb: 1 }}>{title}</Typography>
    <Typography variant="body2" sx={{ color: 'text.secondary', maxWidth: 360, mb: action ? 3 : 0 }}>
      {description}
    </Typography>
    {action && (
      <Button
        variant="contained"
        size="small"
        startIcon={action.icon && <action.icon size={14} />}
        onClick={action.onClick}
      >
        {action.label}
      </Button>
    )}
  </Box>
);
```

### Apply empty states:

```typescript
// Logs — no results
<EmptyState
  icon={FileText}
  title="No logs found"
  description="Try adjusting your filters or time range to find what you're looking for."
/>

// Traces — no results
<EmptyState
  icon={GitBranch}
  title="No traces found"
  description="No traces match your current filters. Try expanding the time range."
/>

// Alerts — no firing alerts
<EmptyState
  icon={CheckCircle}
  title="All clear"
  description="No firing alerts at the moment. Your services are running smoothly."
/>

// Monitors — none created
<EmptyState
  icon={Radio}
  title="No monitors yet"
  description="Create your first monitor to start tracking uptime and performance."
  action={{ label: 'Create Monitor', icon: Plus, onClick: () => setCreateModalOpen(true) }}
/>

// Incidents — none open
<EmptyState
  icon={ShieldCheck}
  title="No open incidents"
  description="No active incidents. Check resolved incidents in the filter."
/>

// Integrations — filtered empty
<EmptyState
  icon={Layers}
  title="No integrations found"
  description="No integrations match this category."
/>

// Audit log — filtered empty
<EmptyState
  icon={Clock}
  title="No audit events found"
  description="No events match your current search or filters."
/>
```

---

## SECTION 6 — 404 Page

File: `src/pages/NotFoundPage.tsx`

```tsx
export default function NotFoundPage() {
  const navigate = useNavigate();
  return (
    <Box sx={{
      height: '100vh',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      background: 'background.default',
      gap: 2,
    }}>
      <Typography sx={{ fontSize: '72px', fontWeight: 700, color: 'text.disabled', lineHeight: 1 }}>
        404
      </Typography>
      <Typography variant="h3">Page not found</Typography>
      <Typography variant="body2" sx={{ color: 'text.secondary' }}>
        The page you're looking for doesn't exist or has been moved.
      </Typography>
      <Box sx={{ display: 'flex', gap: 1, mt: 1 }}>
        <Button variant="outlined" onClick={() => navigate(-1)} startIcon={<ArrowLeft size={14} />}>
          Go back
        </Button>
        <Button variant="contained" onClick={() => navigate('/')}>
          Dashboard
        </Button>
      </Box>
    </Box>
  );
}

// Add to App.tsx router:
<Route path="*" element={<NotFoundPage />} />
```

---

## SECTION 7 — Error Boundary

File: `src/components/common/ErrorBoundary.tsx`

```typescript
class ErrorBoundary extends React.Component<
  { children: React.ReactNode },
  { hasError: boolean; error: Error | null }
> {
  state = { hasError: false, error: null };

  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }

  render() {
    if (this.state.hasError) {
      return (
        <Box sx={{ p: 4, textAlign: 'center' }}>
          <AlertTriangle size={32} color="#ef4444" />
          <Typography variant="h3" sx={{ mt: 2, mb: 1 }}>
            Something went wrong
          </Typography>
          <Typography variant="body2" sx={{ color: 'text.secondary', mb: 3 }}>
            {this.state.error?.message}
          </Typography>
          <Button
            variant="contained"
            onClick={() => this.setState({ hasError: false, error: null })}
          >
            Try again
          </Button>
        </Box>
      );
    }
    return this.props.children;
  }
}

// Wrap each page route in App.tsx:
<Route path="/logs" element={
  <ErrorBoundary><Logs /></ErrorBoundary>
} />
```

---

## SECTION 8 — Wiring Broken Buttons

### Dashboard
```typescript
// "Edit Dashboard" — show notify.info
onClick={() => notify.info('Dashboard editing coming soon')

// "+ Add Widget" — same
onClick={() => notify.info('Widget library coming soon')

// Environment selector — store in settingsStore
const { defaultEnvironment, updateSettings } = useSettingsStore();
<Select value={defaultEnvironment} onChange={(e) => updateSettings({ defaultEnvironment: e.target.value })}>
```

### Logs
```typescript
// "View all logs →" — already on logs page, scroll to top
onClick={() => window.scrollTo({ top: 0, behavior: 'smooth' })

// Download button
onClick={() => {
  const data = JSON.stringify(filteredLogs, null, 2);
  const blob = new Blob([data], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url; a.download = 'logs-export.json'; a.click();
  notify.success('Logs exported');
}
```

### Traces
```typescript
// "Copy ID" button
onClick={() => {
  navigator.clipboard.writeText(traceId);
  notify.success('Trace ID copied to clipboard');
}

// "Share" button
onClick={() => {
  navigator.clipboard.writeText(window.location.href);
  notify.success('Link copied to clipboard');
}
```

### Alerts
```typescript
// Silence alert — actually update row state
const [silencedAlerts, setSilencedAlerts] = useState<string[]>([]);

const handleSilence = (alertId: string, duration: string) => {
  setSilencedAlerts(prev => [...prev, alertId]);
  notify.success(`Alert silenced for ${duration}`);
};

// Show silenced row at 50% opacity with SILENCED chip
```

### Incidents
```typescript
// "Create Postmortem" button
onClick={() => notify.info('Postmortem template coming soon')

// Add note — actually append to timeline
const [notes, setNotes] = useState<TimelineEvent[]>([]);

const handleAddNote = () => {
  if (!noteText.trim()) return;
  setNotes(prev => [{
    type: 'note',
    time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    actor: 'Demo User',
    description: noteText,
  }, ...prev]);
  setNoteText('');
  notify.success('Note added');
};
```

### Settings
```typescript
// "Save Changes" — persist via settingsStore (already persisted via Zustand)
// Just show toast since Zustand persist handles storage
onClick={() => notify.success('Settings saved')

// "Generate API Key"
const handleGenerateKey = () => {
  const newKey = {
    id: Date.now().toString(),
    name: newKeyName || 'New Key',
    created: 'just now',
    lastUsed: '—',
    scopes: selectedScopes,
    key: `sk_live_${Math.random().toString(36).substring(2, 34)}`,
  };
  setApiKeys(prev => [...prev, newKey]);
  notify.success('API key generated');
  setShowNewKeyModal(false);
};

// Copy API key
onClick={() => {
  navigator.clipboard.writeText(key.key);
  notify.success('API key copied');
}

// Data source "Test Connection"
onClick={() => {
  notify.loading('Testing connection...');
  setTimeout(() => notify.success('Connection successful'), 1500);
}
```

### Integrations
```typescript
// Wire category tab filter
const [category, setCategory] = useState('All');
const filtered = category === 'All'
  ? mockIntegrations
  : mockIntegrations.filter(i => i.category === category);

// Configure button
onClick={() => notify.info(`${integration.name} configuration coming soon`)

// Disconnect button
onClick={() => {
  notify.success(`${integration.name} disconnected`);
  // update local state
}
```

---

## SECTION 9 — Docs Page

File: `src/pages/docs/index.tsx`

```tsx
const docSections = [
  {
    title: 'Getting Started',
    icon: BookOpen,
    links: [
      { label: 'Quick Start Guide',  url: 'https://github.com/obsadmin' },
      { label: 'Installation',       url: 'https://github.com/obsadmin' },
      { label: 'Configuration',      url: 'https://github.com/obsadmin' },
    ],
  },
  {
    title: 'Features',
    icon: Layers,
    links: [
      { label: 'Logs Explorer',       url: '#' },
      { label: 'Metrics & Infra',     url: '#' },
      { label: 'Traces & APM',        url: '#' },
      { label: 'Alerts & Incidents',  url: '#' },
      { label: 'Synthetics',          url: '#' },
    ],
  },
  {
    title: 'API Reference',
    icon: Code2,
    links: [
      { label: 'REST API',         url: '#' },
      { label: 'Query Language',   url: '#' },
      { label: 'Webhooks',         url: '#' },
    ],
  },
  {
    title: 'Community',
    icon: Users,
    links: [
      { label: 'GitHub',           url: 'https://github.com/obsadmin' },
      { label: 'Discord',          url: '#' },
      { label: 'Contributing',     url: 'https://github.com/obsadmin' },
      { label: 'Changelog',        url: '#' },
    ],
  },
  {
    title: 'Self-Hosting',
    icon: Server,
    links: [
      { label: 'Docker Setup',     url: '#' },
      { label: 'Kubernetes',       url: '#' },
      { label: 'Environment Vars', url: '#' },
    ],
  },
  {
    title: 'Integrations',
    icon: Plug,
    links: [
      { label: 'Prometheus',       url: '#' },
      { label: 'Loki',             url: '#' },
      { label: 'Tempo',            url: '#' },
      { label: 'Elasticsearch',    url: '#' },
    ],
  },
];
```

Cards: 3 per row, each card has icon + title + link list with ExternalLink icons.

---

## SECTION 10 — Demo Data Page Wiring

File: `src/pages/demo/index.tsx`

Each dataset row has a real "Reload" handler:

```typescript
const datasets = [
  {
    id: 'metrics',
    icon: Activity,
    title: 'Infrastructure Metrics',
    description: '11 hosts, 847 containers, 60min of metrics data',
    loaded: true,
    onReload: () => {
      notify.success('Infrastructure metrics reloaded');
    },
    onClear: () => {
      notify.info('Infrastructure metrics cleared');
    },
  },
  // ... same for logs, traces, alerts, synthetics
];

// "Load All Demo Data" button
onClick={() => {
  notify.loading('Loading all demo data...');
  setTimeout(() => {
    notify.success('All demo data loaded successfully');
  }, 1800);
}
```

---

## Prompt for Deepseek

```
Read all files in docs/design-system/.
Now read docs/design-system/phase7e-polish.md.

Apply ALL sections in order:

SECTION 1 — Dynamic breadcrumbs using useLocation()
SECTION 2 — Notification bell popover with mock data, unread badge
SECTION 3 — Global search modal, ⌘K shortcut, keyboard navigation
SECTION 4 — PageSkeleton component, apply to 8 pages with 1200ms delay
SECTION 5 — EmptyState component, apply to 7 locations
SECTION 6 — 404 page + wildcard route in App.tsx
SECTION 7 — ErrorBoundary class component, wrap all page routes
SECTION 8 — Wire all broken buttons across all pages
SECTION 9 — Docs page with 6 card sections
SECTION 10 — Demo Data page with working reload/clear handlers

CRITICAL:
- Global search ⌘K must work on ALL pages via AppShell listener
- Notification bell unread count must show as red Badge
- Clicking notification row marks it read AND navigates
- PageSkeleton must match the layout it's replacing
- EmptyState icon background uses alpha(primary.main, 0.08)
- 404 page has NO sidebar/topbar — standalone full screen
- ErrorBoundary wraps each route individually not the whole app
- Broken button fixes must use existing state patterns per page
- Demo Data reload shows loading toast then success after 1800ms
- All imports from lucide-react only — no other icon library
```
