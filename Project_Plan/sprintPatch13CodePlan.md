# Plomvix — Sprint 13 Patch Code Plan
### For: DeepSeek V4 Pro Coding Agent
### Language: TypeScript | React 18 | Vite | Module: github.com/plomvix/plomvix

> Execute tasks in exact order. Each task is atomic — one file or one concern.
> Do not skip ahead. Each task depends on the previous.
> Every task has a Verify step — do not proceed until it passes.

---

## CONTEXT

Sprints 1–13 are complete and Sprint 13 has already been coded. Sprint 11 added the
UI foundation, Sprint 12 added the Theme Engine + Developer Design Panel, and
Sprint 13 added cookie auth, login/logout UI, protected routes, auth provider, and
the centralized frontend API client.

This patch sprint migrates the already-coded Sprint 11–13 browser UI to
**Tailwind CSS v4 + shadcn/ui** without changing product behavior.

This is a **patch code plan**, not a new feature sprint. Its purpose is to avoid
building future UI screens on the old custom-CSS-only foundation.

**What this patch delivers:**
- Tailwind CSS v4 installed through the Vite plugin
- shadcn/ui initialized in the existing `ui/` Vite React app
- `@/*` import alias configured for TypeScript and Vite
- `cn()` helper added for class composition
- shadcn-compatible CSS variables wired to existing Plomvix theme tokens
- Existing Sprint 11 app shell migrated to Tailwind utilities
- Existing Sprint 11 sidebar/header/placeholder pages migrated to Tailwind utilities
- Existing Sprint 11 toast/event UI styled with Tailwind while preserving the event system
- Existing Sprint 12 ThemeProvider and Design Panel migrated to Tailwind/shadcn primitives
- Existing Sprint 13 login/logout/protected-route UI migrated to Tailwind/shadcn primitives
- Existing custom Button primitive replaced with a compatibility wrapper around shadcn Button
- No localStorage JWT storage introduced
- No backend route changes
- No new product features
- Full UI typecheck/build verification

**What this patch does NOT do:**
- No Admin UI — shifted to the next sprint after this patch
- No Log Explorer UI
- No trace UI
- No OTLP or Prometheus UI
- No backend auth rewrite
- No API contract changes
- No replacement of the Sprint 11 app event/toast system with Sonner
- No Bootstrap, MUI, Chakra, Ant Design, or other UI frameworks
- No new theme source of truth — `theme.json` and Sprint 12 CSS variables remain authoritative

---

## TAILWIND + SHADCN DESIGN — READ BEFORE WRITING ANY CODE

The existing Plomvix theme engine remains the source of truth.
Tailwind and shadcn/ui are implementation tools, not a replacement theme system.

**Theme authority rule:**
```
theme.json → ThemeProvider → --plx-* CSS variables → shadcn-compatible CSS variables → Tailwind/shadcn components
```

**Important:** Do not hardcode a second color system in React components. Components
must use Tailwind classes that resolve to shadcn variables such as `bg-background`,
`text-foreground`, `border-border`, `bg-primary`, and `text-muted-foreground`.
Those variables must be aliases of existing `--plx-*` variables.

**Pinned dependency rule:**
Do not use `latest`, `^`, or `~` in `package.json` for new dependencies. Use exact
versions. The exact versions below are the versions to use for this patch:

```json
{
  "tailwindcss": "4.3.0",
  "@tailwindcss/vite": "4.3.0",
  "shadcn": "4.8.3",
  "class-variance-authority": "0.7.1",
  "clsx": "2.1.1",
  "tailwind-merge": "3.6.0",
  "lucide-react": "1.17.0",
  "tw-animate-css": "1.4.0"
}
```

If shadcn CLI adds Radix packages with ranges, pin the exact resolved versions from
`package-lock.json` back into `package.json` before finishing the patch.

---

## EXISTING UI PRESERVATION RULES

Preserve these behaviours from Sprints 11–13:

