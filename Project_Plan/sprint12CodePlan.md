# Plomvix — Sprint 12 Task Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: Go 1.22 | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–11 are complete. Sprint 11 added the **UI Foundation only**:
Vite + React + TypeScript, data-driven route registry, sidebar, app event provider,
toast system, button primitive, placeholder pages, Go static UI serving, Vite dev
proxying, UI config, and Makefile UI targets.

Sprint 12 adds the **Theme Engine + Developer Design Panel**. This is the first
real UI customization layer, but it must remain scoped: no login UI, no protected
routes, no admin UI, no log explorer, no traces, no OTLP, and no Prometheus.

**What Sprint 12 delivers:**
- `theme.json` at project root
- `internal/theme/` backend package
- `GET /api/theme` public endpoint
- `PUT /api/theme` admin endpoint
- `POST /api/theme/reset` admin endpoint
- `GET /api/theme/export` admin endpoint
- Backend validation for all design tokens
- Frontend `ThemeProvider` and `useTheme()` hook
- Theme loading on app startup
- Theme tokens injected as CSS variables
- Light/dark mode toggle
- `dev_panel` boolean in `theme.json`
- `/dev/design` route visible only when `dev_panel === true`
- Developer Design Panel with live token editing and component previews
- Import/export support in the design panel
- Save/reset buttons wired to backend APIs
- OpenAPI and docs updated for theme endpoints
- Unit tests for backend theme loading, validation, save, reset, and export
- Frontend type-check/build verification

**What Sprint 12 does NOT do:**
- No authentication UI — Sprint 13
- No protected route wrapper — Sprint 13
- No admin dashboard — Sprint 14
- No log explorer — Sprint 15
- No trace storage or trace UI — Sprint 16
- No OTLP or Prometheus remote write — Sprint 17
- No database storage for themes — `theme.json` is file-backed in Sprint 12
- No multi-user theme preferences — one global theme only

---

## THEME DESIGN — READ BEFORE WRITING ANY CODE

The theme is a global design-token document stored at project root:

```
theme.json
```

The backend loads, validates, serves, saves, resets, and exports this file.
The frontend consumes the theme and maps tokens to CSS variables.

**Theme shape:**

```json
{
  "version": 1,
  "dev_panel": true,
  "mode": "light",
  "tokens": {
    "colors": {
      "primary": "#2563eb",
      "secondary": "#64748b",
      "accent": "#14b8a6",
      "background": "#ffffff",
      "background_muted": "#f8fafc",
      "surface": "#ffffff",
      "surface_muted": "#f1f5f9",
      "text": "#0f172a",
      "text_muted": "#64748b",
      "border": "#e2e8f0",
      "error": "#dc2626",
      "warning": "#f59e0b",
      "success": "#16a34a",
      "info": "#0ea5e9"
    },
    "dark_colors": {
      "primary": "#60a5fa",
      "secondary": "#94a3b8",
      "accent": "#2dd4bf",
      "background": "#020617",
      "background_muted": "#0f172a",
      "surface": "#111827",
      "surface_muted": "#1e293b",
      "text": "#f8fafc",
      "text_muted": "#94a3b8",
      "border": "#334155",
      "error": "#f87171",
      "warning": "#fbbf24",
      "success": "#4ade80",
      "info": "#38bdf8"
    },
    "typography": {
      "font_family": "Inter, ui-sans-serif, system-ui, sans-serif",
      "font_size_xs": "0.75rem",
      "font_size_sm": "0.875rem",
      "font_size_base": "1rem",
      "font_size_lg": "1.125rem",
      "font_size_xl": "1.25rem",
      "font_weight_regular": "400",
      "font_weight_medium": "500",
      "font_weight_semibold": "600",
      "font_weight_bold": "700"
    },
    "radii": {
      "sm": "0.375rem",
      "md": "0.5rem",
      "lg": "0.75rem",
      "xl": "1rem"
    },
    "spacing": {
      "xs": "0.25rem",
      "sm": "0.5rem",
      "md": "1rem",
      "lg": "1.5rem",
      "xl": "2rem"
    },
    "shadows": {
      "sm": "0 1px 2px rgba(15, 23, 42, 0.08)",
      "md": "0 8px 20px rgba(15, 23, 42, 0.12)",
      "lg": "0 16px 40px rgba(15, 23, 42, 0.16)"
    },
    "layout": {
      "sidebar_width": "260px",
      "navbar_height": "56px",
      "transition_speed": "160ms"
    }
  }
}
```

**Validation rules:**
- `version` must be `1`
- `mode` must be `light` or `dark`
- `dev_panel` must be a boolean
- Every required token key must exist
- Color tokens must be valid hex colors in `#rgb` or `#rrggbb` format
- Size tokens must be non-empty CSS length strings ending in `px`, `rem`, or `em`
- Duration tokens, specifically `layout.transition_speed`, must be non-empty CSS duration strings ending in `ms` or `s`
- Font weights must be numeric strings from `100` through `900`
- Font family and shadows must be non-empty strings
- Extra/unknown JSON keys are not preserved in Sprint 12 because the Go model is intentionally typed. Required keys cannot be missing.

---

## API DESIGN — READ BEFORE WRITING ANY CODE

Theme endpoints live under `/api/theme` because `/admin/*` is already used for
system administration. `GET /api/theme` is public so the UI can load a theme before
login exists. All theme write/export operations require admin authentication.

