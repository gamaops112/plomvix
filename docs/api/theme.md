# Plomvix Theme API

Sprint 12 introduces the theme engine with design tokens stored in `theme.json` and served via API endpoints.

## Theme file

Location: `theme.json` at project root.

The theme is a global design-token document with the following shape:

```json
{
  "version": 1,
  "dev_panel": true,
  "mode": "light",
  "tokens": {
    "colors": { ... },
    "dark_colors": { ... },
    "typography": { ... },
    "radii": { ... },
    "spacing": { ... },
    "shadows": { ... },
    "layout": { ... }
  }
}
```

## Validation rules

- `version` must be `1`
- `mode` must be `light` or `dark`
- `dev_panel` must be a boolean
- Color tokens must be valid hex colors (`#rgb` or `#rrggbb`)
- Size tokens must be CSS length strings (`px`, `rem`, `em`)
- `layout.transition_speed` must be a CSS duration (`ms` or `s`)
- Font weights must be numeric strings `100`–`900`
- Font family and shadows must be non-empty strings

## Endpoints

### GET /api/theme
- **Auth:** Public (no auth required)
- **Response:** Plomvix success envelope with current theme in `data`
- Creates default `theme.json` if the file does not exist

### PUT /api/theme
- **Auth:** Admin (JWT or API key, admin role required)
- **Body:** Complete theme document as JSON
- **Response:** Plomvix success envelope with saved theme in `data`
- Replaces the full theme document (no partial patching in Sprint 12)
- Returns 400 with validation errors on invalid input

### POST /api/theme/reset
- **Auth:** Admin (JWT or API key, admin role required)
- **Body:** None
- **Response:** Plomvix success envelope with default theme in `data`
- Replaces current theme with factory defaults

### GET /api/theme/export
- **Auth:** Admin (JWT or API key, admin role required)
- **Response:** Raw downloadable JSON file (`application/json`, `Content-Disposition: attachment`)
- Returns the current saved theme as an indented JSON file
- Does NOT use the Plomvix response envelope

## Frontend CSS variable mapping

Theme tokens are injected as CSS variables on `document.documentElement`:

| Token path | CSS variable |
|---|---|
| `tokens.colors.primary` | `--plx-color-primary` |
| `tokens.colors.secondary` | `--plx-color-secondary` |
| `tokens.typography.font_family` | `--plx-font-family` |
| `tokens.radii.md` | `--plx-radius-md` |
| `tokens.spacing.md` | `--plx-spacing-md` |
| `tokens.shadows.md` | `--plx-shadow-md` |
| `tokens.layout.sidebar_width` | `--plx-sidebar-width` |

## Dev panel

When `theme.dev_panel` is `true`, the `/dev/design` route appears in the sidebar. Set to `false` to hide the developer design panel in production.

## Notes

- Theme is file-backed (`theme.json`). No database storage in Sprint 12.
- One global theme only. No multi-user preferences.
- UI tests are deferred until the UI test stack is introduced.