- `BrowserRouter basename="/app"` stays unchanged
- Main app routes stay registered without `/app` prefix
- `/login` and `/logout` continue to work as top-level browser routes
- Protected route behaviour remains unchanged
- Cookie auth remains httpOnly-cookie based
- API client keeps `credentials: "include"`
- Session-expired event behaviour remains unchanged
- ThemeProvider still loads `/api/theme`
- `theme.dev_panel` still controls visibility of the Design Panel route
- Light/dark mode still uses Sprint 12 theme mode state
- Existing toast/event system remains the only toast system

---

## TASK 01 — Baseline verify current Sprint 13 UI

**Action:** From the project root, verify the current coded Sprint 13 state before
making migration changes.

```bash
cd ui
npm install
npm run typecheck
npm run build
cd ..
CGO_ENABLED=1 make build
```

**Verify:** All commands exit with code 0 before starting this patch.

---

## TASK 02 — Install Tailwind and shadcn dependencies with exact versions

**Action:** In `ui/`, install the required dependencies with exact versions.

```bash
cd ui
npm install --save-dev --save-exact tailwindcss@4.3.0 @tailwindcss/vite@4.3.0 shadcn@4.8.3
npm install --save --save-exact class-variance-authority@0.7.1 clsx@2.1.1 tailwind-merge@3.6.0 lucide-react@1.17.0 tw-animate-css@1.4.0
```

If `@types/node` is not already installed, install an exact version already compatible
with the existing Vite setup. If it already exists, do not change it.

**Verify:**
```bash
cd ui
node -e "const p=require('./package.json'); const all={...p.dependencies,...p.devDependencies}; for (const k of ['tailwindcss','@tailwindcss/vite','shadcn','class-variance-authority','clsx','tailwind-merge','lucide-react','tw-animate-css']) { if (!all[k]) throw new Error(k+' missing'); if (/^[~^]|latest/.test(all[k])) throw new Error(k+' is not pinned: '+all[k]); }"
npm run typecheck
```

---

## TASK 03 — Configure Vite for Tailwind v4 and @ alias

**Action:** Update `ui/vite.config.ts`.

**Requirements:**
- Import `tailwindcss` from `@tailwindcss/vite`
- Add `tailwindcss()` to the Vite `plugins` array after `react()`
- Add or preserve `resolve.alias["@"] = path.resolve(__dirname, "./src")`
- Do not remove the Sprint 13 dev proxy rules for API/UI routes
- Do not change the React plugin configuration unless required by the existing file

**Example shape:**
```ts
import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  // keep existing server/proxy config
})
```

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 04 — Configure TypeScript @ alias

**Action:** Update TypeScript config files in `ui/`.

**Requirements:**
- Add `baseUrl: "."`
- Add `paths: { "@/*": ["./src/*"] }`
- Apply this to `tsconfig.json` and `tsconfig.app.json` if both exist
- Preserve existing strict mode settings from Sprint 11–13
- Do not loosen TypeScript strictness

**Verify:**
```bash
cd ui
npx tsc --showConfig | grep '"@/\*"' -n || true
npm run typecheck
```

---

## TASK 05 — Add shadcn components.json

**Action:** Create `ui/components.json` if it does not already exist.

**Full file content:**
```json
{
  "$schema": "https://ui.shadcn.com/schema.json",
  "style": "new-york",
  "rsc": false,
  "tsx": true,
  "tailwind": {
    "config": "",
    "css": "src/index.css",
    "baseColor": "neutral",
    "cssVariables": true,
    "prefix": ""
  },
  "aliases": {
    "components": "@/components",
    "utils": "@/lib/utils",
    "ui": "@/components/ui",
    "lib": "@/lib",
    "hooks": "@/hooks"
  },
  "iconLibrary": "lucide"
}
```

**Verify:**
```bash
cd ui
cat components.json | python3 -m json.tool > /dev/null
grep -n '"ui": "@/components/ui"' components.json
```

