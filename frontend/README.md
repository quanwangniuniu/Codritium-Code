# Codritium frontend

Next.js (app router) client for the Codritium backend. See the root
[README](../README.md) for the full stack setup.

## Run

```bash
npm install
npm run dev        # http://localhost:3000
```

The backend must be running (default `http://localhost:8080`).

## Environment

| Variable               | Used by                         | Default                 |
| ---------------------- | ------------------------------- | ----------------------- |
| `NEXT_PUBLIC_API_BASE` | browser calls (`src/lib/api.ts`) | `http://localhost:8080` |
| `CODRITIUM_API_URL`    | server components (`src/lib/api-server.ts`) | `http://localhost:8080` |

## Checks

```bash
npx tsc --noEmit   # type check (what CI runs)
npm run lint
npm run build
```

## Layout

- `src/app` — routes; the problem workspace lives in `problems/[id]/workspace`
- `src/components` — UI; `ide/` is the editor, `reply/` the session replay viewer
- `src/lib` — API clients, local session stores, i18n, theme
