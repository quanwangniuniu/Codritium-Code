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
| `NEXT_PUBLIC_API_BASE` | browser calls (`src/shared/api/client.ts`) | `http://localhost:8080` |
| `CODRITIUM_API_URL`    | server components (`src/shared/api/server.ts`) | `http://localhost:8080` |

## Checks

```bash
npx tsc --noEmit   # type check (what CI runs)
npm run lint
npm test           # vitest unit tests (pure modules)
npm run build
```

## Layout

- `src/app` — routes only; each page re-exports a page component from a feature
- `src/features/<name>` — one folder per product area (auth, forum, home, plans,
  problems, profile, reply, settings, submissions, workspace) with
  `components/`, `hooks/`, `lib/`, `api.ts` (browser endpoints), `server.ts`
  (server-component reads), `types.ts` and `i18n.ts` (its English strings)
- `src/shared` — cross-feature code: `api/` (browser + server clients, `ApiError`),
  `ui/`, `layout/` (nav, toasts, banner, markdown), `editor/` (Monaco pane),
  `format/`, `labels/`, `i18n/` (`t()`, locale store, merged dictionaries), `lib/`
