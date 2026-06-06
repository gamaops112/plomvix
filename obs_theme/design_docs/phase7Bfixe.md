# Phase 7B Fixes — Settings, Profile, Density, Timezones
## obsAdmin

---

## Install Dependencies

```bash
npm install @vvo/tzdb
```

---

## Fix 1 — Timezone Selector with @vvo/tzdb

File: `src/pages/settings/components/TimezoneSelect.tsx`

```typescript
import { getTimeZones } from '@vvo/tzdb';
import { useMemo } from 'react';
import { Autocomplete, TextField, Box, Typography } from '@mui/material';

const TimezoneSelect = ({ value, onChange }) => {
  const timezones = useMemo(() => {
    return getTimeZones().map(tz => ({
      label: `${tz.currentTimeFormat} — ${tz.name}`,
      value: tz.name,
      offset: tz.currentTimeOffsetInMinutes,
      region: tz.continentName,
      abbreviation: tz.abbreviation,
    }));
  }, []);

  const selected = timezones.find(tz => tz.value === value) ?? null;

  return (
    <Autocomplete
      options={timezones}
      groupBy={(option) => option.region}
      getOptionLabel={(option) => option.label}
      value={selected}
      onChange={(_, newValue) => onChange(newValue?.value ?? 'UTC')}
      renderInput={(params) => (
        <TextField
          {...params}
          size="small"
          placeholder="Search timezone..."
        />
      )}
      renderOption={(props, option) => (
        <Box component="li" {...props} sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
          <Typography variant="body2">{option.value}</Typography>
          <Typography variant="caption" sx={{ color: 'text.disabled' }}>
            {option.abbreviation}  {option.offset >= 0 ? '+' : ''}{Math.floor(option.offset / 60)}:{String(Math.abs(option.offset % 60)).padStart(2, '0')}
          </Typography>
        </Box>
      )}
      sx={{ width: '100%', maxWidth: 400 }}
      ListboxProps={{ style: { maxHeight: 300 } }}
    />
  );
};

export default TimezoneSelect;
```

Replace the hardcoded UTC `<Select>` in `GeneralTab.tsx` with this component.

Store selected timezone in settings store (see Fix 3 below).

---

## Fix 2 — Separate Profile Page

### New file: `src/pages/profile/index.tsx`

Route: `/profile`

### Layout
```
← Back

Profile                          [Save Changes]

┌─────────────────────────────────────────────┐
│ PERSONAL INFO                               │
│                                             │
│ Full Name                                   │
│ [Demo User_____________________]            │
│                                             │
│ Email Address                               │
│ [demo@obsadmin.io______________]            │
│ (email cannot be changed in demo mode)      │
│                                             │
│ Timezone                                    │
│ [TimezoneSelect component]                  │
│                                             │
│ Role                                        │
│ [Admin] ← read only chip, not editable      │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│ CHANGE PASSWORD                             │
│                                             │
│ Current Password                            │
│ [____________________] [👁]                 │
│                                             │
│ New Password                                │
│ [____________________] [👁]                 │
│                                             │
│ Confirm New Password                        │
│ [____________________] [👁]                 │
│                                             │
│ Password requirements:                      │
│ ● Min 8 characters                         │
│ ● At least one uppercase letter            │
│ ● At least one number                      │
│ ● At least one special character           │
│                                             │
│ [Update Password]                           │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│ NOTIFICATION PREFERENCES                    │
│                                             │
│ Email notifications                         │
│ [✓] Alert fired          [✓] Alert resolved │
│ [✓] Incident created     [ ] Deployment     │
│ [ ] Weekly digest                           │
│                                             │
│ In-app notifications                        │
│ [✓] All alerts           [✓] Mentions       │
│ [✓] Incidents            [ ] System updates │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│ API KEYS                                    │
│                                             │
│ [+ Generate New Key]                        │
│                                             │
│ Admin Key      2d ago  1h ago   [Copy][Revoke]│
│ Read-only Key  5d ago  3h ago   [Copy][Revoke]│
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│ DANGER ZONE                                 │
│                                             │
│ Delete Account                              │
│ This action cannot be undone.               │
│                                 [Delete Account]│
│ (red outlined button, disabled in demo)     │
└─────────────────────────────────────────────┘
```