| Method | Path | Auth | Purpose |
|---|---|---|---|
| `GET` | `/api/theme` | Public | Return current theme, creating default file if missing |
| `PUT` | `/api/theme` | Admin | Validate and save a complete theme document |
| `POST` | `/api/theme/reset` | Admin | Replace current theme with defaults |
| `GET` | `/api/theme/export` | Admin | Download current saved theme JSON |

**Response format:** JSON API endpoints use existing `pkg/utils` response helpers.
`GET /api/theme/export` is the intentional exception: it returns raw downloadable JSON.

**Important:** `PUT /api/theme` replaces the full theme document. Partial patching
is intentionally deferred.

---

## FRONTEND DESIGN — READ BEFORE WRITING ANY CODE

Sprint 11 already created the app shell, route registry, sidebar, app events,
toasts, and button primitive. Sprint 12 must build on those instead of replacing
them.

**Theme frontend rules:**
- Theme state is global and lives in `ThemeProvider`
- `ThemeProvider` loads `/api/theme` once on app startup
- CSS variables are injected on `document.documentElement`
- Active mode decides whether `tokens.colors` or `tokens.dark_colors` is applied
- Light/dark toggle only changes local UI mode until saved from the design panel
- Save/reset/import/export actions emit toast events via the Sprint 11 event system
- Save/reset/backend-export calls may return 401 until Sprint 13 adds browser login/cookie auth; Sprint 12 must show a clear error toast and must not add a temporary token input
- `/dev/design` appears in the sidebar only when `theme.dev_panel === true`
- If theme loading fails, app falls back to bundled defaults and emits an error toast

---

## TASK 01 — Create default theme.json

**Action:** Create `theme.json` at project root using the exact Theme shape from
`THEME DESIGN` above.

**Verify:**
```bash
cat theme.json | python3 -m json.tool > /dev/null
jq '.version == 1 and .dev_panel == true and .mode == "light"' theme.json | grep true
```

---

## TASK 02 — Create internal/theme directory and types.go

**Action — Part A:** Create package directory:
```bash
mkdir -p internal/theme
```

**Action — Part B:** Create `internal/theme/types.go`.

**Full file content:**
```go
package theme

// Theme is the complete persisted Plomvix design-token document.
type Theme struct {
    Version  int    `json:"version"`
    DevPanel bool   `json:"dev_panel"`
    Mode     string `json:"mode"`
    Tokens   Tokens `json:"tokens"`
}

// Tokens groups all design-token categories.
type Tokens struct {
    Colors     ColorTokens      `json:"colors"`
    DarkColors ColorTokens      `json:"dark_colors"`
    Typography TypographyTokens `json:"typography"`
    Radii      ScaleTokens      `json:"radii"`
    Spacing    ScaleTokens      `json:"spacing"`
    Shadows    ShadowTokens     `json:"shadows"`
    Layout     LayoutTokens     `json:"layout"`
}

// ColorTokens contains semantic color tokens.
type ColorTokens struct {
    Primary         string `json:"primary"`
    Secondary       string `json:"secondary"`
    Accent          string `json:"accent"`
    Background      string `json:"background"`
    BackgroundMuted string `json:"background_muted"`
    Surface         string `json:"surface"`
    SurfaceMuted    string `json:"surface_muted"`
    Text            string `json:"text"`
    TextMuted       string `json:"text_muted"`
    Border          string `json:"border"`
    Error           string `json:"error"`
    Warning         string `json:"warning"`
    Success         string `json:"success"`
    Info            string `json:"info"`
}

// TypographyTokens contains font-related tokens.
type TypographyTokens struct {
    FontFamily         string `json:"font_family"`
    FontSizeXS         string `json:"font_size_xs"`
    FontSizeSM         string `json:"font_size_sm"`
    FontSizeBase       string `json:"font_size_base"`
    FontSizeLG         string `json:"font_size_lg"`
    FontSizeXL         string `json:"font_size_xl"`
    FontWeightRegular  string `json:"font_weight_regular"`
    FontWeightMedium   string `json:"font_weight_medium"`
    FontWeightSemibold string `json:"font_weight_semibold"`
    FontWeightBold     string `json:"font_weight_bold"`
}

// ScaleTokens contains common spacing or radius scale values.
// Spacing uses xs/sm/md/lg/xl. Radii uses sm/md/lg/xl only; XS is present
// because the struct is shared, but validation skips XS for radii.
type ScaleTokens struct {
    XS string `json:"xs"`
    SM string `json:"sm"`
    MD string `json:"md"`
    LG string `json:"lg"`
    XL string `json:"xl"`
}

// ShadowTokens contains elevation tokens.
type ShadowTokens struct {
    SM string `json:"sm"`
    MD string `json:"md"`
    LG string `json:"lg"`
}

// LayoutTokens contains app-shell sizing tokens.
type LayoutTokens struct {
    SidebarWidth    string `json:"sidebar_width"`
    NavbarHeight    string `json:"navbar_height"`
    TransitionSpeed string `json:"transition_speed"`
}
```

**Verify:** `go build ./internal/theme/` compiles with no errors.

---

## TASK 03 — Create internal/theme/defaults.go

**Action:** Create `internal/theme/defaults.go`.

