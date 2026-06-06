# Usage Rules — Design System

This document tells contributors WHEN and HOW to use design tokens and components.
Read this before building any new page or component.

---

## The Core Rule

> **Color carries meaning. Never use semantic colors decoratively.**

- `#10b981` green = healthy / success only
- `#f59e0b` amber = warning / degraded only  
- `#ef4444` red = error / critical only
- `#06b6d4` cyan = accent / interactive only

If you want to color something for visual variety, use `text.secondary` or a neutral gray. Never repurpose status colors.

---

## When to use each background level

| Situation | Background token |
|---|---|
| The page root | `bg.page` → `#0f1117` |
| Any card, panel, sidebar | `bg.surface` → `#161b27` |
| Dropdown, tooltip, modal | `bg.elevated` → `#1c2333` |
| Hover state on a row or item | `bg.hover` → `#1e2438` |
| Selected/active row or nav item | `bg.selected` → `#1a2540` |

Never use `background: paper` from MUI directly — use the named token.

---

## Status badge rules

Use `<Chip>` with the correct color prop:

```tsx
<Chip label="Healthy"  color="success" size="small" />
<Chip label="Degraded" color="warning" size="small" />
<Chip label="Down"     color="error"   size="small" />
<Chip label="Unknown"  color="default" size="small" />
```

Never hardcode status colors inline — always go through the Chip color system.

---

## Typography rules

| Content type | Variant |
|---|---|
| Page title | `h2` |
| Section heading inside a card | `h4` |
| Table column header | `caption` (auto uppercase) |
| Body content, descriptions | `body2` |
| Log lines, trace IDs, hashes | `mono` |
| Big metric number | `metric` |
| Smaller metric | `metricSm` |
| Muted label under a metric | `caption2` |

---

## Chart rules

Always use chart colors in order: `chart.1` → `chart.2` → `chart.3` etc.
Never pick arbitrary colors for chart series.

For single-series charts (most metric charts) always use `chart.1` (`#06b6d4`).

For status-encoded charts (error rate, availability):
- Use `status.success` for the healthy line
- Use `status.error` for the error line

Grid lines: always `chart.grid` (`#1f2535`).
Axis labels: always `chart.axis` (`#4d566b`).

---

## Observability-specific component mapping

| UI Element | Component |
|---|---|
| Service health indicator | `<Chip color="success/warning/error" />` |
| Metric tile / stat card | Card + `metric` typography variant |
| Log level badge | `<Chip>` with level-specific color |
| Trace span | Custom component — see Phase 5 |
| Alert severity | `<Chip color="error/warning/info" />` |
| Time range picker | Custom — wraps `<Select>` + `dayjs` |
| Query editor | Monaco Editor component |
| Large log table | TanStack Virtual + TanStack Table |
| Inline sparkline | ECharts instance, no axis, no tooltip |

---

## Log level colors

| Level | Color | Token |
|---|---|---|
| ERROR / FATAL | `#ef4444` | `status.error` |
| WARN | `#f59e0b` | `status.warning` |
| INFO | `#06b6d4` | `status.info` |
| DEBUG | `#8b93a8` | `text.secondary` |
| TRACE | `#4d566b` | `text.tertiary` |

---

## Do not

- Do not add `box-shadow` to cards or panels — elevation is expressed by bg color only
- Do not use `border-radius` above `6px` anywhere
- Do not use MUI's default blue (`#1976d2`) — it is overridden but watch for edge cases
- Do not use `fontWeight: 700` — max is `600`
- Do not use `fontSize` below `11px`
- Do not wrap every piece of content in a `<Card>` — use cards only for discrete data objects
- Do not use `padding` above `24px` inside cards

---

## Spacing reference (quick)

```
4px  — gaps between inline elements, icon padding
8px  — tight internal padding (chip, badge)
12px — default component internal padding
16px — card padding, form field gaps
24px — section gaps, page padding
32px — major layout gaps
```

Always use multiples of 4.

---

## File naming conventions

```
components/
  common/       → shared across all pages (StatusBadge, StatCard, TimeRangePicker)
  layout/       → Sidebar, Topbar, PageWrapper
  charts/       → chart wrapper components
  tables/       → table components with TanStack

pages/
  dashboard/
  logs/
  traces/
  metrics/
  alerts/
  settings/
```