### Form validation with Zod
```typescript
const profileSchema = z.object({
  name: z.string().min(2, 'Name must be at least 2 characters'),
  email: z.string().email('Invalid email'),
  timezone: z.string(),
});

const passwordSchema = z.object({
  currentPassword: z.string().min(1, 'Required'),
  newPassword: z.string()
    .min(8, 'Min 8 characters')
    .regex(/[A-Z]/, 'Must contain uppercase')
    .regex(/[0-9]/, 'Must contain a number')
    .regex(/[^A-Za-z0-9]/, 'Must contain special character'),
  confirmPassword: z.string(),
}).refine(data => data.newPassword === data.confirmPassword, {
  message: "Passwords don't match",
  path: ['confirmPassword'],
});
```

### Demo mode behavior
- Email field: disabled + helper text "Email cannot be changed in demo mode"
- Password update: shows `notify.info('Password changes not available in demo mode')`
- Delete account: button disabled + tooltip "Not available in demo mode"
- Save profile: shows `notify.success('Profile updated')`

### Add route to App.tsx
```tsx
<Route path="/profile" element={<AuthGuard><ProfilePage /></AuthGuard>} />
```

### Add to navConfig or just router — no sidebar entry needed for profile.

---

## Fix 3 — Settings Store

Create `src/store/settingsStore.ts`:

```typescript
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface SettingsState {
  // General
  timezone:       string;
  dateFormat:     string;
  defaultTimeRange: string;
  defaultEnvironment: string;
  rowsPerPage:    number;
  sendTelemetry:  boolean;

  // Appearance
  density:        'compact' | 'default' | 'comfortable';
  showSectionLabels: boolean;
  showIcons:      boolean;

  // Actions
  updateSettings: (settings: Partial<SettingsState>) => void;
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      timezone:           'UTC',
      dateFormat:         'YYYY-MM-DD',
      defaultTimeRange:   'Last 15 minutes',
      defaultEnvironment: 'production',
      rowsPerPage:        25,
      sendTelemetry:      false,
      density:            'default',
      showSectionLabels:  true,
      showIcons:          true,

      updateSettings: (newSettings) => set((state) => ({
        ...state,
        ...newSettings,
      })),
    }),
    { name: 'obsadmin-settings' }
  )
);
```

---

## Fix 4 — Wire Density to Table Row Heights

Density must apply immediately on change — no save needed.

### In `src/theme/index.ts` — export a getDensityTokens helper:

```typescript
export const getDensityTokens = (density: 'compact' | 'default' | 'comfortable') => ({
  tableRowHeight: density === 'compact' ? 28 : density === 'comfortable' ? 44 : 36,
  cardPadding:    density === 'compact' ? 12 : density === 'comfortable' ? 24 : 16,
  inputHeight:    density === 'compact' ? 28 : density === 'comfortable' ? 40 : 32,
});
```

### In every table component read density from store:

```typescript
import { useSettingsStore } from '../../store/settingsStore';
import { getDensityTokens } from '../../theme';

const { density } = useSettingsStore();
const { tableRowHeight } = getDensityTokens(density);

// Apply to MuiTableCell sx:
sx={{ height: tableRowHeight, padding: density === 'compact' ? '4px 12px' : '8px 12px' }}
```

Apply to these components:
- `HostsTable.tsx`
- `TracesTable.tsx`
- `LogsTable.tsx`
- `FiringAlertsTable.tsx`
- `AlertRulesTable.tsx`
- `IncidentsList.tsx`
- `UsersTable.tsx` (Members tab)

### In Appearance tab — radio changes fire immediately:
```typescript
const { density, updateSettings } = useSettingsStore();

<RadioGroup
  value={density}
  onChange={(e) => updateSettings({ density: e.target.value as any })}
>
  <FormControlLabel value="compact"     control={<Radio size="small" />} label="Compact" />
  <FormControlLabel value="default"     control={<Radio size="small" />} label="Default" />
  <FormControlLabel value="comfortable" control={<Radio size="small" />} label="Comfortable" />
</RadioGroup>
```

---

## Fix 5 — Wire Sidebar Settings

Sidebar section labels + icons toggles must work immediately.

### In `Sidebar.tsx`:
```typescript
import { useSettingsStore } from '../store/settingsStore';

const { showSectionLabels, showIcons } = useSettingsStore();

// Section label: only render if showSectionLabels is true
{showSectionLabels && !sidebarCollapsed && (
  <Typography variant="caption" sx={{ ... }}>
    {section.label}
  </Typography>
)}

// Icon: only render if showIcons is true
{showIcons && (
  <Icon size={16} />
)}
```