---

## TASK 06 — Create cn utility helper

**Action:** Create `ui/src/lib/utils.ts`.

**Full file content:**
```ts
import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}
```

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 07 — Update global CSS for Tailwind and shadcn variable bridge

**Action:** Update the existing global stylesheet used by `ui/src/main.tsx`.
This is usually `ui/src/index.css`.

**Requirements:**
- Add Tailwind v4 import at the top:
  ```css
  @import "tailwindcss";
  @import "tw-animate-css";
  ```
- Add dark custom variant:
  ```css
  @custom-variant dark (&:is(.dark *));
  ```
- Add shadcn `@theme inline` variables that point to shadcn aliases
- Add shadcn aliases that point to Plomvix `--plx-*` variables
- Preserve any required Sprint 11–13 global reset styles if still needed
- Do not delete the existing Plomvix theme CSS variables

**Required alias block:**
```css
@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-destructive: var(--destructive);
  --color-destructive-foreground: var(--destructive-foreground);
  --color-border: var(--border);
  --color-input: var(--input);
  --color-ring: var(--ring);
  --radius-sm: calc(var(--radius) - 4px);
  --radius-md: calc(var(--radius) - 2px);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) + 4px);
}

:root {
  --background: var(--plx-color-background, #ffffff);
  --foreground: var(--plx-color-text, #0f172a);
  --card: var(--plx-color-surface, #ffffff);
  --card-foreground: var(--plx-color-text, #0f172a);
  --popover: var(--plx-color-surface, #ffffff);
  --popover-foreground: var(--plx-color-text, #0f172a);
  --primary: var(--plx-color-primary, #2563eb);
  --primary-foreground: #ffffff;
  --secondary: var(--plx-color-secondary, #64748b);
  --secondary-foreground: #ffffff;
  --muted: var(--plx-color-surface-muted, #f1f5f9);
  --muted-foreground: var(--plx-color-text-muted, #64748b);
  --accent: var(--plx-color-accent, #14b8a6);
  --accent-foreground: #ffffff;
  --destructive: var(--plx-color-error, #dc2626);
  --destructive-foreground: #ffffff;
  --border: var(--plx-color-border, #e2e8f0);
  --input: var(--plx-color-border, #e2e8f0);
  --ring: var(--plx-color-primary, #2563eb);
  --radius: var(--plx-radius-md, 0.5rem);
}

* {
  border-color: var(--border);
}

body {
  margin: 0;
  min-width: 320px;
  min-height: 100vh;
  background: var(--background);
  color: var(--foreground);
  font-family: var(--plx-font-family, Inter, ui-sans-serif, system-ui, sans-serif);
}
```

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 08 — Update ThemeProvider to toggle dark class

**Action:** Update `ui/src/theme/ThemeContext.tsx` or the file that manages Sprint 12 theme mode.

**Requirements:**
- When `mode === "dark"`, add `dark` class to `document.documentElement`
- When `mode === "light"`, remove `dark` class from `document.documentElement`
- Also set `document.documentElement.dataset.theme = mode`
- Keep existing `applyTheme(theme, mode)` behaviour
- Do not persist local-only mode unless the existing Sprint 12 save flow explicitly saves the draft

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 09 — Add shadcn UI components with pinned CLI

**Action:** In `ui/`, run shadcn with the pinned CLI version.

```bash
cd ui
npx shadcn@4.8.3 add button input label card alert dialog dropdown-menu separator skeleton badge table tabs textarea tooltip
```

**Requirements:**
- Components must be generated under `ui/src/components/ui/`
- Do not use `npx shadcn@latest`
- If the CLI asks to overwrite existing local files, do not overwrite custom Sprint 11–13 files outside `src/components/ui/`
- If the CLI adds package ranges to `package.json`, pin them to exact resolved versions from `package-lock.json`

