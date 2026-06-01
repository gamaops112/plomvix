# Plomvix — Sprint 11 Code Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 + React 18 + TypeScript + Vite | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.
> If a Verify step fails, fix the issue and re-run the same Verify step before continuing.

---

## CONTEXT

Sprints 1–10 are complete. Sprint 11 begins **Plomvix v1.1** and adds the **UI foundation only**.

Sprint 11 implements:
- `ui/` as a Vite + React 18 + TypeScript app
- Root Makefile targets: `ui-install`, `ui-dev`, `ui-build`, `dev`, and updated `build`
- `UIConfig` struct in Go config with `ui.enabled` and `ui.dev_mode` fields
- Go server serves built React app at `/app/*` in production
- Go server proxies `/app/*` to Vite on port 3000 in development
- Minimal React shell with placeholder pages only
- Go tests for the SPA handler
- Updated `.gitignore`

**What Sprint 11 does NOT do:**
- No theme engine, no `theme.json`, no developer design panel
- No login/logout UI, no admin UI, no API key UI
- No log explorer, no trace storage, no OTLP, no Prometheus remote write

---

## SPRINT 11 GOAL

By the end of Sprint 11, Plomvix has a working frontend foundation. Developers run
the React app on port 3000 during development, build it into `ui/dist/`, and the Go
server serves the built app from `/app/*` in production.

---

## FEATURE 1 — Workspace setup

---

### TASK 01 — Verify Node and npm

**Action:**
```bash
node --version   # must be 20.x or newer
npm --version
```

**Verify:** Both print versions and exit 0.

---

### TASK 02 — Create ui/ directory

**Action:**
```bash
mkdir -p ui/src
```

**Verify:**
```bash
test -d ui && test -d ui/src
```

---

### TASK 03 — Create ui/package.json

**Action:** Create `ui/package.json` with pinned versions.

**Full file content:**
```json
{
  "name": "plomvix-ui",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite --host 0.0.0.0 --port 3000",
    "build": "tsc -b && vite build",
    "preview": "vite preview --host 0.0.0.0 --port 3000",
    "typecheck": "tsc -b"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^6.26.0"
  },
  "devDependencies": {
    "@types/react": "^18.3.0",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.0",
    "concurrently": "^8.2.0",
    "typescript": "^5.5.0",
    "vite": "^5.4.0"
  }
}
```

**NOTE:** All versions are pinned with `^` — no `"latest"`. This ensures
reproducible builds regardless of when `npm install` is run.

**Verify:**
```bash
cat ui/package.json | python3 -m json.tool > /dev/null
grep -q '"vite": "\^' ui/package.json
```

---

### TASK 04 — Create ui/tsconfig.json

**Action:** Create `ui/tsconfig.json`.

