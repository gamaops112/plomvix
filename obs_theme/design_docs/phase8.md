# Phase 8 — Login & Auth Spec
## obsAdmin

---

## File Structure

```
src/
├── pages/
│   └── auth/
│       ├── LoginPage.tsx           ← main login screen
│       ├── ForgotPasswordPage.tsx  ← forgot password screen
│       └── components/
│           ├── LoginForm.tsx       ← email + password form
│           ├── SSOButtons.tsx      ← GitHub + Google SSO
│           └── DemoCredentials.tsx ← demo credentials display
├── store/
│   └── authStore.ts                ← Zustand auth state
├── lib/
│   └── auth.ts                     ← JWT helpers, cookie utils
└── components/
    └── guards/
        └── AuthGuard.tsx           ← route protection wrapper
```

---

## Demo Credentials

```
Email:    demo@obsadmin.io
Password: ObsAdmin@demo
JWT expiry: 24 hours
Cookie:   obsadmin_token (httpOnly simulation via js-cookie)
```

---

## Auth Store

`src/store/authStore.ts`

```typescript
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface User {
  id: string;
  name: string;
  email: string;
  role: 'admin' | 'editor' | 'viewer';
  avatar: string;
}

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<{ success: boolean; error?: string }>;
  loginDemo: () => void;
  logout: () => void;
}

// Demo user definition
const DEMO_USER: User = {
  id: 'demo_user',
  name: 'Demo User',
  email: 'demo@obsadmin.io',
  role: 'admin',
  avatar: 'DU',
};

// Fake JWT — base64 encoded payload, expires in 24h
const generateDemoToken = () => {
  const payload = {
    sub: 'demo_user',
    email: 'demo@obsadmin.io',
    role: 'admin',
    iat: Math.floor(Date.now() / 1000),
    exp: Math.floor(Date.now() / 1000) + 86400, // 24 hours
  };
  return btoa(JSON.stringify(payload));
};

const isTokenExpired = (token: string): boolean => {
  try {
    const payload = JSON.parse(atob(token));
    return payload.exp < Math.floor(Date.now() / 1000);
  } catch {
    return true;
  }
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      token: null,
      isAuthenticated: false,

      login: async (email: string, password: string) => {
        // Simulate network delay
        await new Promise(resolve => setTimeout(resolve, 800));

        if (
          email === 'demo@obsadmin.io' &&
          password === 'ObsAdmin@demo'
        ) {
          const token = generateDemoToken();
          set({ user: DEMO_USER, token, isAuthenticated: true });
          // Store in cookie simulation
          document.cookie = `obsadmin_token=${token}; max-age=86400; path=/; SameSite=Strict`;
          return { success: true };
        }

        return { success: false, error: 'Invalid email or password' };
      },

      loginDemo: () => {
        const token = generateDemoToken();
        set({ user: DEMO_USER, token, isAuthenticated: true });
        document.cookie = `obsadmin_token=${token}; max-age=86400; path=/; SameSite=Strict`;
      },

      logout: () => {
        set({ user: null, token: null, isAuthenticated: false });
        // Clear cookie
        document.cookie = 'obsadmin_token=; max-age=0; path=/;';
      },
    }),
    {
      name: 'obsadmin-auth',
      // On rehydration, check if token is expired
      onRehydrateStorage: () => (state) => {
        if (state?.token && isTokenExpired(state.token)) {
          state.user = null;
          state.token = null;
          state.isAuthenticated = false;
        }
      },
    }
  )
);
```

---

## AuthGuard Component

`src/components/guards/AuthGuard.tsx`

```typescript
import { Navigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '../../store/authStore';

export default function AuthGuard({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuthStore();
  const location = useLocation();

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}
```

Wrap entire app in `App.tsx`:
```tsx
<Route path="/login"           element={<LoginPage />} />
<Route path="/forgot-password" element={<ForgotPasswordPage />} />
<Route path="/*" element={
  <AuthGuard>
    <AppShell>
      <Routes>
        {/* all existing routes */}
      </Routes>
    </AppShell>
  </AuthGuard>
} />
```

---

## Login Page Layout

Split screen — left branding, right form.

```
┌──────────────────────────┬──────────────────────────┐
│                          │                          │
│   LEFT PANEL (45%)       │   RIGHT PANEL (55%)      │
│   Dark background        │   Surface background     │
│   #0f1117                │   #161b27                │
│                          │                          │
│   Logo + brand           │   Login form             │
│   Tagline                │                          │
│   Feature highlights     │                          │
│   Screenshot/visual      │                          │
│                          │                          │
└──────────────────────────┴──────────────────────────┘
```

---

## Left Panel

Background: `#0f1117`
Full height, sticky

### Content (centered vertically)
```
⚡ obsAdmin

Open-source observability platform
for modern engineering teams.

────────────────────────────────

✓ Unified logs, metrics & traces
✓ Real-time alerting & incidents
✓ Distributed tracing & APM
✓ Infrastructure monitoring
✓ Synthetics & uptime checks

────────────────────────────────

[Mini dashboard screenshot — SVG illustration
 showing a dark dashboard with charts]
```

Logo: `Activity` icon from lucide, `#06b6d4`, 28px + "obsAdmin" text, 22px weight 600
Tagline: `body1`, `text.secondary`
Feature list: checkmarks in `#06b6d4`, text `text.secondary`, 14px
Mini visual: SVG mockup of the dashboard, subtle, low opacity

Bottom of left panel:
```
MIT License  •  v0.1.0  •  github.com/obsadmin
```
Font size 11px, color `text.tertiary`

---

## Right Panel

