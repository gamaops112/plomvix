# Component Overrides

Add these inside the `components` key of `createTheme()` in `src/theme/index.ts`.
Each section below is a drop-in addition to the components object.

---

## Button

```typescript
MuiButton: {
  defaultProps: {
    disableElevation: true,
    disableRipple: false,
    size: 'small',
  },
  styleOverrides: {
    root: {
      textTransform: 'none',
      fontWeight: 500,
      fontSize: '13px',
      borderRadius: '4px',
      padding: '5px 12px',
      minHeight: '30px',
    },
    contained: {
      background: '#06b6d4',
      color: '#0f1117',
      '&:hover': { background: '#0891b2' },
    },
    outlined: {
      borderColor: '#2a3147',
      color: '#e8eaf0',
      '&:hover': {
        background: '#1e2438',
        borderColor: '#3d4663',
      },
    },
    text: {
      color: '#8b93a8',
      '&:hover': { background: '#1e2438', color: '#e8eaf0' },
    },
  },
},
```

---

## IconButton

```typescript
MuiIconButton: {
  defaultProps: { size: 'small' },
  styleOverrides: {
    root: {
      borderRadius: '4px',
      color: '#8b93a8',
      '&:hover': { background: '#1e2438', color: '#e8eaf0' },
    },
  },
},
```

---

## Input / TextField

```typescript
MuiOutlinedInput: {
  styleOverrides: {
    root: {
      fontSize: '13px',
      borderRadius: '4px',
      background: '#161b27',
      '& .MuiOutlinedInput-notchedOutline': {
        borderColor: '#2a3147',
      },
      '&:hover .MuiOutlinedInput-notchedOutline': {
        borderColor: '#3d4663',
      },
      '&.Mui-focused .MuiOutlinedInput-notchedOutline': {
        borderColor: '#06b6d4',
        borderWidth: '1px',
        boxShadow: '0 0 0 2px rgba(6,182,212,0.15)',
      },
    },
    input: {
      padding: '6px 10px',
      height: '20px',
      color: '#e8eaf0',
      '&::placeholder': { color: '#4d566b', opacity: 1 },
    },
  },
},
MuiInputLabel: {
  styleOverrides: {
    root: {
      fontSize: '13px',
      color: '#8b93a8',
      '&.Mui-focused': { color: '#06b6d4' },
    },
  },
},
```

---

## Select

```typescript
MuiSelect: {
  defaultProps: { size: 'small' },
  styleOverrides: {
    icon: { color: '#8b93a8' },
  },
},
MuiMenu: {
  styleOverrides: {
    paper: {
      background: '#1c2333',
      border: '1px solid #2a3147',
      borderRadius: '4px',
      boxShadow: 'none',
    },
  },
},
MuiMenuItem: {
  styleOverrides: {
    root: {
      fontSize: '13px',
      minHeight: '32px',
      padding: '6px 12px',
      color: '#e8eaf0',
      '&:hover': { background: '#1e2438' },
      '&.Mui-selected': {
        background: '#1a2540',
        '&:hover': { background: '#1e2438' },
      },
    },
  },
},
```

---

## Table

```typescript
MuiTable: {
  styleOverrides: {
    root: { borderCollapse: 'collapse' },
  },
},
MuiTableHead: {
  styleOverrides: {
    root: {
      '& .MuiTableCell-root': {
        background: '#161b27',
        borderBottom: '1px solid #2a3147',
        color: '#8b93a8',
        fontSize: '11px',
        fontWeight: 500,
        letterSpacing: '0.04em',
        textTransform: 'uppercase',
        padding: '8px 12px',
        whiteSpace: 'nowrap',
      },
    },
  },
},
MuiTableBody: {
  styleOverrides: {
    root: {
      '& .MuiTableRow-root': {
        '&:hover': { background: '#1e2438' },
        '&.Mui-selected': { background: '#1a2540' },
      },
    },
  },
},
MuiTableCell: {
  styleOverrides: {
    root: {
      fontSize: '13px',
      padding: '7px 12px',
      borderBottom: '1px solid #1f2535',
      color: '#e8eaf0',
      height: '36px',
    },
    body: {
      color: '#e8eaf0',
    },
  },
},
```

---

## Card

