# Admin UI

The Plomvix Admin UI is a browser-based administration interface for managing
users, API keys, and monitoring system health. It is built with React 18,
TypeScript, Tailwind CSS v4, and [shadcn/ui](https://ui.shadcn.com/).

## Route

```
/app/admin
```

Accessible from the sidebar **only for admin users**. Non-admin users see a
forbidden state.

The route is protected by the same authentication guards introduced in
Sprint 11-13. An authenticated admin session is required.

## Authentication

- The Admin UI requires an **authenticated admin user** (the user whose
  `Role` field is `admin`).
- Login is handled via the existing `/login` flow. The session is tracked by
  the httpOnly `plomvix_token` cookie.
- Non-admin users navigating to `/app/admin` receive a forbidden UI; no admin
  data is exposed to non-admin sessions.

## User Management

### List Users

Displays all registered users in a table with columns:

| Column       | Description                              |
|--------------|------------------------------------------|
| Username     | The user's login name                    |
| Role         | Either `admin` (preserved from creation) |
| Created At   | Timestamp of user creation               |
| Updated At   | Timestamp of last user modification      |

### Create User

A form that accepts:

- **Username** (required)
- **Password** (required)

The role is auto-assigned or preserved as `admin` (no role dropdown exists
in Sprint 14).

### Edit User

Inline or dialog-based editing that allows:

- Updating the **username**
- Optionally changing the **password** (leave blank to keep the current
  password)

### Delete User

- Requires the admin to **type the target username** into a confirmation
  field before the delete action is enabled.
- The admin **cannot delete their own account** — the UI blocks this
  operation.

## API Key Management

### Key Status Per User

Each row in the user table shows one of two states:

| Status   | Meaning                                  |
|----------|------------------------------------------|
| Active   | The user has a valid (non-revoked) key   |
| None     | No API key has been generated yet        |

### Generate API Key

- Clicking **"Generate Key"** creates a new API key for the selected user.
- The **plaintext key is displayed once** in a modal or inline panel.
- **Full-access warning:** Sprint 14 API keys grant **full API access**
  across all endpoints. Scopes/permissions are not implemented yet. This
  warning is displayed prominently when a key is generated.

### Revoke API Key

- Users with an active key see a **"Revoke"** button.
- The action requires an explicit confirmation (dialog or modal).
- Once revoked, the key is invalidated server-side and the status reverts
  to **None**.

### Copy to Clipboard

A **"Copy"** button copies the generated plaintext key to the system
clipboard. A brief visual confirmation ("Copied!") is shown.

### Show / Hide Generated Key

- The plaintext key is masked by default (dots or hidden field).
- A **Show/Hide toggle** (eye icon) reveals or masks the key in the UI.

### Security: Plaintext Key Handling

- The plaintext key exists **only in browser memory** while the generation
  modal is open.
- It is **never persisted** to `localStorage` or `sessionStorage`.
- It is **never transmitted in URL query parameters**.
- Dismissing the modal or navigating away permanently discards the plaintext
  value. The key cannot be recovered — only a new one can be generated.

## Agent Configuration Examples

Static, copy-paste-ready configuration snippets for common agents. Use
`YOUR_PLOMVIX_API_KEY` as the placeholder.

### curl

```bash
curl -H "X-API-Key: YOUR_PLOMVIX_API_KEY" \
     -H "Content-Type: application/json" \
     https://your-plomvix-host/health
```

### Telegraf

```toml
[[outputs.http]]
  url = "https://your-plomvix-host/api/v1/ingest"
  headers = { X-API-Key = "YOUR_PLOMVIX_API_KEY" }
```

### Vector

```toml
[sinks.plomvix]
  type = "http"
  inputs = ["*"]
  uri  = "https://your-plomvix-host/api/v1/ingest"
  [sinks.plomvix.request.headers]
    X-API-Key = "YOUR_PLOMVIX_API_KEY"
```

### Fluent Bit

```ini
[OUTPUT]
    Name     http
    Match    *
    Host     your-plomvix-host
    Port     443
    URI      /api/v1/ingest
    Format   json
    Header   X-API-Key YOUR_PLOMVIX_API_KEY
    tls      On
```

## System Stats

A row of summary cards at the top of the Admin UI displaying key runtime
metrics:

| Card        | Source / Meaning                           |
|-------------|--------------------------------------------|
| WAL         | WAL segment count / latest sequence number |
| Hot Tier    | Hot tier size and usage                    |
| Cold Tier   | Cold tier size and usage                   |
| Runtime     | Go version, OS, architecture               |
| Build Info  | Build version, commit hash, build date     |
| Uptime      | Process uptime since start                 |

Stats cards **auto-refresh every 30 seconds**. A **manual refresh button**
is also available to fetch the latest data on demand.

## Current Limitations (Sprint 14)

The following features are not yet implemented:

- **No RBAC expansion** — only a single `admin` role exists
- **No API key scopes** — all keys grant full API access
- **No audit logs** — user/API operations are not logged for audit
- **No charts or graphs** — all stats are numeric cards only
- **No role editor** — roles are preserved at creation and cannot be changed
  through the UI

These limitations will be addressed in future sprints.

## Architecture

Built on the Sprint 11-13 infrastructure:

- **React 18** — UI framework with hooks and concurrent features
- **TypeScript** — type-safe development
- **Tailwind CSS v4** — utility-first styling
- **shadcn/ui** — accessible React components
- **Centralized API client** — all API calls go through the shared client
  with automatic cookie-based auth
- **Cookie-based auth** — httpOnly `plomvix_token` cookie (no localStorage
  tokens)
- **Toast event system** — consistent user feedback via the shared toast
  infrastructure
- **Theme variables** — `--plx-*` CSS custom properties for light/dark mode
  consistency
- **Protected routes** — `<ProtectedRoute>` wrapper ensures authenticated
  sessions for all `/app/*` routes
