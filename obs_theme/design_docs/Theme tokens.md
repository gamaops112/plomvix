# Theme Tokens — Observability Platform

## Stack Context
- React + TypeScript + Vite
- MUI (Material UI) v5+
- Dark-first theme
- Reference: Elastic, Datadog, New Relic visual style

---

## Color Tokens

### Backgrounds
| Token | Value | Usage |
|---|---|---|
| `bg.page` | `#0f1117` | App root background |
| `bg.surface` | `#161b27` | Cards, panels, sidebar |
| `bg.elevated` | `#1c2333` | Dropdowns, tooltips, modals |
| `bg.hover` | `#1e2438` | Row hover, menu item hover |
| `bg.selected` | `#1a2540` | Active nav item, selected row |
| `bg.overlay` | `rgba(0,0,0,0.6)` | Modal backdrop |

### Borders
| Token | Value | Usage |
|---|---|---|
| `border.subtle` | `#1f2535` | Card edges, dividers |
| `border.default` | `#2a3147` | Input borders, table borders |
| `border.strong` | `#3d4663` | Focused inputs, active elements |

### Text
| Token | Value | Usage |
|---|---|---|
| `text.primary` | `#e8eaf0` | Main content, headings |
| `text.secondary` | `#8b93a8` | Labels, metadata, captions |
| `text.tertiary` | `#4d566b` | Placeholders, disabled |
| `text.inverse` | `#0f1117` | Text on light/accent backgrounds |

### Accent — Cyan
| Token | Value | Usage |
|---|---|---|
| `accent.default` | `#06b6d4` | Primary buttons, links, active indicators |
| `accent.hover` | `#0891b2` | Button hover |
| `accent.subtle` | `#06b6d415` | Accent background tints |
| `accent.border` | `#06b6d440` | Accent bordered elements |

### Semantic — Status Colors
These are used ONLY for status meaning. Never decorative.

| Token | Value | Meaning |
|---|---|---|
| `status.success` | `#10b981` | Healthy, ok, passing |
| `status.success.bg` | `#10b98115` | Success background tint |
| `status.warning` | `#f59e0b` | Degraded, slow, at risk |
| `status.warning.bg` | `#f59e0b15` | Warning background tint |
| `status.error` | `#ef4444` | Down, critical, failing |
| `status.error.bg` | `#ef444415` | Error background tint |
| `status.info` | `#06b6d4` | Informational, neutral alert |
| `status.info.bg` | `#06b6d415` | Info background tint |
| `status.muted` | `#4d566b` | Unknown, no data, inactive |

### Chart Colors
Ordered by priority — first color gets the most important series.
| Token | Value | Usage |
|---|---|---|
| `chart.1` | `#06b6d4` | Primary series |
| `chart.2` | `#8b5cf6` | Secondary series |
| `chart.3` | `#f59e0b` | Tertiary series |
| `chart.4` | `#10b981` | Fourth series |
| `chart.5` | `#f97316` | Fifth series |
| `chart.6` | `#ec4899` | Sixth series |
| `chart.grid` | `#1f2535` | Chart grid lines |
| `chart.axis` | `#4d566b` | Axis labels, tick marks |

---

## Typography

### Font Families
```
UI font:   "Inter", system-ui, sans-serif
Mono font: "JetBrains Mono", "Fira Code", monospace
```
Mono is used for: log lines, trace IDs, query editors, metric values, timestamps.

### Type Scale
| Token | Size | Weight | Line Height | Usage |
|---|---|---|---|---|
| `type.h1` | 24px | 600 | 1.3 | Page titles |
| `type.h2` | 20px | 600 | 1.3 | Section headings |
| `type.h3` | 16px | 600 | 1.4 | Card headings |
| `type.h4` | 14px | 600 | 1.4 | Sub-section labels |
| `type.body1` | 14px | 400 | 1.6 | Main body content |
| `type.body2` | 13px | 400 | 1.5 | Secondary content |
| `type.caption` | 11px | 500 | 1.4 | Labels, badges, table headers |
| `type.mono` | 13px | 400 | 1.6 | Log lines, code, IDs |
| `type.metric` | 28px | 600 | 1.2 | Big numbers on stat cards |
| `type.metric.sm` | 20px | 600 | 1.2 | Smaller metric values |

---

## Spacing System
Base unit: `4px`

| Token | Value | Usage |
|---|---|---|
| `space.1` | 4px | Inline gaps, icon padding |
| `space.2` | 8px | Tight internal padding |
| `space.3` | 12px | Default component padding |
| `space.4` | 16px | Card padding, section gaps |
| `space.5` | 20px | Medium gaps |
| `space.6` | 24px | Large section spacing |
| `space.8` | 32px | Page section gaps |
| `space.10` | 40px | Major layout gaps |

---

## Shape & Radius
| Token | Value | Usage |
|---|---|---|
| `radius.sm` | 3px | Badges, chips, tags |
| `radius.md` | 4px | Buttons, inputs, cards |
| `radius.lg` | 6px | Modals, large panels |

Intentionally flat. No `8px+` radius — kills the MUI default bubbly look.

---

## Elevation
No box-shadows. Elevation is expressed through background color steps only.

| Level | Background | Usage |
|---|---|---|
| 0 | `#0f1117` | Page |
| 1 | `#161b27` | Cards, sidebar |
| 2 | `#1c2333` | Dropdowns, popovers |
| 3 | `#1e2438` | Tooltips, hover states |

Exception: focused inputs use `0 0 0 2px #06b6d440` — a tint glow, not elevation.

---

## Layout

### Sidebar
| Property | Value |
|---|---|
| Width (expanded) | 220px |
| Width (collapsed) | 56px |
| Background | `#161b27` |
| Border right | `1px solid #1f2535` |

### Topbar
| Property | Value |
|---|---|
| Height | 48px |
| Background | `#161b27` |
| Border bottom | `1px solid #1f2535` |

### Content area
| Property | Value |
|---|---|
| Padding | 24px |
| Max width | none (full width) |

### Table row height
| Density | Height |
|---|---|
| Default | 36px |
| Compact | 28px |
| Comfortable | 44px |

Default is compact-medium — matches Datadog density.
