# Phase 7D Fixes — Users Page
## obsAdmin

---

## Fix 1 — Members Tab Toolbar

Add above the table:

```tsx
<Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center' }}>
  <TextField
    size="small"
    placeholder="Search members..."
    InputProps={{ startAdornment: <Search size={14} /> }}
    sx={{ width: 260 }}
    value={search}
    onChange={(e) => setSearch(e.target.value)}
  />
  <Select
    size="small"
    value={roleFilter}
    onChange={(e) => setRoleFilter(e.target.value)}
    sx={{ width: 130 }}
  >
    <MenuItem value="all">All Roles</MenuItem>
    <MenuItem value="Admin">Admin</MenuItem>
    <MenuItem value="Editor">Editor</MenuItem>
    <MenuItem value="Viewer">Viewer</MenuItem>
  </Select>
  <Select
    size="small"
    value={statusFilter}
    onChange={(e) => setStatusFilter(e.target.value)}
    sx={{ width: 130 }}
  >
    <MenuItem value="all">All Status</MenuItem>
    <MenuItem value="active">Active</MenuItem>
    <MenuItem value="pending">Pending</MenuItem>
  </Select>
  <Box sx={{ flex: 1 }} />
  <Button
    variant="outlined"
    size="small"
    startIcon={<Mail size={14} />}
    onClick={() => setInviteModalOpen(true)}
  >
    Invite Member
  </Button>
  <Button
    variant="contained"
    size="small"
    startIcon={<UserPlus size={14} />}
    onClick={() => setCreateModalOpen(true)}
  >
    Add User
  </Button>
</Box>
```

Filter logic:
```typescript
const filteredUsers = mockUsers.filter(u => {
  const matchSearch = 
    u.name.toLowerCase().includes(search.toLowerCase()) ||
    u.email.toLowerCase().includes(search.toLowerCase());
  const matchRole   = roleFilter === 'all'   || u.role === roleFilter;
  const matchStatus = statusFilter === 'all' || u.status === statusFilter;
  return matchSearch && matchRole && matchStatus;
});
```

---

## Fix 2 — Role Chips Colors

```typescript
// Role chip color mapping
const roleColors: Record<string, 'primary' | 'secondary' | 'default'> = {
  Admin:  'primary',    // cyan
  Editor: 'secondary',  // purple
  Viewer: 'default',    // gray
};

<Chip
  label={user.role}
  color={roleColors[user.role]}
  size="small"
  sx={{ borderRadius: '3px', fontSize: '11px', fontWeight: 500 }}
/>
```

---

## Fix 3 — Inline Role Change in Table Row

Replace static role chip with an inline Select that appears on hover:

```tsx
const [editingRole, setEditingRole] = useState<string | null>(null);

// In table row Role cell:
<TableCell sx={{ width: 120 }}>
  {editingRole === user.id ? (
    <Select
      size="small"
      value={user.role}
      autoFocus
      onBlur={() => setEditingRole(null)}
      onChange={(e) => {
        handleRoleChange(user.id, e.target.value);
        setEditingRole(null);
        notify.success(`${user.name}'s role updated to ${e.target.value}`);
      }}
      sx={{ height: 28, fontSize: '12px' }}
    >
      <MenuItem value="Admin">Admin</MenuItem>
      <MenuItem value="Editor">Editor</MenuItem>
      <MenuItem value="Viewer">Viewer</MenuItem>
    </Select>
  ) : (
    <Box
      sx={{ display: 'flex', alignItems: 'center', gap: 0.5, cursor: 'pointer' }}
      onClick={() => user.status !== 'pending' && setEditingRole(user.id)}
    >
      <Chip
        label={user.role}
        color={roleColors[user.role]}
        size="small"
        sx={{ borderRadius: '3px', fontSize: '11px' }}
      />
      {user.status !== 'pending' && (
        <Pencil
          size={11}
          style={{ opacity: 0, transition: 'opacity 0.1s' }}
          className="role-edit-icon"
        />
      )}
    </Box>
  )}
</TableCell>