**Requirements:**
- Define `const DefaultPath = "theme.json"`
- Implement `DefaultTheme() *Theme`
- Return the same values as the root `theme.json`
- Return a new struct on every call so callers cannot mutate shared defaults

**Verify:** `go build ./internal/theme/` compiles with no errors.

---

## TASK 04 — Create internal/theme/validate.go

**Action:** Create `internal/theme/validate.go`.

**Imports required:**
```go
import (
    "fmt"
    "regexp"
    "strconv"
    "strings"
)
```

**Implement:**
```go
func Validate(t *Theme) error
```

**Validation behaviour:**
- Collect all validation errors into one error, like config validation
- Return errors in this format:
  ```go
  fmt.Errorf("plomvix theme validation failed:\n  - %s", strings.Join(errs, "\n  - "))
  ```
- Validate every rule listed in `THEME DESIGN`

**Helper functions to implement:**
```go
func validateColors(prefix string, c ColorTokens, errs *[]string)
func validateTypography(t TypographyTokens, errs *[]string)
func validateScale(prefix string, s ScaleTokens, requireXS bool, errs *[]string)
func validateShadows(s ShadowTokens, errs *[]string)
func validateLayout(l LayoutTokens, errs *[]string)
func isHexColor(s string) bool
func isCSSLength(s string) bool
func isCSSDuration(s string) bool
func isFontWeight(s string) bool
```

**IMPORTANT — `requireXS` usage:**
- Call `validateScale("spacing", t.Tokens.Spacing, true, &errs)` — spacing has `xs`
- Call `validateScale("radii", t.Tokens.Radii, false, &errs)` — radii has NO `xs` in `theme.json`; `XS` will be an empty string after JSON unmarshal and must NOT be validated when `requireXS=false`
- When `requireXS=false`, `validateScale` must skip validation of the `XS` field entirely — do not error on empty `XS`
- `validateLayout` must validate `sidebar_width` and `navbar_height` with `isCSSLength`
- `validateLayout` must validate `transition_speed` with `isCSSDuration`, accepting values like `160ms` or `0.2s`

**Verify:** `go build ./internal/theme/` compiles with no errors.

---

## TASK 05 — Create internal/theme/store.go

**Action:** Create `internal/theme/store.go`.

**Imports required:**
```go
import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sync"
)
```

**Store struct:**
```go
// Store manages the file-backed global theme document.
type Store struct {
    path string
    mu   sync.RWMutex
}
```

**Public API:**
```go
func NewStore(path string) *Store
func (s *Store) Load() (*Theme, error)
func (s *Store) Save(t *Theme) error
func (s *Store) Reset() (*Theme, error)
func (s *Store) ExportJSON() ([]byte, error)
func (s *Store) Path() string
```

**Behaviour:**
- `Load()` must avoid lock upgrading deadlocks:
  1. Acquire a read lock only while reading an existing file.
  2. If the file exists, release the read lock, unmarshal, validate, and return.
  3. If the file is missing, release the read lock before creating defaults, then call `Reset()`.
- `Save()` acquires a **write lock** (`s.mu.Lock()` / `s.mu.Unlock()`) for the entire operation including temp file write and rename.
- `Reset()` acquires a **write lock** for the entire operation and must not call `Save()` while holding that lock. Use an unexported helper such as `saveLocked(t *Theme) error` to avoid self-deadlock.
- `ExportJSON()` calls `Load()` internally — `Load()` acquires its own locks, so `ExportJSON()` must NOT hold any lock when calling `Load()`.
- `Load()` creates default `theme.json` if the file does not exist.
- `Load()` validates the theme before returning it.
- `Save()` validates before writing.
- `Save()` must create the parent directory before writing: `os.MkdirAll(filepath.Dir(s.path), 0755)`.
- `Save()` writes atomically: write to a temp file in the same directory named `filepath.Base(s.path) + ".tmp"`, then `os.Rename` to the final path — this is safe because `os.Rename` is atomic on the same filesystem.
- JSON output must be indented with two spaces.
- `Reset()` writes `DefaultTheme()` and returns the saved theme.
- `ExportJSON()` returns indented JSON bytes for the current theme.

**Verify:** `go build ./internal/theme/` compiles with no errors.

---

## TASK 06 — Create internal/theme/handler.go

**Action:** Create `internal/theme/handler.go`.

**Imports required:**
```go
import (
    "encoding/json"
    "net/http"

    "github.com/plomvix/plomvix/pkg/utils"
)
```

**Handler struct:**
```go
// Handler serves theme API endpoints.
type Handler struct {
    store *Store
}

// NewHandler creates a theme Handler.
func NewHandler(store *Store) *Handler {
    return &Handler{store: store}
}
```

**Handlers to implement:**
```go
func (h *Handler) GetTheme(w http.ResponseWriter, r *http.Request)
func (h *Handler) UpdateTheme(w http.ResponseWriter, r *http.Request)
func (h *Handler) ResetTheme(w http.ResponseWriter, r *http.Request)
func (h *Handler) ExportTheme(w http.ResponseWriter, r *http.Request)
```

**Behaviour:**
- `GetTheme` returns current theme with `utils.OK`
- `UpdateTheme` decodes a full `Theme`, validates/saves through store, returns saved theme
- `ResetTheme` resets to defaults, returns reset theme
- `ExportTheme` returns raw JSON with headers:
  ```go
  Content-Type: application/json
  Content-Disposition: attachment; filename="plomvix-theme.json"
  ```