**Verify:**
```bash
cd ui
test -f src/components/ui/button.tsx
test -f src/components/ui/input.tsx
test -f src/components/ui/card.tsx
test -f src/components/ui/alert.tsx
test -f src/components/ui/dialog.tsx
test -f src/components/ui/table.tsx
node -e "const p=require('./package.json'); for (const group of ['dependencies','devDependencies']) for (const [k,v] of Object.entries(p[group]||{})) if (/^[~^]|latest/.test(v)) throw new Error(k+' not pinned: '+v)"
npm run typecheck
```

---

## TASK 10 — Normalize generated shadcn imports and formatting

**Action:** Review generated files in `ui/src/components/ui/`.

**Requirements:**
- Generated components must import `cn` from `@/lib/utils`
- No import should point to a non-existent alias
- Components must use `React.ComponentProps` or equivalent strict TypeScript-safe types
- Do not manually rewrite generated component internals unless build/typecheck fails
- Format generated files using the existing project formatter if one exists

**Verify:**
```bash
cd ui
grep -R "@/lib/utils" src/components/ui
npm run typecheck
npm run build
```

---

## TASK 11 — Replace existing custom Button with compatibility wrapper

**Action:** Update the existing Sprint 11 Button primitive file, likely
`ui/src/components/Button.tsx`.

**Requirement:** Preserve the old import path while delegating to shadcn Button.
This prevents Sprint 11–13 pages from breaking.

**Implementation guidance:**
- Import `Button as ShadcnButton` from `@/components/ui/button`
- Export `Button` from the old file path
- Preserve the old public props if they differ from shadcn props
- Map old variants to shadcn variants:
  - primary/default → `default`
  - secondary → `secondary`
  - ghost → `ghost`
  - danger/destructive → `destructive`
- Keep `className` passthrough
- Keep disabled/loading behavior if the old Button had it

**Verify:**
```bash
cd ui
grep -R "from ['\"]@/components/Button" -n src || true
grep -R "from ['\"]\.\./components/Button" -n src || true
npm run typecheck
```

---

## TASK 12 — Migrate app shell layout to Tailwind classes

**Action:** Update the Sprint 11 app shell/layout component.

**Requirements:**
- Replace shell-specific custom CSS class styling with Tailwind classes
- Use shadcn variables via Tailwind classes:
  - `bg-background`
  - `text-foreground`
  - `border-border`
  - `bg-card`
  - `text-muted-foreground`
- Preserve route rendering exactly
- Preserve `BrowserRouter basename="/app"` from `main.tsx`
- Do not change protected-route behaviour from Sprint 13

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 13 — Migrate sidebar to Tailwind classes

**Action:** Update the Sprint 11 sidebar component.

**Requirements:**
- Use Tailwind classes for layout, spacing, borders, active route state, hover state
- Use `--plx-sidebar-width` through Tailwind arbitrary value or CSS variable:
  ```tsx
  className="w-[var(--plx-sidebar-width)]"
  ```
- Keep route registry driven rendering
- Keep Sprint 12 `devOnly` filtering via `theme.dev_panel`
- Keep Sprint 13 auth-aware/admin-aware filtering if already implemented
- Do not hardcode `/dev/design`, `/admin`, or any route path in the sidebar

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 14 — Migrate header/navbar to Tailwind classes

**Action:** Update the Sprint 11 app header/navbar component.

**Requirements:**
- Use Tailwind classes for height, borders, background, spacing, alignment
- Use `h-[var(--plx-navbar-height)]` for navbar height
- Keep ThemeModeToggle from Sprint 12
- Keep auth/logout controls from Sprint 13 if present
- Do not add admin-specific UI here

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 15 — Migrate placeholder pages to Tailwind/shadcn Card

**Action:** Update Sprint 11 placeholder pages.

**Requirements:**
- Use `Card`, `CardHeader`, `CardTitle`, `CardDescription`, and `CardContent`
- Replace custom placeholder CSS with Tailwind classes
- Keep page text/content semantics unchanged
- Do not add product functionality

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 16 — Migrate toast UI styling while preserving event system