```typescript
MuiCard: {
  defaultProps: { elevation: 0 },
  styleOverrides: {
    root: {
      background: '#161b27',
      border: '1px solid #1f2535',
      borderRadius: '4px',
    },
  },
},
MuiCardContent: {
  styleOverrides: {
    root: {
      padding: '16px',
      '&:last-child': { paddingBottom: '16px' },
    },
  },
},
MuiCardHeader: {
  styleOverrides: {
    root: {
      padding: '12px 16px',
      borderBottom: '1px solid #1f2535',
    },
    title: {
      fontSize: '13px',
      fontWeight: 600,
      color: '#e8eaf0',
    },
    subheader: {
      fontSize: '12px',
      color: '#8b93a8',
    },
  },
},
```

---

## Chip / Badge (Status indicators)

```typescript
MuiChip: {
  styleOverrides: {
    root: {
      borderRadius: '3px',
      fontSize: '11px',
      fontWeight: 500,
      height: '20px',
    },
    filled: {
      '&.MuiChip-colorSuccess': {
        background: 'rgba(16,185,129,0.12)',
        color: '#10b981',
      },
      '&.MuiChip-colorWarning': {
        background: 'rgba(245,158,11,0.12)',
        color: '#f59e0b',
      },
      '&.MuiChip-colorError': {
        background: 'rgba(239,68,68,0.12)',
        color: '#ef4444',
      },
      '&.MuiChip-colorInfo': {
        background: 'rgba(6,182,212,0.12)',
        color: '#06b6d4',
      },
      '&.MuiChip-colorDefault': {
        background: '#1e2438',
        color: '#8b93a8',
      },
    },
  },
},
```

---

## Tabs

```typescript
MuiTabs: {
  styleOverrides: {
    root: {
      minHeight: '36px',
      borderBottom: '1px solid #1f2535',
    },
    indicator: {
      background: '#06b6d4',
      height: '2px',
    },
  },
},
MuiTab: {
  styleOverrides: {
    root: {
      minHeight: '36px',
      fontSize: '13px',
      fontWeight: 400,
      textTransform: 'none',
      color: '#8b93a8',
      padding: '0 16px',
      '&.Mui-selected': {
        color: '#e8eaf0',
        fontWeight: 500,
      },
      '&:hover': { color: '#e8eaf0', background: '#1e2438' },
    },
  },
},
```

---

## Tooltip

```typescript
MuiTooltip: {
  styleOverrides: {
    tooltip: {
      background: '#1c2333',
      border: '1px solid #2a3147',
      color: '#e8eaf0',
      fontSize: '12px',
      borderRadius: '4px',
      padding: '6px 10px',
      boxShadow: 'none',
    },
    arrow: {
      color: '#1c2333',
    },
  },
},
```

---

## Divider

```typescript
MuiDivider: {
  styleOverrides: {
    root: {
      borderColor: '#1f2535',
    },
  },
},
```

---

## Drawer (Sidebar)

```typescript
MuiDrawer: {
  styleOverrides: {
    paper: {
      background: '#161b27',
      border: 'none',
      borderRight: '1px solid #1f2535',
    },
  },
},
```

---

## AppBar (Topbar)

```typescript
MuiAppBar: {
  defaultProps: { elevation: 0 },
  styleOverrides: {
    root: {
      background: '#161b27',
      borderBottom: '1px solid #1f2535',
      color: '#e8eaf0',
    },
  },
},
```

---

## Breadcrumbs

```typescript
MuiBreadcrumbs: {
  styleOverrides: {
    root: { fontSize: '13px' },
    separator: { color: '#4d566b' },
    ol: { flexWrap: 'nowrap' },
  },
},
```

---

## Linear Progress (used for metric bars)

```typescript
MuiLinearProgress: {
  styleOverrides: {
    root: {
      height: '3px',
      borderRadius: '2px',
      background: '#2a3147',
    },
    bar: {
      borderRadius: '2px',
    },
  },
},
```

---

## Skeleton (loading states)

```typescript
MuiSkeleton: {
  styleOverrides: {
    root: {
      background: '#1c2333',
      '&::after': {
        background: 'linear-gradient(90deg, transparent, rgba(255,255,255,0.03), transparent)',
      },
    },
  },
},
```