- Validation errors return HTTP 400 with `CodeValidationFailed`
- File or encoding errors return HTTP 500

**Verify:** `go build ./internal/theme/` compiles with no errors.

---

## TASK 07 — Add ThemeConfig to internal/config/config.go

**Action:** Update `internal/config/config.go`.

**Change 1 — Add field to `Config`:**
```go
Theme ThemeConfig `mapstructure:"theme"`
```

**Change 2 — Add struct:**
```go
// ThemeConfig controls the file-backed theme engine.
type ThemeConfig struct {
    Path string `mapstructure:"path"`
}
```

**Change 3 — Add validation rule:**
- `theme.path` must not be empty
- Error message:
  ```text
  theme.path must not be empty
  ```

**Also update:** Any existing config tests, sample configs, or helper constructors that expect a fully valid `config.Config` must set `Theme.Path` to a non-empty value.

**Verify:** `go build ./internal/config/` compiles with no errors.

---

## TASK 08 — Update config.yaml with theme config

**Action:** Add this section after the existing `ui:` section if present, otherwise
after `logging:`:

```yaml
theme:
  path: ./theme.json
```

**Verify:**
```bash
grep -n "theme:" config.yaml
grep -n "path: ./theme.json" config.yaml
go test ./internal/config/...
```

---

## TASK 09 — Register theme routes in internal/server/server.go

**Action:** Update `internal/server/server.go`.

**Change 1 — Add imports:**
```go
themestore "github.com/plomvix/plomvix/internal/theme"
```

**Change 2 — Add field to `Server`:**
```go
themeStore *themestore.Store
```

**Change 3 — Update `New()` signature to accept theme store:**
```go
themeStore *themestore.Store
```

**Change 4 — Assign field in `New()` constructor.**

**Change 5 — In route setup:**
```go
themeHandler := themestore.NewHandler(s.themeStore)

// Public theme endpoint — no auth required so the UI can load before login exists.
r.Get("/api/theme", themeHandler.GetTheme)

// Admin-only theme endpoints — auth + admin role required.
// Use the same two-middleware pattern as existing /admin/* routes from Sprint 2:
// first auth.Middleware(...), then auth.RequireAdmin().
r.Group(func(r chi.Router) {
    r.Use(auth.Middleware(s.store, s.blacklist, s.cfg))
    r.Use(auth.RequireAdmin())
    r.Put("/api/theme",        themeHandler.UpdateTheme)
    r.Post("/api/theme/reset", themeHandler.ResetTheme)
    r.Get("/api/theme/export", themeHandler.ExportTheme)
})
```

**NOTE on field names:** The existing `Server` struct from Sprint 2 uses `s.store`
(not `s.authStore`) for the auth store field. Use `s.store` in all references above.

**Also update:** Every existing `server.New(...)` call site must pass `themeStore`, including `cmd/plomvix/main.go`, server tests, integration helpers from Sprint 10, and any other package-level test helpers. Do not leave any stale constructor calls.

**Verify:** `CGO_ENABLED=1 go build ./internal/server/` compiles with no errors.

---

## TASK 10 — Initialize theme store in cmd/plomvix/main.go

**Action:** Update `cmd/plomvix/main.go`.

**Change 1 — Add import:**
```go
themestore "github.com/plomvix/plomvix/internal/theme"
```

**Change 2 — After config and logger initialization, before opening WAL/hot/cold/auth stores:**
```go
themePath := cfg.Theme.Path
if themePath == "" {
    themePath = themestore.DefaultPath
}
themeStore := themestore.NewStore(themePath)
if _, err := themeStore.Load(); err != nil {
    logger.Error("failed to load theme", zap.Error(err))
    os.Exit(1)
}
```

Theme initialization must happen before resources with deferred cleanup are opened, because `os.Exit(1)` bypasses defers.

**Change 3 — Pass `themeStore` to `server.New(...)`.**

**Verify:** `CGO_ENABLED=1 go build ./cmd/plomvix/` compiles with no errors.

---

## TASK 11 — Create internal/theme/store_test.go

**Action:** Create `internal/theme/store_test.go`.

**Tests required:**
- `TestLoadCreatesDefaultThemeWhenMissing`
- `TestSaveAndLoadTheme`
- `TestResetRestoresDefaults`
- `TestExportJSONReturnsValidJSON`
- `TestSaveRejectsInvalidTheme`

**Verify:** `go test ./internal/theme/` passes.

---

## TASK 12 — Create internal/theme/validate_test.go

**Action:** Create `internal/theme/validate_test.go`.

**Tests required:**
- valid default theme passes
- invalid version fails
- invalid mode fails
- invalid hex color fails
- empty required color fails
- invalid CSS length fails
- invalid CSS duration fails for `layout.transition_speed`
- invalid font weight fails
- empty shadow fails
- validation returns multiple errors together

**Verify:** `go test ./internal/theme/` passes.

---

## TASK 13 — Create internal/theme/handler_test.go

**Action:** Create `internal/theme/handler_test.go` using `httptest`.

**Tests required:**
- `GET /api/theme` returns default theme
- `PUT /api/theme` saves a valid theme
- `PUT /api/theme` rejects invalid theme with 400
- `POST /api/theme/reset` resets changed theme
- `GET /api/theme/export` returns JSON and content-disposition header