### In Appearance tab:
```typescript
const { showSectionLabels, showIcons, updateSettings } = useSettingsStore();

<FormControlLabel
  control={
    <Switch
      checked={showSectionLabels}
      onChange={(e) => updateSettings({ showSectionLabels: e.target.checked })}
      size="small"
    />
  }
  label="Show section labels"
/>
<FormControlLabel
  control={
    <Switch
      checked={showIcons}
      onChange={(e) => updateSettings({ showIcons: e.target.checked })}
      size="small"
    />
  }
  label="Show icons"
/>
```

---

## Fix 6 — Notification Channels Link in Settings

File: `src/pages/settings/tabs/NotificationsTab.tsx`

```typescript
import { useNavigate } from 'react-router-dom';

const navigate = useNavigate();

// WRONG — was linking to data sources
onClick={() => navigate('/settings?tab=datasources')}

// CORRECT — link to alerts notification channels tab
onClick={() => navigate('/alerts?tab=channels')}
```

Also update the link text to be clear:
```tsx
<Button
  variant="text"
  size="small"
  endIcon={<ExternalLink size={12} />}
  onClick={() => navigate('/alerts?tab=channels')}
>
  Manage notification channels in Alerts →
</Button>
```

---

## Fix 7 — User Avatar Menu Links

File: `src/layout/Topbar.tsx`

```typescript
// WRONG — both go to /settings
{ label: 'Profile',  onClick: () => navigate('/settings') }
{ label: 'Settings', onClick: () => navigate('/settings') }

// CORRECT — separate routes
{ label: 'Profile',  onClick: () => { navigate('/profile');  handleMenuClose(); } }
{ label: 'Settings', onClick: () => { navigate('/settings'); handleMenuClose(); } }
```

---

## Fix 8 — Read Timezone from Settings Store

Anywhere timestamps are displayed, use the stored timezone.

```typescript
import { useSettingsStore } from '../store/settingsStore';
import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc';
import timezone from 'dayjs/plugin/timezone';

dayjs.extend(utc);
dayjs.extend(timezone);

const { timezone } = useSettingsStore();

// Format timestamps using stored timezone
const formatTime = (isoString: string) =>
  dayjs(isoString).tz(timezone).format('HH:mm:ss.SSS');

const formatDateTime = (isoString: string) =>
  dayjs(isoString).tz(timezone).format('YYYY-MM-DD HH:mm:ss');
```

Apply to:
- `LogsTable.tsx` — log timestamps
- `TracesTable.tsx` — trace start times
- `FiringAlertsTable.tsx` — alert start times
- `IncidentsList.tsx` — incident times
- `TraceDetailPage.tsx` — span timestamps

Also install dayjs plugins:
```bash
npm install dayjs
```
(already installed — just add plugin imports)

---

## Prompt for Deepseek

```
Read all files in docs/design-system/.
Now read docs/design-system/phase7b-fixes.md.

Apply ALL fixes in this exact order:

1. Install @vvo/tzdb:
   npm install @vvo/tzdb

2. Create src/components/common/TimezoneSelect.tsx
   as specified in Fix 1

3. Create src/store/settingsStore.ts — Fix 3

4. Create src/pages/profile/index.tsx — Fix 2
   Full profile page with 5 sections as specified
   Add /profile route to App.tsx

5. Wire density to all 7 table components — Fix 4
   getDensityTokens helper in theme/index.ts
   Apply tableRowHeight to every table

6. Wire sidebar settings toggles — Fix 5
   showSectionLabels and showIcons from settingsStore

7. Fix notification channels link — Fix 6
   Navigate to /alerts?tab=channels not datasources

8. Fix user avatar menu links — Fix 7
   Profile → /profile, Settings → /settings

9. Wire timezone to timestamp formatting — Fix 8
   Use dayjs.tz() in all table components

10. Replace hardcoded UTC Select in GeneralTab.tsx
    with TimezoneSelect component

CRITICAL:
- settingsStore must use persist middleware
- Density and sidebar toggles must apply IMMEDIATELY
  without needing to click Save
- Profile page password form: all 3 fields need
  show/hide toggle independently
- Demo mode: disable email + delete account,
  show notify.info on password change attempt
- TimezoneSelect must be grouped by continent/region
```