**Action:** Update the Sprint 11 toast components.

**Requirements:**
- Preserve existing app event/toast provider API
- Do not install or use Sonner in this patch
- Style toast viewport and toast cards with Tailwind classes
- Use semantic variants for success/error/warning/info with Plomvix variables
- Keep request/session/auth toasts from Sprint 13 working

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 17 — Migrate ThemeModeToggle to shadcn Button

**Action:** Update `ui/src/components/ThemeModeToggle.tsx`.

**Requirements:**
- Use shadcn Button or the compatibility Button wrapper
- Use `lucide-react` icons if appropriate, for example `Sun` and `Moon`
- Preserve local-only toggle behaviour from Sprint 12
- Do not trigger backend save from this component

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 18 — Migrate Design Panel page shell to shadcn layout primitives

**Action:** Update `ui/src/pages/dev/DevDesignPage.tsx`.

**Requirements:**
- Use Tailwind grid/flex classes for layout
- Use shadcn `Card`, `Tabs`, `Button`, `Alert`, and `Separator` where useful
- Preserve all existing sections:
  - Colors
  - Typography
  - Spacing/Layout
  - Shadows
  - Components Preview
  - Import/Export
- Preserve disabled-state when `theme.dev_panel === false`
- Preserve Save/Reset behaviour and toast events

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 19 — Migrate ColorEditor to shadcn inputs/cards

**Action:** Update `ui/src/pages/dev/ColorEditor.tsx`.

**Requirements:**
- Use shadcn `Card`, `Input`, `Label`, and `Alert` where useful
- Keep `<input type="color">` for visual color picking
- Keep text input for exact hex editing
- Preserve local validation before updating draft
- Preserve immutable nested updates to `draft.tokens.colors` and `draft.tokens.dark_colors`
- Preserve `#rgb` normalization if Sprint 12 implemented it

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 20 — Migrate TypographyEditor to shadcn inputs/cards

**Action:** Update `ui/src/pages/dev/TypographyEditor.tsx`.

**Requirements:**
- Use shadcn `Input`, `Label`, `Card`, and `Separator`
- Preserve font family, font sizes, and font weight editing
- Preserve live preview text
- Preserve immutable draft updates

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 21 — Migrate SpacingEditor and ShadowEditor

**Action:** Update `ui/src/pages/dev/SpacingEditor.tsx` and
`ui/src/pages/dev/ShadowEditor.tsx`.

**Requirements:**
- Use shadcn `Input`, `Label`, `Card`, and `Separator`
- Preserve spacing/radii/layout/shadow editing
- Preserve transition speed editing
- Preserve visual previews
- Preserve immutable draft updates

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 22 — Migrate ComponentPreview to shadcn primitives

**Action:** Update `ui/src/pages/dev/ComponentPreview.tsx`.

**Requirements:**
- Show previews using actual shadcn components where available:
  - Button
  - Input
  - Table
  - Card
  - Badge
  - Dialog mockup markup
  - Alert
  - Skeleton
- Keep chart placeholder as plain markup; do not add chart dependencies
- Keep modal preview as static/mockup unless existing component already supports state
- Do not add admin/log-explorer-specific previews

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 23 — Migrate ImportExportPanel to shadcn primitives

**Action:** Update `ui/src/pages/dev/ImportExportPanel.tsx`.

**Requirements:**
- Use shadcn `Button`, `Input`, `Card`, and `Alert`
- Preserve draft-only import behaviour
- Preserve local JSON validation before applying imported theme
- Preserve local draft export behaviour if backend export is admin-only
- Do not require Sprint 13 login just to export the current local draft

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 24 — Migrate login page to shadcn form primitives

**Action:** Update the Sprint 13 login page.