**Verify:** `go test ./internal/theme/` passes.

---

## TASK 14 — Update api/openapi.json for theme endpoints

**Action:** Update `api/openapi.json`.

**Add schemas:**
- `Theme`
- `ThemeTokens`
- `ColorTokens`
- `TypographyTokens`
- `ScaleTokens`
- `ShadowTokens`
- `LayoutTokens`

**Add paths:**
- `GET /api/theme`
- `PUT /api/theme`
- `POST /api/theme/reset`
- `GET /api/theme/export`

**Security:**
- `GET /api/theme` → public `security: []`
- `GET /api/theme/export` → `[{"BearerAuth":[]},{"APIKeyAuth":[]}]` and description says admin role required
- `PUT /api/theme` → `[{"BearerAuth":[]},{"APIKeyAuth":[]}]` and description says admin role required
- `POST /api/theme/reset` → `[{"BearerAuth":[]},{"APIKeyAuth":[]}]` and description says admin role required

**Response note:** `GET /api/theme/export` returns raw `application/json` file content, not a Plomvix success envelope.

**Verify:**
```bash
cat api/openapi.json | python3 -m json.tool > /dev/null
! grep -R "\.\.\.\|TODO\|PLACEHOLDER" api/openapi.json
grep -n '"/api/theme"' api/openapi.json
grep -n '"/api/theme/reset"' api/openapi.json
grep -n '"/api/theme/export"' api/openapi.json
```

---

## TASK 15 — Create docs/api/theme.md

**Action:** Create `docs/api/theme.md`.

**Document:**
- Theme file location: `theme.json`
- Full theme JSON shape
- Validation rules
- `GET /api/theme`
- `PUT /api/theme`
- `POST /api/theme/reset`
- `GET /api/theme/export`
- Auth rules for public vs admin endpoints, including that `GET /api/theme/export` is admin-only
- Raw JSON download behaviour for `GET /api/theme/export`
- Frontend CSS variable mapping
- Dev panel enable/disable behaviour

**Verify:** `cat docs/api/theme.md` shows all endpoint sections and validation rules.

---

## TASK 16 — Update README.md with theme section

**Action:** Update `README.md`.

**Add:**
- Theme engine overview
- `theme.json` configuration note
- How to enable/disable developer design panel
- Link/reference to `docs/api/theme.md`

**Verify:**
```bash
grep -n "Theme" README.md
grep -n "theme.json" README.md
grep -n "Developer Design Panel" README.md
```

---

## TASK 17 — Create ui/src/theme/types.ts

**Action:** Create `ui/src/theme/types.ts` matching backend JSON exactly.

**NOTE on naming convention:** TypeScript interfaces below use `snake_case` property
names to exactly match the backend JSON keys. This avoids a serialization transform
layer. Add the following eslint disable comment at the top of the file to suppress
the camelCase naming rule:
```ts
/* eslint-disable @typescript-eslint/naming-convention */
```
If the project does not use eslint, no action needed — TypeScript itself does not
enforce naming conventions.

**Exports required:**
```ts
export type ThemeMode = 'light' | 'dark'
export interface Theme
export interface ThemeTokens
export interface ColorTokens
export interface TypographyTokens
export interface ScaleTokens
export interface ShadowTokens
export interface LayoutTokens
```

**Important:** TypeScript property names must match JSON snake_case keys exactly.

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 18 — Create ui/src/theme/defaultTheme.ts

**Action:** Create `ui/src/theme/defaultTheme.ts`.

**Requirements:**
- Export `defaultTheme: Theme`
- Values must match backend `DefaultTheme()` and root `theme.json`
- This is the frontend fallback when `/api/theme` fails

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 19 — Create ui/src/theme/cssVariables.ts

**Action:** Create `ui/src/theme/cssVariables.ts`.

**Exports required:**
```ts
export function applyTheme(theme: Theme, mode?: ThemeMode): void
export function themeToCSSVariables(theme: Theme, mode?: ThemeMode): Record<string, string>
```

**CSS variable names:**
```css
--plx-color-primary
--plx-color-secondary
--plx-color-accent
--plx-color-background
--plx-color-background-muted
--plx-color-surface
--plx-color-surface-muted
--plx-color-text
--plx-color-text-muted
--plx-color-border
--plx-color-error
--plx-color-warning
--plx-color-success
--plx-color-info
--plx-font-family
--plx-font-size-xs
--plx-font-size-sm
--plx-font-size-base
--plx-font-size-lg
--plx-font-size-xl
--plx-font-weight-regular
--plx-font-weight-medium
--plx-font-weight-semibold
--plx-font-weight-bold
--plx-radius-sm
--plx-radius-md
--plx-radius-lg
--plx-radius-xl
--plx-spacing-xs
--plx-spacing-sm
--plx-spacing-md
--plx-spacing-lg
--plx-spacing-xl
--plx-shadow-sm
--plx-shadow-md
--plx-shadow-lg
--plx-sidebar-width
--plx-navbar-height
--plx-transition-speed
```

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 20 — Update ui global CSS to consume CSS variables

**Action:** Update the existing global stylesheet from Sprint 11.