Background: `#161b27`
Full height, overflow-y auto

### Content (centered vertically, max-width 400px, mx auto)

```
Welcome back

Sign in to your obsAdmin instance

[─────────────────────────────────]  ← GitHub SSO button
[─────────────────────────────────]  ← Google SSO button

─────────── or continue with ───────────

Email
[________________________]

Password
[________________________] [👁]

[Forgot password?]                (right-aligned link)

[Sign In]                         (full width, contained, cyan)

─────────────────────────────────────────

🔑 Demo Access
────────────────────────────────────────
Email:    demo@obsadmin.io
Password: ObsAdmin@demo

These credentials are pre-filled above.
JWT expires in 24 hours.
────────────────────────────────────────
```

---

## Login Form Component

`src/pages/auth/components/LoginForm.tsx`

Uses React Hook Form + Zod:

```typescript
const loginSchema = z.object({
  email:    z.string().email('Invalid email address'),
  password: z.string().min(1, 'Password is required'),
});
```

### Pre-filled demo credentials
Form defaultValues:
```typescript
defaultValues: {
  email:    'demo@obsadmin.io',
  password: 'ObsAdmin@demo',
}
```

### Loading state
On submit: button shows `<CircularProgress size={16} />` + "Signing in..." text
Input fields disabled during loading

### Error state
Wrong credentials: red helper text below password field
```
Invalid email or password
```
MUI `<Alert severity="error">` above the form:
```
Invalid email or password. Use demo@obsadmin.io / ObsAdmin@demo to try the demo.
```

### Success flow
On successful login:
1. `notify.success('Welcome back, Demo User!')` 
2. Navigate to `/` (dashboard)
3. If came from a protected route: navigate back to `location.state.from`

### Password visibility toggle
Eye icon inside password field — `Eye` / `EyeOff` from lucide-react
Click toggles `type="password"` ↔ `type="text"`

---

## SSO Buttons Component

`src/pages/auth/components/SSOButtons.tsx`

```tsx
// Two buttons, full width, outlined style
// GitHub: GitHub icon (lucide) + "Continue with GitHub"
// Google: custom Google SVG icon + "Continue with Google"

// On click: show notify.info('SSO not configured in demo mode')
// Both buttons disabled in demo — tooltip: "SSO available when self-hosted"
```

Button styling:
```
background: #0f1117  (dark) / #f9fafb  (light)
border: 1px solid #2a3147  (dark) / #e5e7eb  (light)
color: text.primary
height: 40px
border-radius: 4px
font-size: 13px
font-weight: 500
hover: background bg.hover
```

---

## Demo Credentials Display

`src/pages/auth/components/DemoCredentials.tsx`

```tsx
// Subtle box at bottom of form
// Background: #06b6d415 (dark) / #eff6ff (light)
// Border: 1px solid #06b6d440
// Border-radius: 4px
// Padding: 12px 16px

// Shows:
// 🔑 Demo Access  (caption, uppercase, #06b6d4)
// Email + password in mono font
// "Pre-filled above" note in text.tertiary
```

---

## Forgot Password Page

Route: `/forgot-password`

Same split layout as login — left panel identical.

Right panel form:
```
Forgot your password?

Enter your email address and we'll send
you a reset link.

Email
[________________________]

[Send Reset Link]         (full width, cyan)

← Back to sign in        (link)
```

On submit:
- Loading state 1s
- Show success state:
```
✓ Check your email

We sent a password reset link to
demo@obsadmin.io

The link expires in 30 minutes.

[← Back to sign in]
```
- In demo mode: `notify.info('Password reset not available in demo mode')`

Zod schema:
```typescript
z.object({
  email: z.string().email('Please enter a valid email')
})
```

---

## Update Topbar — User Info

Now that auth exists, update `Topbar.tsx` to use real user from auth store:

```typescript
const { user, logout } = useAuthStore();

// Avatar circle shows user.avatar (initials)
// User menu shows user.name + user.email
// Sign out → logout() + navigate('/login') + notify.success('Signed out')
```

---

## Install dependency

```bash
npm install js-cookie
npm install --save-dev @types/js-cookie
```

---

## Prompt for Deepseek

```
Read all files in docs/design-system/.
Now read docs/design-system/phase8-login.md.

Build in this order:

1. src/lib/auth.ts — token helpers (generateDemoToken, isTokenExpired)
2. src/store/authStore.ts — Zustand auth store with demo login
3. src/components/guards/AuthGuard.tsx — route protection
4. src/pages/auth/components/SSOButtons.tsx
5. src/pages/auth/components/DemoCredentials.tsx
6. src/pages/auth/components/LoginForm.tsx — React Hook Form + Zod
   - Pre-fill demo credentials as defaultValues
   - Password visibility toggle
   - Loading + error states
7. src/pages/auth/LoginPage.tsx — split screen layout
   - Left: branding, features list, version info
   - Right: SSO buttons + divider + LoginForm + DemoCredentials
8. src/pages/auth/ForgotPasswordPage.tsx — same split layout
9. Update App.tsx — wrap all existing routes in AuthGuard
10. Update Topbar.tsx — use useAuthStore for user info + logout

CRITICAL RULES:
- Demo credentials MUST be pre-filled in form defaultValues
- Token expiry check on store rehydration — expired = force logout
- AuthGuard redirects to /login with location state for redirect back
- After login navigates to location.state.from OR '/'
- SSO buttons show notify.info in demo mode — never throw errors
- Left panel must be visually rich — not just a logo and text
- Password field must have show/hide toggle
- Do not use any authentication library — implement with Zustand + cookie only
```
