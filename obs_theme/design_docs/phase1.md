# Phase 1 — App Shell Layout Spec
## obsAdmin

This is the frame every page lives inside. Build this before any page content.

---

## Layout Structure

```
┌─────────────────────────────────────────────────┐
│                   TOPBAR (48px)                  │
├──────────────┬──────────────────────────────────┤
│              │                                   │
│   SIDEBAR    │         CONTENT AREA              │
│  (220/56px)  │         (flex: 1)                 │
│              │                                   │
│              │                                   │
└──────────────┴──────────────────────────────────┘
```

---

## File Structure to Create

```
src/
├── layout/
│   ├── AppShell.tsx          ← root layout wrapper
│   ├── Sidebar.tsx           ← collapsible sidebar
│   ├── Topbar.tsx            ← top navigation bar
│   └── navConfig.ts          ← all nav items defined here
├── pages/
│   ├── dashboard/
│   │   └── index.tsx         ← empty placeholder
│   ├── logs/
│   │   └── index.tsx
│   ├── traces/
│   │   └── index.tsx
│   ├── metrics/
│   │   └── index.tsx
│   ├── alerts/
│   │   └── index.tsx
│   ├── synthetics/
│   │   └── index.tsx
│   ├── apm/
│   │   └── index.tsx
│   ├── users/
│   │   └── index.tsx
│   └── settings/
│       └── index.tsx
└── App.tsx                   ← sets up router + AppShell
```

---

## Topbar Spec

### Dimensions
| Property | Value |
|---|---|
| Height | 48px |
| Background | `#161b27` |
| Border bottom | `1px solid #1f2535` |
| Padding | `0 16px` |
| Position | fixed, top 0, full width |
| z-index | 1100 |

### Left section
- **Logo / brand mark** — "obsAdmin" text in `#06b6d4`, font weight 600, 15px
- Small icon/logo mark to the left of text (use any simple SVG or Lucide icon like `Activity`)

### Center section — Search
- Full width search input, max-width `480px`, centered
- Placeholder: `Search services, logs, traces...`
- Background: `#0f1117`
- Border: `1px solid #2a3147`
- On focus: border `#06b6d4`, glow `0 0 0 2px rgba(6,182,212,0.12)`
- Height: `32px`
- Left icon: `Search` from lucide-react, size 14, color `#4d566b`
- Right hint: `⌘K` badge — small, color `#4d566b`, font size 11px

### Right section
- **Time range picker** — `<Select>` styled button showing "Last 15 minutes"
  - Options: Last 5m, Last 15m, Last 1h, Last 6h, Last 24h, Last 7d, Custom
- **Refresh button** — outlined, cyan, icon `RefreshCw` from lucide + "Refresh" text
- **Notifications bell** — `Bell` icon, badge count if alerts exist
- **User avatar** — circle, initials fallback, opens user menu on click

### Breadcrumbs (below main topbar row)
- Secondary row, height `32px`, background `#0f1117`, border bottom `1px solid #1f2535`
- Shows current path: `Observability > Infrastructure > Hosts`
- Font size 12px, color `#8b93a8`, active page color `#e8eaf0`
- Separator: `/` in `#4d566b`

---

## Sidebar Spec

### Dimensions
| State | Width |
|---|---|
| Expanded | 220px |
| Collapsed | 56px |

| Property | Value |
|---|---|
| Background | `#161b27` |
| Border right | `1px solid #1f2535` |
| Position | fixed, left 0, below topbar |
| Height | `calc(100vh - 48px)` |
| Transition | `width 200ms ease` |
| z-index | 1000 |

### Collapse toggle
- At the bottom of the sidebar
- Icon: `PanelLeftClose` / `PanelLeftOpen` from lucide-react
- On click: toggles between 220px and 56px
- State stored in Zustand — persists across page navigations

### Nav item anatomy
```
[icon]  [label]          ← expanded
[icon]                   ← collapsed (tooltip shows label on hover)
```

| Property | Value |
|---|---|
| Height | 34px |
| Padding | `0 12px` |
| Border radius | `4px` |
| Margin | `1px 8px` |
| Icon size | 16px |
| Label font size | 13px |
| Default color | `#8b93a8` |
| Hover background | `#1e2438` |
| Hover color | `#e8eaf0` |
| Active background | `#1a2540` |
| Active color | `#e8eaf0` |
| Active left border | `2px solid #06b6d4` |

### Section labels
- Font size: 10px, weight 500, uppercase, letter-spacing 0.08em
- Color: `#4d566b`
- Padding: `16px 20px 4px`
- Hidden when sidebar is collapsed

---

## Nav Config — `navConfig.ts`