**Requirements:**
- Replace hardcoded shell colors with CSS variables
- `body` uses `--plx-color-background`, `--plx-color-text`, and `--plx-font-family`
- Sidebar width uses `--plx-sidebar-width`
- Navbar/header height uses `--plx-navbar-height`
- Buttons/cards/toasts use theme colors where applicable
- Keep fallback values in CSS, e.g. `var(--plx-color-primary, #2563eb)`

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 21 — Create ui/src/theme/api.ts

**Action:** Create `ui/src/theme/api.ts`.

**Exports required:**
```ts
export async function fetchTheme(): Promise<Theme>
export async function saveTheme(theme: Theme): Promise<Theme>
export async function resetTheme(): Promise<Theme>
export async function exportTheme(): Promise<Blob>
```

**Behaviour:**
- Use browser `fetch`.
- Include `Content-Type: application/json` for `PUT`.
- Use `credentials: 'same-origin'` for all requests to support future cookie auth.
- Throw descriptive `Error` messages on non-2xx responses.
- `fetchTheme`, `saveTheme`, and `resetTheme` read data from the existing Plomvix response envelope.
- `exportTheme` reads a raw `Blob` from `GET /api/theme/export`; it must not try to unwrap a Plomvix envelope.
- `saveTheme`, `resetTheme`, and `exportTheme` call admin endpoints and may return 401 until Sprint 13 adds browser login/cookie auth. Surface that as a normal error for the design panel to toast.

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 22 — Create ui/src/theme/ThemeContext.tsx

**Action:** Create `ui/src/theme/ThemeContext.tsx`.

**Exports required:**
```tsx
export function ThemeProvider({ children }: { children: React.ReactNode }): React.ReactElement
export function useTheme(): ThemeContextValue
```

**NOTE:** Use `React.ReactElement` as the return type, not `JSX.Element`.
`JSX.Element` is from the global `JSX` namespace and is considered legacy in
React 18 TypeScript strict mode. `React.ReactElement` is the correct modern type.

**ThemeContextValue:**
```ts
interface ThemeContextValue {
  theme: Theme           // last saved/loaded theme from backend
  draft: Theme           // current in-progress edits — editors read and write this
  mode: ThemeMode
  loading: boolean
  error: string | null
  setMode: (mode: ThemeMode) => void
  setDraftTheme: (theme: Theme) => void  // updates draft without saving
  saveDraft: () => Promise<void>         // persists draft to backend
  resetToDefault: () => Promise<void>    // resets both draft and saved to defaults
  reloadTheme: () => Promise<void>
}
```

**NOTE on `draft` vs `theme`:**
- `theme` is the last confirmed saved/loaded state from the backend
- `draft` is the live in-progress state that editors update via `setDraftTheme`
- On mount: `draft` is initialised to the loaded `theme`
- On save: `theme` is updated to match `draft`
- On reset: both `theme` and `draft` are set to defaults
- CSS variables are applied from `draft` (so live previews work immediately)
- Editors must read from `draft`, not `theme`

**Behaviour:**
- Load theme on mount
- Use `defaultTheme` while loading
- Apply CSS variables whenever draft or mode changes
- Emit app/toast events on load failure, save success/failure, reset success/failure
- `setMode()` updates local mode, updates `draft.mode`, and applies CSS variables immediately; it does not persist to disk until `saveDraft()` runs

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 23 — Wrap app with ThemeProvider

**Action:** Update `ui/src/main.tsx` (the entry file from Sprint 11).

**Requirement:** `ThemeProvider` must wrap `App` inside the existing provider tree.
The exact wrapping order must be:

```tsx
<BrowserRouter basename="/app">
  <AppEventProvider>          {/* Sprint 11 — must remain outermost */}
    <ThemeProvider>           {/* Sprint 12 — inside AppEventProvider so it can emit toasts */}
      <App />
    </ThemeProvider>
  </AppEventProvider>
</BrowserRouter>
```

`ThemeProvider` must be **inside** `AppEventProvider` because it uses `useAppEvents()`
to emit error toasts on theme load failure. Placing it outside `AppEventProvider`
will cause a runtime error from `useAppEvents()`.

Do not remove `StrictMode`, `BrowserRouter`, or `AppEventProvider` from Sprint 11.

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 24 — Create ui/src/components/ThemeModeToggle.tsx

**Action:** Create `ui/src/components/ThemeModeToggle.tsx`.

**Behaviour:**
- Uses `useTheme()`
- Shows current mode
- Toggles between `light` and `dark`
- Uses existing Button primitive
- Emits no backend request by itself; it only changes local mode

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 25 — Add ThemeModeToggle to app shell

**Action:** Update the existing app shell/header component.

**Requirement:**
- Add `ThemeModeToggle` in a top-right/header location
- Do not disturb existing sidebar or route rendering
- Use CSS variables for styling

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 26 — Create ui/src/pages/dev/DevDesignPage.tsx shell

**Action:** Create `ui/src/pages/dev/DevDesignPage.tsx` before registering the route.

**Initial content:**
- Page title: `Developer Design Panel`
- Short description
- Sections placeholders:
  - Colors
  - Typography
  - Spacing
  - Components Preview
  - Import / Export
- If `theme.dev_panel === false`, show disabled-state message

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 27 — Add dev design route metadata

**Action:** Update the Sprint 11 route registry `ui/src/app/routes.tsx`.

