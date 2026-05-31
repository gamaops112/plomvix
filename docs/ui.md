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
