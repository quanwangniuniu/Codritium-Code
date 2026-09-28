# Codritium

Self-serve AI-coding interview practice. IDE-style workspace, 5-dimension scoring grader, integrated AI chat panel.

## Stack

- **Frontend**: Next.js 15 (App Router) + TypeScript + Tailwind v4 + shadcn/ui + Monaco Editor
- **Backend**: Go 1.26 + pgx/v5 + Anthropic Go SDK + E2B Go SDK
- **DB**: Postgres 17 (port 5434)
- **Sandbox**: E2B SaaS (Firecracker microVM) for candidate code execution
- **Grader**: Anthropic Claude Sonnet 4.6 (5 independent dimensions, double-run averaged)

## Quick start

```bash
cp .env.example .env   # fill in keys
docker-compose up -d   # start Postgres
cd backend && go run ./cmd/server   # backend on :8080
cd frontend && npm run dev          # frontend on :3000
```

## Layout

```
Codritium/
├── backend/        # Go API server
├── frontend/       # Next.js app
├── sandbox/        # E2B integration helpers
├── scripts/        # strip_problem.py and other tooling
├── migrations/     # Postgres schema migrations (001-...)
├── seed/           # initial problem + mock user seed data
├── docker-compose.yml
├── .env            # secrets (never committed)
├── .env.example
└── README.md
```

## Environment

`.env` keys required:

```
ANTHROPIC_API_KEY=sk-...
E2B_API_KEY=e2b_...
DATABASE_URL=postgres://codritium:codritium@localhost:5434/codritium?sslmode=disable
COOKIE_SECRET=...
```

## Authentication

MVP uses demo cookie session with 5 mock users (no GitHub OAuth yet). Switch user from the avatar dropdown.

## Status

Pre-MVP. Goal: end-to-end loop (login → pick problem → write code with AI chat → submit → 5-dimension scoring).