**Requirements:**
- Use shadcn `Card`, `Input`, `Label`, `Button`, and `Alert`
- Preserve username/password fields
- Preserve inline error states
- Preserve submit behaviour against `POST /auth/login`
- Preserve redirect on success
- Do not store JWT in localStorage or sessionStorage
- Keep cookie auth behaviour unchanged

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 25 — Migrate logout route UI

**Action:** Update the Sprint 13 logout route/page.

**Requirements:**
- Use Tailwind/shadcn loading and error states
- Use `Card` and `Button` where appropriate
- Preserve call to `POST /auth/logout`
- Preserve redirect to `/login`
- Preserve idempotent behaviour if already implemented

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 26 — Migrate protected-route fallback UI

**Action:** Update the Sprint 13 protected route wrapper/loading fallback.

**Requirements:**
- Use shadcn `Skeleton` for loading state
- Use shadcn `Alert` for auth/session errors if a visible error exists
- Preserve redirect rules exactly
- Do not add new auth routes

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 27 — Migrate API error display components

**Action:** Update any Sprint 13 API/auth error display components.

**Requirements:**
- Use shadcn `Alert` for visible API/auth errors
- Preserve centralized API client behaviour
- Preserve session-expired event emission/listening semantics
- Do not add duplicate toast emission

**Verify:** `cd ui && npm run typecheck` passes.

---

## TASK 28 — Remove or shrink obsolete custom CSS

**Action:** Review existing custom CSS files from Sprints 11–13.

**Requirements:**
- Remove CSS that has been replaced by Tailwind classes
- Keep only global reset, CSS variables, and unavoidable shared styles
- Do not remove CSS needed by the theme variable bridge
- Do not leave dead class names that are no longer referenced

**Verify:**
```bash
cd ui
npm run typecheck
npm run build
```

---

## TASK 29 — Add a UI dependency audit script or npm script

**Action:** Add an npm script in `ui/package.json` named `check:deps`.

**Script behaviour:**
It must fail if any dependency/devDependency value starts with `^`, `~`, or equals
`latest`.

**Example script:**
```json
{
  "scripts": {
    "check:deps": "node ./scripts/check-deps-pinned.mjs"
  }
}
```

Create `ui/scripts/check-deps-pinned.mjs`:
```js
import fs from 'node:fs'

const pkg = JSON.parse(fs.readFileSync(new URL('../package.json', import.meta.url), 'utf8'))
const groups = ['dependencies', 'devDependencies', 'optionalDependencies']
let failed = false
for (const group of groups) {
  for (const [name, version] of Object.entries(pkg[group] ?? {})) {
    if (typeof version === 'string' && (/^[~^]/.test(version) || version === 'latest')) {
      console.error(`${group}.${name} is not pinned: ${version}`)
      failed = true
    }
  }
}
if (failed) process.exit(1)
```

**Verify:**
```bash
cd ui
npm run check:deps
```

---

## TASK 30 — Add utility tests for cn and theme variable mapping if test runner exists

**Action:** If Sprint 11–13 already includes a UI test runner, add tests for:
- `cn()` merges conflicting Tailwind classes correctly
- Theme mode toggles `document.documentElement.classList` with `dark`
- shadcn alias variables exist in computed/global CSS setup where feasible

If no UI test runner exists, do not add one in this patch. Document the deferred UI
test coverage in `docs/api/theme.md` or the existing UI docs.

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

## TASK 31 — Update Sprint 12 theme docs for Tailwind/shadcn bridge

**Action:** Update `docs/api/theme.md` or the existing theme documentation.

**Document:**
- `theme.json` remains the source of truth
- Tailwind/shadcn consume theme through CSS variables
- `--plx-*` variables are mapped to shadcn variables such as `--background`, `--foreground`, `--primary`, and `--border`
- Light/dark mode toggles the `dark` class on `document.documentElement`
- Do not edit shadcn component CSS variables manually outside the theme bridge

**Verify:**
```bash
grep -n "Tailwind" docs/api/theme.md
grep -n "shadcn" docs/api/theme.md
grep -n "--plx" docs/api/theme.md
```