// Show pencil icon on row hover via CSS:
// '.MuiTableRow-root:hover .role-edit-icon': { opacity: 1 }
```

---

## Fix 4 — Actions Column

```tsx
<TableCell align="right" sx={{ width: 100 }}>
  <Box sx={{
    display: 'flex',
    gap: 0.5,
    justifyContent: 'flex-end',
    opacity: 0,
    transition: 'opacity 0.15s',
    '.MuiTableRow-root:hover &': { opacity: 1 },
  }}>
    <Tooltip title="Edit user">
      <IconButton
        size="small"
        onClick={(e) => { e.stopPropagation(); setEditUser(user); setEditModalOpen(true); }}
      >
        <Pencil size={13} />
      </IconButton>
    </Tooltip>
    {user.id !== 'u1' && ( // can't remove yourself
      <Tooltip title="Remove user">
        <IconButton
          size="small"
          onClick={(e) => { e.stopPropagation(); handleRemoveUser(user); }}
        >
          <UserMinus size={13} color="#ef4444" />
        </IconButton>
      </Tooltip>
    )}
  </Box>
</TableCell>
```

Remove user flow:
```tsx
// Confirmation dialog before removing
<Dialog open={confirmRemoveOpen} maxWidth="xs">
  <DialogTitle>Remove {userToRemove?.name}?</DialogTitle>
  <DialogContent>
    <Typography variant="body2" color="text.secondary">
      This will revoke their access to obsAdmin immediately.
    </Typography>
  </DialogContent>
  <DialogActions>
    <Button onClick={() => setConfirmRemoveOpen(false)}>Cancel</Button>
    <Button
      color="error"
      variant="contained"
      onClick={() => {
        handleConfirmRemove();
        notify.success(`${userToRemove?.name} removed`);
      }}
    >
      Remove
    </Button>
  </DialogActions>
</Dialog>
```

---

## Fix 5 — Create User Modal

`src/pages/users/components/CreateUserModal.tsx`

Width: `480px`

### Zod Schema
```typescript
const createUserSchema = z.object({
  name:            z.string().min(2, 'Name must be at least 2 characters'),
  email:           z.string().email('Invalid email address'),
  role:            z.enum(['Admin', 'Editor', 'Viewer']),
  password:        z.string()
    .min(8,        'Min 8 characters')
    .regex(/[A-Z]/, 'Must contain uppercase letter')
    .regex(/[0-9]/, 'Must contain a number')
    .regex(/[^A-Za-z0-9]/, 'Must contain special character'),
  confirmPassword: z.string(),
  sendWelcomeEmail: z.boolean(),
}).refine(
  data => data.password === data.confirmPassword,
  { message: "Passwords don't match", path: ['confirmPassword'] }
);
```

### Modal Layout
```
Add User                                    [×]
────────────────────────────────────────────────

PERSONAL INFO

Full Name
[_______________________________________]

Email Address
[_______________________________________]

Role
○ Admin    — Full access to all features
● Editor   — Can edit but not manage users
○ Viewer   — Read-only access

────────────────────────────────────────────────

SET PASSWORD

Password
[_______________________________________] [👁]

Confirm Password
[_______________________________________] [👁]

Password strength: [████████░░] Strong

Requirements:
● Min 8 characters
● Uppercase letter
● Number
● Special character

────────────────────────────────────────────────

OPTIONS

[✓] Send welcome email to user

────────────────────────────────────────────────

                    [Cancel]    [Create User]
```

### Password strength indicator
```typescript
const getPasswordStrength = (password: string) => {
  let score = 0;
  if (password.length >= 8)           score++;
  if (password.length >= 12)          score++;
  if (/[A-Z]/.test(password))         score++;
  if (/[0-9]/.test(password))         score++;
  if (/[^A-Za-z0-9]/.test(password))  score++;

  if (score <= 1) return { label: 'Weak',   color: '#ef4444', value: 20  };
  if (score <= 2) return { label: 'Fair',   color: '#f59e0b', value: 40  };
  if (score <= 3) return { label: 'Good',   color: '#f59e0b', value: 60  };
  if (score <= 4) return { label: 'Strong', color: '#10b981', value: 80  };
  return              { label: 'Very Strong', color: '#10b981', value: 100 };
};

// Render as MUI LinearProgress with dynamic color
<LinearProgress
  variant="determinate"
  value={strength.value}
  sx={{
    height: 4,
    borderRadius: 2,
    mt: 1,
    '& .MuiLinearProgress-bar': { background: strength.color },
  }}