```typescript
import {
  LayoutDashboard, FileText, GitBranch, Activity,
  Bell, Settings, FlaskConical, Cpu, Users,
  BookOpen, Layers, Radio, ShieldAlert
} from 'lucide-react';

export const navSections = [
  {
    label: 'Overview',
    items: [
      { label: 'Dashboard',   icon: LayoutDashboard, path: '/' },
    ],
  },
  {
    label: 'Observe',
    items: [
      { label: 'Logs',        icon: FileText,        path: '/logs' },
      { label: 'Traces',      icon: GitBranch,       path: '/traces' },
      { label: 'Metrics',     icon: Activity,        path: '/metrics' },
      { label: 'APM',         icon: Cpu,             path: '/apm' },
    ],
  },
  {
    label: 'Synthetics',
    items: [
      { label: 'Monitors',    icon: Radio,           path: '/synthetics' },
    ],
  },
  {
    label: 'Alerting',
    items: [
      { label: 'Alerts',      icon: Bell,            path: '/alerts' },
      { label: 'Incidents',   icon: ShieldAlert,     path: '/incidents' },
    ],
  },
  {
    label: 'Platform',
    items: [
      { label: 'Users',       icon: Users,           path: '/users' },
      { label: 'Integrations',icon: Layers,          path: '/integrations' },
      { label: 'Settings',    icon: Settings,        path: '/settings' },
    ],
  },
  {
    label: 'Developer',
    items: [
      { label: 'Demo Data',   icon: FlaskConical,    path: '/demo' },
      { label: 'Docs',        icon: BookOpen,        path: '/docs' },
    ],
  },
];
```

---

## Page Placeholders

Every page in `src/pages/*/index.tsx` should render this for now:

```tsx
import { Box, Typography } from '@mui/material';

export default function PageName() {
  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h2">Page Name</Typography>
      <Typography variant="body2" sx={{ color: 'text.secondary', mt: 1 }}>
        Coming in Phase X
      </Typography>
    </Box>
  );
}
```

---

## Router Setup — `App.tsx`

```tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import AppShell from './layout/AppShell';

// import all page components

export default function App() {
  return (
    <BrowserRouter>
      <AppShell>
        <Routes>
          <Route path="/"             element={<Dashboard />} />
          <Route path="/logs"         element={<Logs />} />
          <Route path="/traces"       element={<Traces />} />
          <Route path="/metrics"      element={<Metrics />} />
          <Route path="/apm"          element={<APM />} />
          <Route path="/synthetics"   element={<Synthetics />} />
          <Route path="/alerts"       element={<Alerts />} />
          <Route path="/incidents"    element={<Incidents />} />
          <Route path="/users"        element={<Users />} />
          <Route path="/integrations" element={<Integrations />} />
          <Route path="/settings"     element={<Settings />} />
          <Route path="/demo"         element={<Demo />} />
        </Routes>
      </AppShell>
    </BrowserRouter>
  );
}
```

---

## Zustand Store — sidebar state

```typescript
// src/store/uiStore.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface UIState {
  sidebarCollapsed: boolean;
  toggleSidebar: () => void;
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      toggleSidebar: () =>
        set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),
    }),
    { name: 'obsadmin-ui' }
  )
);
```

---

## Content Area

```tsx
// inside AppShell.tsx
<Box sx={{
  marginLeft: sidebarCollapsed ? '56px' : '220px',
  marginTop: '80px',        // 48px topbar + 32px breadcrumb row
  transition: 'margin-left 200ms ease',
  minHeight: 'calc(100vh - 80px)',
  background: '#0f1117',
  overflow: 'auto',
}}>
  {children}
</Box>
```

---

## Modals to build in Phase 1

### User Management Modal
Triggered from `/users` page and from user avatar menu.

```
┌─────────────────────────────────┐
│ Manage Users              [x]   │
├─────────────────────────────────┤
│ [+ Invite User]    [Search...]  │
│                                 │
│ Name     Role    Status  Action │
│ ─────────────────────────────── │
│ John D.  Admin   ● Active  [⋮] │
│ Jane S.  Viewer  ● Active  [⋮] │
└─────────────────────────────────┘
```

| Property | Value |
|---|---|
| Width | 600px |
| Background | `#161b27` |
| Border | `1px solid #2a3147` |
| Border radius | `6px` |
| Header padding | `16px 20px` |
| Body padding | `20px` |

### Demo Data Modal
Triggered from `/demo` page.
Lets user load mock datasets for: Logs, Metrics, Traces, Alerts.
Each dataset has a toggle + description + "Load" button.

---

## Prompt for Deepseek

```
Read docs/design-system/theme-tokens.md, mui-theme.md, 
component-overrides.md, and usage-rules.md.

Now read docs/design-system/phase1-shell.md.

Build exactly what is specified:
1. src/store/uiStore.ts — Zustand sidebar state with persist
2. src/layout/navConfig.ts — nav sections as specified
3. src/layout/Topbar.tsx — 48px bar + breadcrumb row below it
4. src/layout/Sidebar.tsx — collapsible, grouped nav, active state
5. src/layout/AppShell.tsx — composes topbar + sidebar + content area
6. All page placeholders in src/pages/
7. App.tsx with React Router setup

Use only MUI components + lucide-react icons.
Follow all token values from theme-tokens.md exactly.
Do not add any extra styling or creative decisions.
```