---

## TASK 32 — Update README UI stack section

**Action:** Update `README.md`.

**Add or update:**
- UI stack: Vite + React 18 + TypeScript + Tailwind CSS + shadcn/ui
- Theme system note: Plomvix tokens remain source of truth
- No Bootstrap note if a UI-stack comparison section exists
- Developer command reminders: `make ui-build`, `cd ui && npm run check:deps`

**Verify:**
```bash
grep -n "Tailwind" README.md
grep -n "shadcn" README.md
grep -n "Vite" README.md
```

---

## TASK 33 — Run full UI verification

**Action:** Run the complete UI verification sequence.

```bash
cd ui
npm run check:deps
npm run typecheck
npm run build
```

If a test runner exists:
```bash
npm test -- --run
```

**Verify:** All commands exit with code 0 and `ui/dist/` exists.

---

## TASK 34 — Run full repository verification

**Action:** From the project root:

```bash
CGO_ENABLED=1 make build
CGO_ENABLED=1 make test
```

If `make lint` exists and is expected to pass at this point:
```bash
CGO_ENABLED=1 make lint
```

**Verify:** All applicable commands exit with code 0.

---

## TASK 35 — Run browser smoke test manually

**Action:** Run the app and manually check UI routes.

```bash
CGO_ENABLED=1 make build
./plomvix
```

Open the app in a browser and verify:
- `/login` loads and is styled with Tailwind/shadcn components
- Login still works
- Protected `/app/` redirects unauthenticated users to `/login`
- Authenticated `/app/` loads the app shell
- Sidebar and header render with Tailwind styling
- Theme mode toggle still works
- `/app/dev/design` still loads when `theme.dev_panel === true`
- Design Panel editors still update the live draft preview
- Toasts still appear and are styled correctly
- No duplicate toast systems appear

**Verify:** All manual checks pass.

---

## TASK 36 — Final package and source audit

**Action:** Run final audit commands.

```bash
cd ui
npm run check:deps
! grep -R "bootstrap\|@mui\|antd\|chakra" package.json src || true
! grep -R "shadcn@latest\|latest" package.json package-lock.json src components.json || true
cd ..
git status --short
```

**Verify:**
- No Bootstrap/MUI/Ant/Chakra dependency was introduced
- No `latest`, `^`, or `~` remains in `ui/package.json`
- `package-lock.json` is updated intentionally
- `git status --short` shows only intentional patch files changed

---

## FINAL SPRINT 13 PATCH ACCEPTANCE CHECKLIST

- Tailwind CSS v4 is installed with exact pinned versions
- `@tailwindcss/vite` is configured in `vite.config.ts`
- `@/*` alias works in Vite and TypeScript
- shadcn `components.json` exists
- shadcn components exist under `ui/src/components/ui/`
- `cn()` helper exists at `ui/src/lib/utils.ts`
- `ui/package.json` contains no `latest`, `^`, or `~` for dependencies/devDependencies
- `npm run check:deps` passes
- Plomvix `--plx-*` theme variables map to shadcn variables
- `ThemeProvider` toggles `document.documentElement.classList` with `dark`
- Existing Sprint 11 app shell is migrated to Tailwind classes
- Existing Sprint 11 sidebar/header are migrated to Tailwind classes
- Existing Sprint 11 toast system is preserved and styled with Tailwind
- Existing Sprint 12 Theme Design Panel is migrated to Tailwind/shadcn primitives
- Existing Sprint 13 login page is migrated to shadcn form primitives
- Existing Sprint 13 logout/protected-route UI is migrated
- No Sonner or second toast system is introduced
- No backend API contract changes are introduced
- No Admin UI is added in this patch
- `cd ui && npm run typecheck` passes
- `cd ui && npm run build` passes
- `CGO_ENABLED=1 make build` passes
- `CGO_ENABLED=1 make test` passes