/>
```

### Role selection — radio cards
```tsx
{['Admin', 'Editor', 'Viewer'].map(role => (
  <Box
    key={role}
    onClick={() => setValue('role', role)}
    sx={{
      border: `1px solid ${watch('role') === role
        ? theme.palette.primary.main
        : theme.palette.divider}`,
      background: watch('role') === role
        ? alpha(theme.palette.primary.main, 0.06)
        : 'transparent',
      borderRadius: '4px',
      p: 1.5,
      cursor: 'pointer',
      mb: 1,
      display: 'flex',
      alignItems: 'center',
      gap: 1.5,
    }}
  >
    <Radio
      checked={watch('role') === role}
      size="small"
      sx={{ p: 0 }}
    />
    <Box>
      <Typography variant="body2" fontWeight={500}>{role}</Typography>
      <Typography variant="caption" sx={{ color: 'text.secondary' }}>
        {role === 'Admin'  ? 'Full access to all features' :
         role === 'Editor' ? 'Can edit but not manage users or delete' :
                             'Read-only access to all data'}
      </Typography>
    </Box>
  </Box>
))}
```

On submit:
- Add user to local state
- `notify.success('User created successfully')`
- If sendWelcomeEmail: `notify.info('Welcome email sent to user')`
- Close modal

---

## Fix 6 — Invite Member Modal

`src/pages/users/components/InviteMemberModal.tsx`

Width: `420px` — simpler than Create User

```
Invite Team Member                         [×]
────────────────────────────────────────────

Email Address
[_______________________________________]

Role
[Editor ▼]