**Step 1 — Update the `AppRoute` type** to add optional fields needed by Sprint 12:
```ts
export type AppRoute = {
  path: string;
  label: string;
  element: React.ReactNode;  // keep as ReactNode, same as Sprint 11
  nav: boolean;
  devOnly?: boolean;         // ← ADD: if true, hidden unless theme.dev_panel === true
  group?: string;            // ← ADD: optional grouping label for sidebar sections
}
```

**Step 2 — Add the dev design route** to `appRoutes`:
```ts
{
  path: '/dev/design',
  label: 'Design Panel',
  element: <DevDesignPage />,   // use element: not component:
  nav: true,
  devOnly: true,
  group: 'Developer',
}
```

**NOTE:** Use `element: <DevDesignPage />` (instantiated JSX), NOT
`component: DevDesignPage` (uninstantiated). Sprint 11 used `element: ReactNode`
and this must stay consistent. Import `DevDesignPage` at the top of `routes.tsx`.

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 28 — Create ui/src/pages/dev/ColorEditor.tsx

**Action:** Create `ui/src/pages/dev/ColorEditor.tsx`.

**Behaviour:**
- Edit `tokens.colors` and `tokens.dark_colors`
- Use `<input type="color">` for hex colors
- Include text input beside each color input for exact hex editing
- Validate local color strings before updating draft
- Group light and dark colors separately

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 29 — Create ui/src/pages/dev/TypographyEditor.tsx

**Action:** Create `ui/src/pages/dev/TypographyEditor.tsx`.

**Behaviour:**
- Edit font family
- Edit font sizes
- Edit font weights
- Show live typography preview text using draft values

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 30 — Create ui/src/pages/dev/SpacingEditor.tsx

**Action:** Create `ui/src/pages/dev/SpacingEditor.tsx`.

**Behaviour:**
- Edit spacing scale
- Edit radii scale
- Edit layout sidebar width, navbar height, transition speed
- Show visual spacing blocks and radius preview boxes

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 31 — Create ui/src/pages/dev/ShadowEditor.tsx

**Action:** Create `ui/src/pages/dev/ShadowEditor.tsx`.

**Behaviour:**
- Edit `sm`, `md`, and `lg` shadow strings
- Show cards previewing each shadow
- Keep editing as text inputs because shadows are free-form CSS values

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 32 — Create ui/src/pages/dev/ComponentPreview.tsx

**Action:** Create `ui/src/pages/dev/ComponentPreview.tsx`.

**Component previews required:**
- Button
- Input
- Table row
- Card
- Badge
- Chart placeholder
- Modal mockup
- Sidebar item
- Navbar item

**Important:** This file is preview-only. Do not introduce modal state management or chart dependencies.
Use plain markup styled with CSS variables.

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 33 — Create ui/src/pages/dev/ImportExportPanel.tsx

**Action:** Create `ui/src/pages/dev/ImportExportPanel.tsx`.

**Behaviour:**
- Export button downloads the current local `draft` theme as `plomvix-theme.json` without requiring a backend call. This keeps local export usable before Sprint 13 auth UI exists.
- Import accepts a local `.json` file.
- Imported JSON is parsed as a `Theme` object and applied to draft only.
- Import does not save automatically.
- Invalid JSON emits error toast.
- Do not use the admin-only backend `GET /api/theme/export` for the default panel export button in Sprint 12; keep that API available for future authenticated workflows.

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 34 — Wire editors into DevDesignPage

**Action:** Update `ui/src/pages/dev/DevDesignPage.tsx`.

**Requirements:**
- Render all editor sections
- Maintain a draft theme through `ThemeContext`
- Add Save button wired to `saveDraft()`
- Add Reset button wired to `resetToDefault()`
- Add local Light/Dark preview toggle wired to `setMode()`
- Show loading and error states
- If Save or Reset returns 401 because Sprint 13 auth UI does not exist yet, show a clear toast such as `Admin login required to save theme`. Do not add a temporary token field or hardcoded credential workaround.

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 35 — Add frontend theme tests if test runner exists

**Action:** If Sprint 11 added a UI test runner, add tests for:
- `themeToCSSVariables()` maps light colors
- `themeToCSSVariables()` maps dark colors
- design route is hidden when `dev_panel` is false
- import rejects malformed JSON

If no UI test runner exists yet, do not add a new dependency in Sprint 12. Instead,
add a short note to `docs/api/theme.md` saying UI tests are deferred until the UI
test stack is introduced.

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

If tests exist:
```bash
cd ui
npm test -- --run
```

---

## TASK 36 — Update sidebar route filtering for dev panel

**Action:** Update the sidebar component from Sprint 11.

**Requirements:**
- Read `theme.dev_panel` from `useTheme()`
- Filter out route entries with `devOnly === true` when `dev_panel` is false
- Preserve existing route ordering and grouping
- Do not hardcode `/dev/design` in sidebar component; use route metadata only

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 37 — Update full app build flow if needed

**Action:** Confirm existing Sprint 11 Makefile targets still build the UI and Go binary.

If the UI build target does not run type checking, update it to run:
```makefile
cd ui && npm run typecheck && npm run build
```

**Verify:**
```bash
make ui-build
CGO_ENABLED=1 make build
```

---

## TASK 38 — Add backend integration smoke test for theme endpoints

