# Relay — communication platform (Laragon / Windows)

This starter is designed for local development on Windows with Laragon. **Docker is not required.**

## Stack
- API: Go + chi
- Web: Next.js + TypeScript
- Database: PostgreSQL
- Cache/presence/pub-sub: Redis
- Audio/video: LiveKit Cloud (WebRTC media infrastructure)

## Requirements
- Laragon (Apache/Nginx is not required for the Next.js dev server)
- Go 1.26+ (the version declared in `apps/api/go.mod`)
- Node.js 20+
- PostgreSQL installed and running locally
- Redis installed and running locally (optional for the initial health-check scaffold; required when presence/pub-sub is implemented)

## Project location
Extract this folder to:

`C:\laragon\www\relay`

## 1. Configure PostgreSQL
Create a PostgreSQL database named `relay` and a database user/password, or use your existing local PostgreSQL credentials.

Example SQL (run in pgAdmin or psql):
```sql
CREATE DATABASE relay;
```

Copy `apps/api/.env.example` to `apps/api/.env` and update `DATABASE_URL` to match your local PostgreSQL username, password, host and port.

Example:
`postgres://postgres:YOUR_PASSWORD@127.0.0.1:5432/relay?sslmode=disable`

## 2. Run the database migration
From Laragon Terminal:
```bash
cd C:\laragon\www\relay
psql "postgres://postgres:YOUR_PASSWORD@127.0.0.1:5432/relay?sslmode=disable" -f apps/api/migrations/001_init.sql
psql "postgres://postgres:YOUR_PASSWORD@127.0.0.1:5432/relay?sslmode=disable" -f apps/api/migrations/002_query_indexes.sql
```
Replace the username/password with your local PostgreSQL credentials.

## 3. Start Go API
Open Laragon Terminal:
```bash
cd C:\laragon\www\relay\apps\api
copy .env.example .env
go mod tidy
go run ./cmd/api
```
API health check: http://localhost:8080/health

## 4. Start Next.js frontend
Open a second Laragon Terminal:
```bash
cd C:\laragon\www\relay\apps\web
npm install
npm run dev
```
Frontend: http://localhost:3000

## Optional: Redis
Install/run Redis as a native Windows-compatible service or use a managed Redis provider. Set `REDIS_URL` in `apps/api/.env`.
Redis is not yet wired into the initial health-check endpoint; it will be integrated in the real-time presence/pub-sub phase.

## Current implementation status
See [IMPLEMENTATION_STATUS.md](IMPLEMENTATION_STATUS.md) for an accurate list of implemented and outstanding functionality. In particular, WebSocket/Redis real-time delivery and LiveKit calling are not implemented yet.

## Important
- Do not rerun `001_init.sql` against a database that already has the tables; it is the initial schema.
- `002_query_indexes.sql` is an additive migration and can be applied to an existing database.
- The source archive may contain local `.env` files; clean project packages intentionally exclude them. Create fresh local files from `.env.example`.
- Laragon is used as the local development environment; Go and Next.js run as their own processes.
- Apache/Nginx is not needed for local development.
- LiveKit credentials must remain on the Go server. Never expose `LIVEKIT_API_SECRET` in frontend environment variables.
- This is an initial scaffold and UI prototype, not yet a finished messaging/calling product.