Personal message (optional)
[                                       ]
[  Hi! I'd like to invite you to...    ]
[                                       ]

────────────────────────────────────────

                [Cancel]  [Send Invite]
```

Schema:
```typescript
z.object({
  email:   z.string().email('Invalid email'),
  role:    z.enum(['Admin', 'Editor', 'Viewer']),
  message: z.string().optional(),
})
```

On submit:
- Add pending user to list
- `notify.success('Invite sent to ${email}')`

---

## Fix 7 — Roles Tab: 3-Column Layout

```tsx
<Grid container spacing={2}>
  {roles.map((role) => (
    <Grid item xs={4} key={role.name}>
      <Card sx={{
        height: '100%',
        border: role.name === 'Admin'
          ? `1px solid ${alpha(theme.palette.primary.main, 0.4)}`
          : `1px solid ${theme.palette.divider}`,
        background: role.name === 'Admin'
          ? alpha(theme.palette.primary.main, 0.04)
          : 'background.paper',
      }}>
        <CardContent>
          {/* Role name + badge */}
          <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
            <Typography variant="h4">{role.name}</Typography>
            {role.name === 'Admin' && (
              <Chip label="Full Access" color="primary" size="small" />
            )}
          </Box>

          <Typography variant="caption" sx={{ color: 'text.secondary', mb: 2, display: 'block' }}>
            {role.description}
          </Typography>

          {/* Permissions list */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, mb: 2 }}>
            {role.permissions.map((perm) => (
              <Box key={perm} sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
                <Check size={13} color={theme.palette.success.main} />
                <Typography variant="caption" sx={{ color: 'text.primary' }}>
                  {perm}
                </Typography>
              </Box>
            ))}
          </Box>

          {/* Member count */}
          <Box sx={{
            pt: 1.5,
            borderTop: `1px solid ${theme.palette.divider}`,
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}>
            <Typography variant="caption" sx={{ color: 'text.secondary' }}>
              {role.memberCount} member{role.memberCount !== 1 ? 's' : ''}
            </Typography>
          </Box>
        </CardContent>
      </Card>
    </Grid>
  ))}
</Grid>
```

Role data:
```typescript
const roles = [
  {
    name: 'Admin',
    description: 'Full access to all features and settings',
    memberCount: 1,
    permissions: [
      'View all data',
      'Create/edit alert rules',
      'Manage users',
      'Configure data sources',
      'Delete data',
      'Manage integrations',
    ],
  },
  {
    name: 'Editor',
    description: 'Can edit but not manage users or delete data',
    memberCount: 3,
    permissions: [
      'View all data',
      'Create/edit alert rules',
      'Configure dashboards',
      'Invite team members',
      'Acknowledge incidents',
    ],
  },
  {
    name: 'Viewer',
    description: 'Read-only access to all data',
    memberCount: 2,
    permissions: [
      'View dashboards',
      'View logs and traces',
      'View alerts',
      'View incidents',
      'Export data',
    ],
  },
];
```

---

## Fix 8 — Audit Log: More Data + Toolbar + Icons

### Expanded mock data (20 entries)
```typescript
export const mockAuditLog = [
  { time: '14:23', user: 'Admin User', action: 'created_alert_rule',  target: 'High Error Rate',   ip: '192.168.1.1' },
  { time: '14:21', user: 'Admin User', action: 'invited_user',        target: 'newdev@company.com', ip: '192.168.1.1' },
  { time: '14:18', user: 'John Doe',   action: 'acknowledged_alert',  target: 'Payment Timeout',   ip: '192.168.1.4' },
  { time: '14:15', user: 'John Doe',   action: 'silenced_alert',      target: 'CPU Spike',         ip: '192.168.1.4' },
  { time: '14:11', user: 'Admin User', action: 'created_incident',    target: 'INC-001',           ip: '192.168.1.1' },
  { time: '14:08', user: 'Jane Smith', action: 'updated_dashboard',   target: 'Main Dashboard',    ip: '192.168.1.7' },
  { time: '14:05', user: 'Admin User', action: 'changed_user_role',   target: 'Bob Chen → Viewer', ip: '192.168.1.1' },
  { time: '14:01', user: 'John Doe',   action: 'created_monitor',     target: 'API Health Check',  ip: '192.168.1.4' },
  { time: '13:58', user: 'Admin User', action: 'invited_user',        target: 'pending@company',   ip: '192.168.1.1' },
  { time: '13:55', user: 'Jane Smith', action: 'deleted_alert_rule',  target: 'Disk Space Low',    ip: '192.168.1.7' },
  { time: '13:50', user: 'Admin User', action: 'connected_datasource',target: 'Prometheus',        ip: '192.168.1.1' },
  { time: '13:45', user: 'Jane Smith', action: 'updated_dashboard',   target: 'Infra Overview',    ip: '192.168.1.7' },
  { time: '13:40', user: 'John Doe',   action: 'resolved_incident',   target: 'INC-003',           ip: '192.168.1.4' },
  { time: '13:35', user: 'Admin User', action: 'created_user',        target: 'alice@obsadmin.io', ip: '192.168.1.1' },
  { time: '13:30', user: 'Bob Chen',   action: 'exported_logs',       target: 'search-service',    ip: '192.168.1.9' },
  { time: '13:25', user: 'Jane Smith', action: 'created_alert_rule',  target: 'Queue Depth',       ip: '192.168.1.7' },
  { time: '13:20', user: 'Admin User', action: 'updated_settings',    target: 'General Settings',  ip: '192.168.1.1' },
  { time: '13:15', user: 'John Doe',   action: 'silenced_alert',      target: 'Memory Usage',      ip: '192.168.1.4' },
  { time: '13:10', user: 'Admin User', action: 'revoked_api_key',     target: 'Old CI Key',        ip: '192.168.1.1' },
  { time: '13:05', user: 'Jane Smith', action: 'created_monitor',     target: 'SSL Certificate',   ip: '192.168.1.7' },
];
```

### Action icons mapping
```typescript
const actionConfig: Record<string, { icon: LucideIcon; label: string; color: string }> = {
  created_alert_rule:   { icon: Bell,       label: 'Created alert rule',    color: '#06b6d4' },
  deleted_alert_rule:   { icon: BellOff,    label: 'Deleted alert rule',    color: '#ef4444' },
  invited_user:         { icon: UserPlus,   label: 'Invited user',          color: '#8b5cf6' },
  created_user:         { icon: UserPlus,   label: 'Created user',          color: '#8b5cf6' },
  changed_user_role:    { icon: Shield,     label: 'Changed user role',     color: '#f59e0b' },
  removed_user:         { icon: UserMinus,  label: 'Removed user',          color: '#ef4444' },
  silenced_alert:       { icon: VolumeX,    label: 'Silenced alert',        color: '#f59e0b' },
  acknowledged_alert:   { icon: Check,      label: 'Acknowledged alert',    color: '#10b981' },
  created_incident:     { icon: AlertTriangle, label: 'Created incident',   color: '#ef4444' },
  resolved_incident:    { icon: CheckCircle,label: 'Resolved incident',     color: '#10b981' },
  updated_dashboard:    { icon: LayoutDashboard, label: 'Updated dashboard',color: '#06b6d4' },
  created_monitor:      { icon: Radio,      label: 'Created monitor',       color: '#06b6d4' },
  connected_datasource: { icon: Database,   label: 'Connected data source', color: '#10b981' },
  updated_settings:     { icon: Settings,   label: 'Updated settings',      color: '#8b93a8' },
  exported_logs:        { icon: Download,   label: 'Exported logs',         color: '#8b93a8' },
  revoked_api_key:      { icon: Key,        label: 'Revoked API key',       color: '#ef4444' },
};
```

### Action column rendering
```tsx
<TableCell sx={{ width: 280 }}>
  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
    <Box sx={{
      p: 0.5,
      borderRadius: '4px',
      background: alpha(config.color, 0.1),
      display: 'flex',
      alignItems: 'center',
    }}>
      <config.icon size={13} color={config.color} />
    </Box>
    <Typography variant="body2">{config.label}</Typography>
  </Box>
</TableCell>
```

### Audit log toolbar
```tsx
<Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
  <TextField
    size="small"
    placeholder="Search audit log..."
    InputProps={{ startAdornment: <Search size={14} /> }}
    sx={{ width: 260 }}
    value={auditSearch}
    onChange={(e) => setAuditSearch(e.target.value)}
  />
  <Select size="small" value={userFilter} sx={{ width: 160 }}>
    <MenuItem value="all">All Users</MenuItem>
    {mockUsers.map(u => (
      <MenuItem key={u.id} value={u.name}>{u.name}</MenuItem>
    ))}
  </Select>
  <Select size="small" value={actionTypeFilter} sx={{ width: 180 }}>
    <MenuItem value="all">All Actions</MenuItem>
    <MenuItem value="alert">Alert actions</MenuItem>
    <MenuItem value="user">User actions</MenuItem>
    <MenuItem value="incident">Incident actions</MenuItem>
    <MenuItem value="settings">Settings changes</MenuItem>
  </Select>
  <Box sx={{ flex: 1 }} />
  <Button variant="outlined" size="small" startIcon={<Download size={13} />}>
    Export
  </Button>
</Box>
```

### Pagination
```tsx
<TablePagination
  component="div"
  count={filteredAuditLog.length}
  page={page}
  onPageChange={(_, newPage) => setPage(newPage)}
  rowsPerPage={rowsPerPage}
  onRowsPerPageChange={(e) => setRowsPerPage(parseInt(e.target.value))}
  rowsPerPageOptions={[10, 20, 50]}
/>
```

### IP column — de-emphasized
```tsx
<TableCell sx={{ width: 120, color: 'text.disabled' }}>
  <Typography variant="caption" sx={{ fontFamily: 'monospace', color: 'text.disabled' }}>
    {entry.ip}
  </Typography>
</TableCell>
```

---

## Prompt for Deepseek

```
Read all files in docs/design-system/.
Now read docs/design-system/phase7d-users-fixes.md.

Apply ALL fixes in order:

1. Add toolbar to Members tab — Fix 1
   Search + role filter + status filter
   Two buttons: Invite Member + Add User

2. Fix role chip colors — Fix 2
   Admin=primary(cyan), Editor=secondary(purple), Viewer=default

3. Inline role change in table — Fix 3
   Click role chip → shows Select dropdown
   onBlur or onChange closes and saves
   notify.success on role change

4. Add actions column — Fix 4
   Edit + Remove buttons, show on row hover
   Remove needs confirmation dialog

5. Build CreateUserModal.tsx — Fix 5
   React Hook Form + Zod
   Password strength indicator (LinearProgress)
   Role selection as radio cards
   Password show/hide on all password fields

6. Build InviteMemberModal.tsx — Fix 6
   Simple email + role + optional message
   Adds pending user to list

7. Fix Roles tab layout — Fix 7
   3 columns side by side (Grid xs=4)
   Check icons not links for permissions
   Admin card has accent border + chip

8. Expand audit log + toolbar + icons — Fix 8
   20 mock entries
   Action icons with colored backgrounds
   Search + user filter + action type filter
   TablePagination at bottom
   IP column in text.disabled color

CRITICAL:
- Inline role change: clicking outside (onBlur) cancels without saving
- Inline role change: selecting new value saves immediately + toast
- Cannot remove yourself (user.id === currentUser.id)
- CreateUserModal password fields each have independent show/hide toggle
- Roles tab cards must be equal height (use height: 100% on Card)
- All colors through theme tokens
```