**Full file content:**
```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["DOM", "DOM.Iterable", "ES2020"],
    "allowJs": false,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "forceConsistentCasingInFileNames": true,
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx"
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

**NOTE:** `moduleResolution` is `"Bundler"` not `"Node"`. Vite 5+ requires
`"Bundler"` — `"Node"` is deprecated for bundler-based projects and causes
errors with `import.meta.env` and Vite-specific imports.

**Verify:**
```bash
cat ui/tsconfig.json | python3 -m json.tool > /dev/null
grep -q '"moduleResolution": "Bundler"' ui/tsconfig.json
```

---

### TASK 05 — Create ui/tsconfig.node.json

**Action:** Create `ui/tsconfig.node.json`.

**Full file content:**
```json
{
  "compilerOptions": {
    "composite": true,
    "skipLibCheck": true,
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "allowSyntheticDefaultImports": true
  },
  "include": ["vite.config.ts"]
}
```

**NOTE:** `moduleResolution` is `"Bundler"` here too — must match `tsconfig.json`.

**Verify:**
```bash
cat ui/tsconfig.node.json | python3 -m json.tool > /dev/null
grep -q '"moduleResolution": "Bundler"' ui/tsconfig.node.json
```

---

### TASK 06 — Create ui/vite.config.ts

**Action:** Create `ui/vite.config.ts`.

**Full file content:**
```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: '/app/',
  server: {
    host: '0.0.0.0',
    port: 3000,
    strictPort: true,
    proxy: {
      '/api':    'http://localhost:8080',
      '/auth':   'http://localhost:8080',
      '/admin':  'http://localhost:8080',
      '/query':  'http://localhost:8080',
      '/ingest': 'http://localhost:8080',
      '/health': 'http://localhost:8080'
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  }
});
```

**Why `base: '/app/'`:** Go serves the SPA under `/app/*`, so all Vite
asset URLs must resolve under `/app/assets/*`.

**Verify:**
```bash
test -f ui/vite.config.ts
grep -q "base: '/app/'" ui/vite.config.ts
```

---

### TASK 07 — Create ui/index.html

**Action:** Create `ui/index.html`.

**Full file content:**
```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Plomvix</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

**Verify:**
```bash
test -f ui/index.html
grep -q 'id="root"' ui/index.html
```

---

### TASK 08 — Install UI dependencies

**Action:**
```bash
cd ui && npm install
```

**Verify:**
```bash
test -f ui/package-lock.json
test -d ui/node_modules
```

If `npm run typecheck` fails here because source files do not exist yet,
continue — re-run typecheck after TASK 19.

---

## FEATURE 2 — Go config: UIConfig struct

**This must be done before TASK 28 which uses `cfg.UI.Enabled` and `cfg.UI.DevMode`.**

---

### TASK 09 — Add UIConfig to internal/config/config.go

**Action:** Add `UIConfig` struct and `UI` field to the existing `Config` struct.

**Add this struct:**
```go
// UIConfig holds configuration for the web UI.
type UIConfig struct {
    Enabled bool `mapstructure:"enabled"`
    DevMode bool `mapstructure:"dev_mode"`
}
```

**Add this field to Config struct:**
```go
UI UIConfig `mapstructure:"ui"`
```

**Add validation rules** inside the existing `validate()` function — no
validation needed for UI booleans, but add a comment:
```go
// UI config — no validation needed, booleans default to false safely
```

**Verify:**
```go
CGO_ENABLED=1 go build ./internal/config/
```

---

### TASK 10 — Add ui section to config.yaml

**Action:** Add to `config.yaml`:
```yaml
ui:
  enabled: true        # Set to false to disable the web UI entirely
  dev_mode: false      # Set to true in development to proxy /app/* to Vite on port 3000
```

**Add to .golangci.yml** — no changes needed, UIConfig fields are exported and
will be linted correctly.

**Verify:**
```bash
grep -q 'ui:' config.yaml
grep -q 'dev_mode' config.yaml
```

---

## FEATURE 3 — App events and toast system

---

### TASK 11 — Create ui/src/events/types.ts

**Action:** Create `ui/src/events/types.ts`.

**Full file content:**
```ts
export type ToastKind = 'success' | 'error' | 'warning' | 'info';

export type ToastInput = {
  kind?: ToastKind;
  title: string;
  message?: string;
  durationMs?: number;
};

export type ToastItem = Required<Pick<ToastInput, 'kind' | 'title' | 'durationMs'>> & {
  id: string;
  message?: string;
};

export type AppEvent =
  | { type: 'toast:add'; payload: ToastInput }
  | { type: 'toast:remove'; payload: { id: string } }
  | { type: 'app:ready' };
```

**Verify:**
```bash
test -f ui/src/events/types.ts
```

---

### TASK 12 — Create ui/src/events/AppEventProvider.tsx

**Action:** Create `ui/src/events/AppEventProvider.tsx`.

**Full file content:**
```tsx
import { createContext, ReactNode, useCallback, useContext, useMemo, useState } from 'react';
import type { AppEvent, ToastItem } from './types';

type AppEventContextValue = {
  toasts: ToastItem[];
  emit: (event: AppEvent) => void;
};

const AppEventContext = createContext<AppEventContextValue | null>(null);

export function AppEventProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const emit = useCallback((event: AppEvent) => {
    switch (event.type) {
      case 'toast:add': {
        const toast: ToastItem = {
          id: crypto.randomUUID(),
          kind: event.payload.kind ?? 'info',
          title: event.payload.title,
          message: event.payload.message,
          durationMs: event.payload.durationMs ?? 5000
        };
        setToasts((prev) => [...prev, toast]);
        window.setTimeout(() => {
          setToasts((prev) => prev.filter((t) => t.id !== toast.id));
        }, toast.durationMs);
        return;
      }
      case 'toast:remove': {
        setToasts((prev) => prev.filter((t) => t.id !== event.payload.id));
        return;
      }
      case 'app:ready': {
        return;
      }
    }
  }, []);

  const value = useMemo(() => ({ toasts, emit }), [toasts, emit]);

  return (
    <AppEventContext.Provider value={value}>
      {children}
    </AppEventContext.Provider>
  );
}

export function useAppEvents() {
  const ctx = useContext(AppEventContext);
  if (!ctx) throw new Error('useAppEvents must be used inside AppEventProvider');
  return ctx;
}
```

**Verify:**
```bash
test -f ui/src/events/AppEventProvider.tsx
```

---

### TASK 13 — Create ui/src/components/feedback/ToastViewport.tsx

**Action:** Create `ui/src/components/feedback/ToastViewport.tsx`.

**Full file content:**
```tsx
import { useAppEvents } from '../../events/AppEventProvider';

export function ToastViewport() {
  const { toasts, emit } = useAppEvents();

  return (
    <div className="toast-viewport" aria-live="polite" aria-relevant="additions removals">
      {toasts.map((toast) => (
        <div className={`toast toast-${toast.kind}`} key={toast.id} role="status">
          <div>
            <strong>{toast.title}</strong>
            {toast.message ? <p>{toast.message}</p> : null}
          </div>
          <button
            type="button"
            className="toast-close"
            aria-label={`Dismiss ${toast.title}`}
            onClick={() => emit({ type: 'toast:remove', payload: { id: toast.id } })}
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
```

**Verify:**
```bash
test -f ui/src/components/feedback/ToastViewport.tsx
```

---

### TASK 14 — Create ui/src/components/ui/Button.tsx

**Action:** Create `ui/src/components/ui/Button.tsx`.

**Full file content:**
```tsx
import type { ButtonHTMLAttributes, ReactNode } from 'react';

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  variant?: 'primary' | 'secondary' | 'ghost';
};

export function Button({
  children,
  variant = 'primary',
  className = '',
  ...props
}: ButtonProps) {
  return (
    <button
      className={`button button-${variant} ${className}`.trim()}
      type="button"
      {...props}
    >
      {children}
    </button>
  );
}
```

**Verify:**
```bash
test -f ui/src/components/ui/Button.tsx
```

---

## FEATURE 4 — Routes and navigation

---

### TASK 15 — Create placeholder pages

**Action:** Create four page files.

**ui/src/pages/HomePlaceholder.tsx:**
```tsx
import { Button } from '../components/ui/Button';
import { useAppEvents } from '../events/AppEventProvider';

export function HomePlaceholder() {
  const { emit } = useAppEvents();
  return (
    <div className="page-card">
      <h1>Plomvix</h1>
      <p>UI foundation — Sprint 11</p>
      <Button
        onClick={() =>
          emit({ type: 'toast:add', payload: { kind: 'success', title: 'Toast works!' } })
        }
      >
        Test toast
      </Button>
    </div>
  );
}
```

**ui/src/pages/ExplorePlaceholder.tsx:**
```tsx
export function ExplorePlaceholder() {
  return <div className="page-card"><h1>Explore</h1><p>Coming soon.</p></div>;
}
```

**ui/src/pages/AdminPlaceholder.tsx:**
```tsx
export function AdminPlaceholder() {
  return <div className="page-card"><h1>Admin</h1><p>Coming soon.</p></div>;
}
```

**ui/src/pages/NotFoundPage.tsx:**
```tsx
export function NotFoundPage() {
  return <div className="page-card"><h1>404</h1><p>Page not found.</p></div>;
}
```

**Verify:**
```bash
test -f ui/src/pages/HomePlaceholder.tsx
test -f ui/src/pages/ExplorePlaceholder.tsx
test -f ui/src/pages/AdminPlaceholder.tsx
test -f ui/src/pages/NotFoundPage.tsx
grep -q 'toast:add' ui/src/pages/HomePlaceholder.tsx
```

---

### TASK 16 — Create route registry

**Action:** Create `ui/src/app/routes.tsx`.

**Full file content:**
```tsx
import type { ReactNode } from 'react';
import { HomePlaceholder } from '../pages/HomePlaceholder';
import { ExplorePlaceholder } from '../pages/ExplorePlaceholder';
import { AdminPlaceholder } from '../pages/AdminPlaceholder';

export type AppRoute = {
  path: string;
  label: string;
  element: ReactNode;
  nav: boolean;
};

export const appRoutes: AppRoute[] = [
  { path: '/',        label: 'Home',    element: <HomePlaceholder />,    nav: false },
  { path: '/explore', label: 'Explore', element: <ExplorePlaceholder />, nav: true  },
  { path: '/admin',   label: 'Admin',   element: <AdminPlaceholder />,   nav: true  },
];

export const navRoutes = appRoutes.filter((route) => route.nav);
```

**Verify:**
```bash
test -f ui/src/app/routes.tsx
grep -q 'navRoutes' ui/src/app/routes.tsx
grep -q 'appRoutes.filter' ui/src/app/routes.tsx
```

---

### TASK 17 — Create AppRoutes component

**Action:** Create `ui/src/app/AppRoutes.tsx`.

**Full file content:**
```tsx
import { Route, Routes } from 'react-router-dom';
import { appRoutes } from './routes';
import { NotFoundPage } from '../pages/NotFoundPage';

export function AppRoutes() {
  return (
    <Routes>
      {appRoutes.map((route) => (
        <Route key={route.path} path={route.path} element={route.element} />
      ))}
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  );
}
```

**Verify:**
```bash
test -f ui/src/app/AppRoutes.tsx
grep -q 'appRoutes.map' ui/src/app/AppRoutes.tsx
```

---

### TASK 18 — Create Sidebar component

**Action:** Create `ui/src/components/layout/Sidebar.tsx`.

**Full file content:**
```tsx
import { NavLink } from 'react-router-dom';
import { navRoutes } from '../../app/routes';

export function Sidebar() {
  return (
    <nav className="sidebar">
      <div className="sidebar-logo">Plomvix</div>
      <ul className="sidebar-nav">
        {navRoutes.map((route) => (
          <li key={route.path}>
            <NavLink
              to={route.path}
              className={({ isActive }) =>
                `sidebar-nav-item${isActive ? ' sidebar-nav-item--active' : ''}`
              }
            >
              {route.label}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  );
}
```

**Verify:**
```bash
test -f ui/src/components/layout/Sidebar.tsx
grep -q 'navRoutes.map' ui/src/components/layout/Sidebar.tsx
```

---

### TASK 19 — Create Shell component

**Action:** Create `ui/src/components/layout/Shell.tsx`.

**Full file content:**
```tsx
import type { ReactNode } from 'react';
import { Sidebar } from './Sidebar';

export function Shell({ children }: { children: ReactNode }) {
  return (
    <div className="shell">
      <Sidebar />
      <main className="shell-main">{children}</main>
    </div>
  );
}
```

**Verify:**
```bash
test -f ui/src/components/layout/Shell.tsx
```

---

### TASK 20 — Create App component

**Action:** Create `ui/src/App.tsx`.

**Full file content:**
```tsx
import { Shell } from './components/layout/Shell';
import { AppRoutes } from './app/AppRoutes';
import { ToastViewport } from './components/feedback/ToastViewport';

export function App() {
  return (
    <>
      <Shell>
        <AppRoutes />
      </Shell>
      <ToastViewport />
    </>
  );
}
```

**Verify:**
```bash
test -f ui/src/App.tsx
grep -q 'ToastViewport' ui/src/App.tsx
grep -q 'AppRoutes' ui/src/App.tsx
```

---

### TASK 21 — Create main.tsx

**Action:** Create `ui/src/main.tsx`.

**Full file content:**
```tsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AppEventProvider } from './events/AppEventProvider';
import { App } from './App';
import './styles.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter basename="/app">
      <AppEventProvider>
        <App />
      </AppEventProvider>
    </BrowserRouter>
  </StrictMode>
);
```

**Verify:**
```bash
test -f ui/src/main.tsx
grep -q 'AppEventProvider' ui/src/main.tsx
grep -q 'basename="/app"' ui/src/main.tsx
```

---

## FEATURE 5 — Base styling

---

### TASK 22 — Create ui/src/styles.css

**Action:** Create `ui/src/styles.css`.

**Requirements:**
- Define CSS variables in `:root` for colors, spacing, font sizes
- Style `.shell` as a two-column grid (sidebar + main)
- Style `.sidebar`, `.sidebar-logo`, `.sidebar-nav`, `.sidebar-nav-item`, `.sidebar-nav-item--active`
- Style `.shell-main`
- Style `.page-card`
- Style `.button`, `.button-primary`, `.button-secondary`, `.button-ghost`
- Style `.toast-viewport`, `.toast`, `.toast-success`, `.toast-error`, `.toast-warning`, `.toast-info`, `.toast-close`
- Keep all styles minimal and functional — full design system comes in a later sprint

**Verify:**
```bash
test -f ui/src/styles.css
grep -q 'toast-viewport' ui/src/styles.css
grep -q 'button-primary' ui/src/styles.css
grep -q ':root' ui/src/styles.css
```

---

### TASK 23 — Build and typecheck the UI

**Action:**
```bash
cd ui && npm run typecheck && npm run build
```

**Verify:**
```bash
test -f ui/dist/index.html
grep -q '/app/assets/' ui/dist/index.html
```

---

## FEATURE 6 — Update .gitignore

---

### TASK 24 — Update .gitignore

**Action:** Add the following lines to `.gitignore` at the project root:

```gitignore
# UI build output and dependencies
ui/node_modules/
ui/dist/
```

**Verify:**
```bash
grep -q 'ui/node_modules/' .gitignore
grep -q 'ui/dist/' .gitignore
```

---

## FEATURE 7 — Update Makefile

---

### TASK 25 — Add UI targets to Makefile and update build target

**Action:** Add the following targets to the root `Makefile`.

**New targets to add:**
```makefile
## ui-install: Install UI npm dependencies
ui-install:
	cd ui && npm install

## ui-dev: Start Vite development server on port 3000
ui-dev:
	cd ui && npm run dev

## ui-build: Build the React app into ui/dist/
ui-build:
	cd ui && npm run build

## dev: Start Go server and Vite dev server together (development mode)
dev:
	cd ui && npx concurrently \
		"PLOMVIX_UI_DEV_MODE=true go run $(LDFLAGS) ../cmd/plomvix" \
		"npm run dev"
```

**Update the existing `build` target** to also build the UI:
```makefile
## build: Build the Plomvix binary and the React UI
build: ui-build
	go build $(LDFLAGS) -o $(BINARY) ./cmd/plomvix
```

**NOTE:** `LDFLAGS`, `BINARY`, and other variables are already defined in the
existing Makefile from Sprint 1 — do not redefine them, just reference them.

**Verify:**
```bash
make -n ui-install | grep -q 'npm install'
make -n ui-dev     | grep -q 'npm run dev'
make -n ui-build   | grep -q 'npm run build'
make -n dev        | grep -q 'concurrently'
make -n build      | grep -q 'ui-build'
```

---

## FEATURE 8 — Go SPA handler

---

### TASK 26 — Create internal/server/ui.go

**Action:** Create `internal/server/ui.go`.

**Full file content:**
```go
package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// newSPAHandler returns an http.Handler that serves a Vite-built React SPA
// from distDir. It handles three cases:
//  1. index.html missing → 503 with instructions to run make ui-build
//  2. real asset file (e.g. /app/assets/main.js) → serve the file
//  3. any other /app/* route → serve index.html (client-side routing)
func newSPAHandler(distDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(distDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		indexPath := filepath.Join(distDir, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			http.Error(w, "Plomvix UI is not built. Run make ui-build.", http.StatusServiceUnavailable)
			return
		}

		// Strip the /app prefix to get the path relative to distDir
		trimmed := strings.TrimPrefix(r.URL.Path, "/app")
		if trimmed == "" || trimmed == "/" {
			http.ServeFile(w, r, indexPath)
			return
		}

		// Path traversal guard — reject any path that escapes distDir
		cleanDist := filepath.Clean(distDir)
		requestedPath := filepath.Join(cleanDist, filepath.Clean(trimmed))
		if requestedPath != cleanDist &&
			!strings.HasPrefix(requestedPath, cleanDist+string(os.PathSeparator)) {
			http.NotFound(w, r)
			return
		}

		// Serve real files that exist (e.g. assets/main.js, assets/main.css)
		if info, err := os.Stat(requestedPath); err == nil && !info.IsDir() {
			http.StripPrefix("/app", fileServer).ServeHTTP(w, r)
			return
		}

		// Assets path with no matching file → 404 (do not fall through to index.html)
		if strings.HasPrefix(trimmed, "/assets/") {
			http.NotFound(w, r)
			return
		}

		// All other routes → serve index.html for client-side routing
		http.ServeFile(w, r, indexPath)
	})
}
```

**Verify:**
```bash
CGO_ENABLED=1 go build ./internal/server/
```

---

### TASK 27 — Create internal/server/ui_proxy.go

**Action:** Create `internal/server/ui_proxy.go`.

**Full file content:**
```go
package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// newUIProxyHandler returns an http.Handler that reverse-proxies all requests
// to the target URL. Used in development to proxy /app/* to the Vite dev server.
func newUIProxyHandler(target string) (http.Handler, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	return httputil.NewSingleHostReverseProxy(u), nil
}
```

**Verify:**
```bash
CGO_ENABLED=1 go build ./internal/server/
```

---

### TASK 28 — Register /app routes in server.go

**Action:** Update `internal/server/server.go` route setup.

Add the following block inside `setupRoutes()` after all existing routes are
registered. This must not touch any existing route registrations.

```go
// UI routes — served last so they cannot shadow API routes
if s.cfg.UI.Enabled {
    if s.cfg.UI.DevMode {
        uiProxy, err := newUIProxyHandler("http://localhost:3000")
        if err != nil {
            // misconfigured target URL — fail at startup, not at request time
            panic(fmt.Sprintf("failed to create UI proxy handler: %v", err))
        }
        r.Handle("/app", uiProxy)
        r.Handle("/app/*", uiProxy)
    } else {
        uiHandler := newSPAHandler("ui/dist")
        r.Handle("/app", uiHandler)
        r.Handle("/app/*", uiHandler)
    }
}
```

**Import to add to server.go:**
```go
"fmt"
```

Check if `"fmt"` is already imported — only add it if not present.

**Rules:**
- Preserve all existing routes unchanged
- `/health`, `/docs`, `/openapi.json` must continue to work exactly as before
- `/app` routes are always registered last

**Verify:**
```bash
CGO_ENABLED=1 go build ./internal/server/
CGO_ENABLED=1 go test ./internal/server/...
```

---

## FEATURE 9 — Go tests for SPA handler

---

### TASK 29 — Create internal/server/ui_test.go

**Action:** Create `internal/server/ui_test.go`.

**Full file content:**
```go
package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSPAHandlerServesIndexForAppRoute(t *testing.T) {
	distDir := createTestUIDist(t)
	handler := newSPAHandler(distDir)

	req := httptest.NewRequest(http.MethodGet, "/app/explore", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Plomvix Test UI") {
		t.Fatalf("expected index.html content, got %q", rec.Body.String())
	}
}

func TestSPAHandlerServesAsset(t *testing.T) {
	distDir := createTestUIDist(t)
	handler := newSPAHandler(distDir)

	req := httptest.NewRequest(http.MethodGet, "/app/assets/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Fatalf("expected JS content, got %q", rec.Body.String())
	}
}

func TestSPAHandlerReturnsNotFoundForMissingAsset(t *testing.T) {
	distDir := createTestUIDist(t)
	handler := newSPAHandler(distDir)

	req := httptest.NewRequest(http.MethodGet, "/app/assets/missing.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSPAHandlerReturns503WhenUINotBuilt(t *testing.T) {
	distDir := t.TempDir() // empty — no index.html
	handler := newSPAHandler(distDir)

	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Run make ui-build") {
		t.Fatalf("expected build instruction in body, got %q", rec.Body.String())
	}
}

func TestSPAHandlerRejectsPathTraversal(t *testing.T) {
	distDir := createTestUIDist(t)
	handler := newSPAHandler(distDir)

	req := httptest.NewRequest(http.MethodGet, "/app/../../../etc/passwd", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Must not return 200 for a path traversal attempt
	if rec.Code == http.StatusOK {
		t.Fatal("path traversal should not return 200")
	}
}

// createTestUIDist creates a minimal fake ui/dist/ for testing.
func createTestUIDist(t *testing.T) string {
	t.Helper()
	distDir := t.TempDir()

	assetsDir := filepath.Join(distDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(distDir, "index.html"),
		[]byte("<!doctype html><html><body>Plomvix Test UI</body></html>"),
		0644,
	); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(assetsDir, "app.js"),
		[]byte("console.log('plomvix');"),
		0644,
	); err != nil {
		t.Fatalf("failed to write app.js: %v", err)
	}

	return distDir
}
```

**Verify:**
```bash
CGO_ENABLED=1 go test -race ./internal/server/...
```

---

## FEATURE 10 — Full verification

---

### TASK 30 — Run full Go test suite

**Action:**
```bash
CGO_ENABLED=1 go test ./...
```

**Verify:** All tests pass.

---

### TASK 31 — Run UI typecheck and build

**Action:**
```bash
cd ui && npm run typecheck && npm run build
```

**Verify:**
```bash
test -f ui/dist/index.html
grep -q '/app/assets/' ui/dist/index.html
```

---

### TASK 32 — Production smoke test

**Action:**
```bash
make build
./plomvix
```

In a second terminal:
```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/app
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/app/explore
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/app/admin
```

**Verify:** All four return `200`. Stop the server.

---

### TASK 33 — Missing UI build smoke test

**Action:**
```bash
mv ui/dist ui/dist.bak
./plomvix &
sleep 2
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/app)
kill %1
mv ui/dist.bak ui/dist
echo $STATUS
```

**Verify:** `$STATUS` is `503`.

---

### TASK 34 — Development proxy smoke test

**Action — Terminal 1:**
```bash
cd ui && npm run dev
```

**Action — Terminal 2:**
```bash
PLOMVIX_UI_DEV_MODE=true go run ./cmd/plomvix
```

**Action — Terminal 3:**
```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/app
curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/health
```

**Verify:**
- `http://localhost:8080/app` returns `200` (proxied from Vite)
- `http://localhost:3000/health` returns `200` (Vite proxies to Go)

Stop both servers.

---

### TASK 35 — Run gofmt

**Action:**
```bash
find . -name '*.go' -not -path './vendor/*' -exec gofmt -w {} +
```

**Verify:**
```bash
git diff --check
```

---

### TASK 36 — Run lint

**Action:**
```bash
CGO_ENABLED=1 make lint
```

**Verify:** Exits with code 0.

---

### TASK 37 — Add docs/ui.md

**Action:** Create `docs/ui.md`.

**Full file content:**
```markdown
# Plomvix UI

Sprint 11 introduces the Plomvix web UI foundation.

## Development

Start Go backend and Vite together:

```bash
make ui-install   # first time only
make dev
```

Ports:
- Go API: http://localhost:8080
- Vite UI: http://localhost:3000

`PLOMVIX_UI_DEV_MODE=true` makes Go proxy `/app/*` to Vite.

## Production build

```bash
make build   # builds both Go binary and React app
```

React app builds into `ui/dist/`. Go serves it from `GET /app/*`.

## Current scope (Sprint 11)

Shell and placeholder routes only. Not yet included:
- Login / logout UI
- Admin UI
- Log Explorer
- Theme engine / Developer Design Panel
```

**Verify:**
```bash
test -f docs/ui.md
grep -q 'make dev' docs/ui.md
```

---

### TASK 38 — Update README.md

**Action:** Add to `README.md` near the existing usage section:

```markdown
## Web UI

Plomvix includes a React-based web UI served from `/app/*`.

```bash
make ui-install   # install UI dependencies (first time only)
make dev          # start Go + Vite together for development
make build        # build both Go binary and React app for production
```

See [docs/ui.md](docs/ui.md) for details.
```

**Verify:**
```bash
grep -q 'Web UI' README.md
grep -q 'docs/ui.md' README.md
```

---

### TASK 39 — Final retest loop

**Action:** Run all of these in order:
```bash
find . -name '*.go' -not -path './vendor/*' -exec gofmt -w {} +
cd ui && npm run typecheck && npm run build && cd ..
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 make lint
make build
git diff --check
```

If any command fails — fix it and re-run the entire list from the top.
Do not mark Sprint 11 complete until the full list passes in one clean run.

---

## SPRINT 11 COMPLETION CHECKLIST

- `ui/` exists with Vite + React 18 + TypeScript
- `ui/package.json` has all deps pinned with `^` — no `"latest"`
- `tsconfig.json` and `tsconfig.node.json` use `"moduleResolution": "Bundler"`
- `make ui-install` installs npm deps
- `make ui-dev` starts Vite on port 3000
- `make ui-build` creates `ui/dist/`
- `make dev` starts Go + Vite concurrently
- `make build` builds UI then Go binary
- `.gitignore` includes `ui/node_modules/` and `ui/dist/`
- `config.yaml` has `ui.enabled` and `ui.dev_mode`
- `UIConfig` struct exists in `internal/config/config.go`
- `GET /app/*` serves `ui/dist/` when `ui.dev_mode=false`
- `GET /app/*` proxies to Vite when `ui.dev_mode=true`
- Path traversal test passes
- `/health` still returns 200
- `docs/ui.md` exists
- Final retest loop passes clean