**Action:** Add theme checks to existing integration tests if available, or create
`tests/integration/theme_test.go` if Sprint 10 integration tests exist.

**Test flow:**
1. Start test server
2. `GET /api/theme` without auth returns 200
3. `GET /api/theme/export` without auth returns 401
4. `PUT /api/theme` without auth returns 401
5. Login as admin
6. `GET /api/theme/export` with admin token returns 200 raw JSON
7. `PUT /api/theme` with admin token saves a changed primary color
8. `GET /api/theme` returns changed color
9. `POST /api/theme/reset` with admin token restores default primary color

**Verify:**
```bash
CGO_ENABLED=1 go test -race ./tests/integration/...
```

If integration test scaffolding is not present, document this as deferred in
`docs/api/theme.md` and rely on `internal/theme` unit tests for Sprint 12.

---

## TASK 39 — Run Go formatting and backend tests

**Action:**
```bash
find . -name '*.go' -not -path './vendor/*' -exec gofmt -w {} +
CGO_ENABLED=1 go test ./internal/theme/...
CGO_ENABLED=1 go test ./internal/config/...
CGO_ENABLED=1 go test ./internal/server/...
CGO_ENABLED=1 go test ./...
```

**Verify:** All commands exit with code 0.

---

## TASK 40 — Run UI verification

**Action:**
```bash
cd ui
npm install
npm run typecheck
npm run build
```

**Verify:**
- TypeScript exits with zero errors
- Vite build exits with zero errors
- `ui/dist/` exists

---

## TASK 41 — Full build and smoke test

**Action:** Run the following verification sequence from project root:

```bash
#!/bin/bash
set -euo pipefail

SERVER_PID=""
cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -SIGTERM "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "=== Step 1: Backend + UI build ==="
CGO_ENABLED=1 make build

echo "=== Step 2: Run tests ==="
CGO_ENABLED=1 make test

echo "=== Step 3: Boot server ==="
./plomvix > /tmp/plomvix_s12.log 2>&1 &
SERVER_PID=$!
sleep 3

echo "=== Step 4: Public theme endpoint ==="
curl -sf http://localhost:8080/api/theme | jq '.data.version' | grep -q '^1$' \
    && echo "PASS: public theme endpoint works" \
    || { echo "FAIL: public theme endpoint failed"; exit 1; }

echo "=== Step 5: Theme export requires auth ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    http://localhost:8080/api/theme/export)
[ "$STATUS" -eq 401 ] && echo "PASS: export without auth rejected" \
    || { echo "FAIL: expected 401 for export without auth, got $STATUS"; exit 1; }

echo "=== Step 6: Login as admin ==="
TOKEN=$(curl -sf -X POST http://localhost:8080/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"changeme"}' \
    | jq -r '.data.token')
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] \
    && echo "PASS: admin login works" \
    || { echo "FAIL: admin login failed"; exit 1; }

echo "=== Step 7: Theme export endpoint with admin token ==="
curl -sf -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/theme/export | jq '.version' | grep -q '^1$' \
    && echo "PASS: authenticated theme export works" \
    || { echo "FAIL: authenticated theme export failed"; exit 1; }

echo "=== Step 8: UI app loads ==="
curl -sf http://localhost:8080/app/ | grep -qi "plomvix" \
    && echo "PASS: UI app loads" \
    || { echo "FAIL: UI app did not load"; exit 1; }

echo "=== Step 9: Mutating theme endpoint requires auth ==="
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST http://localhost:8080/api/theme/reset)
[ "$STATUS" -eq 401 ] && echo "PASS: reset without auth rejected" \
    || { echo "FAIL: expected 401 for reset without auth, got $STATUS"; exit 1; }

echo "Sprint 12 smoke test PASSED"
```

**Verify:** Script completes with `Sprint 12 smoke test PASSED`.

---

## TASK 42 — Final lint and repository check

**Action:**
```bash
CGO_ENABLED=1 make lint
CGO_ENABLED=1 make build
git status --short
```

**Verify:**
- `make lint` exits with code 0
- `make build` exits with code 0
- `git status --short` shows only intentional Sprint 12 files changed

---

## FINAL SPRINT 12 ACCEPTANCE CHECKLIST

- `theme.json` exists and is valid JSON
- `internal/theme/` exists and has tests
- `GET /api/theme` works without auth
- `GET /api/theme/export` requires admin auth and returns raw downloadable JSON
- `PUT /api/theme` requires admin auth and saves a complete valid theme
- `POST /api/theme/reset` requires admin auth and restores defaults
- `config.yaml` includes `theme.path`
- Go server initializes the theme store at startup
- UI loads theme on startup
- CSS variables are injected on `document.documentElement`
- Light/dark toggle works locally
- `/dev/design` is route-registry driven
- Sidebar shows `/dev/design` only when `theme.dev_panel === true`
- Developer Design Panel supports color, typography, spacing, shadow, layout editing
- Component previews exist for required primitive examples
- Import/export works in the design panel
- Save/reset emit toast events
- OpenAPI includes all theme endpoints
- `docs/api/theme.md` exists
- `README.md` mentions theme engine
- `find ... -exec gofmt -w {} +` has been used
- `CGO_ENABLED=1 make test` passes
- `make ui-build` passes
- `CGO_ENABLED=1 make build` passes
- `CGO_ENABLED=1 make lint` passes
