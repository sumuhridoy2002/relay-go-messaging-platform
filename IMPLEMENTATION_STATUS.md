# Relay implementation status

This document records the state of the repository after the initial source audit and baseline fixes. It deliberately distinguishes working code from planned work.

## Implemented in the current repository

- Go HTTP API using chi and pgxpool.
- PostgreSQL connection check and `/health` endpoint.
- Registration and login with bcrypt password hashes, bounded request bodies, email/username/display-name validation, and explicit duplicate-key handling.
- HS256 JWT issuance and authenticated API middleware.
- Next.js auth proxy with an HttpOnly `relay_token` cookie.
- Current-user endpoint.
- Conversation listing and transactional conversation creation.
- Direct and group conversation records with membership rows.
- User search by username or display name.
- Text message creation and bounded recent message history.
- Next.js chat API proxy and initial chat dashboard.
- New-message user search modal.
- Transaction-scoped advisory locking and an existing-pair lookup to prevent concurrent duplicate direct conversations.
- Additional PostgreSQL indexes for membership lookup, message ordering, and substring user search.

## Not implemented yet

These features must not be represented in the UI or documentation as working until implemented and tested:

- WebSocket transport and Redis Pub/Sub.
- Online presence, typing indicators, delivery acknowledgements, and read receipts.
- Message cursor pagination, editing, deletion, replies, forwarding, and reactions.
- Group administration, membership management, and group settings.
- Attachment and voice-message upload/download.
- LiveKit token generation and audio/video calling.
- Notifications and push notifications.
- Profile editing, password reset, email verification, account deletion, and session revocation.
- Conversation/message search beyond the existing user search.
- Automated Go, API integration, and frontend tests.

## Required local verification

The source archive contained local environment files and a generated Next.js `.next` directory. Those are excluded from clean source packages. Recreate local environment files from the `.env.example` templates and supply fresh local credentials.

Apply migrations in numeric order to the existing `relay` database:

```powershell
psql "$env:DATABASE_URL" -f apps/api/migrations/001_init.sql
psql "$env:DATABASE_URL" -f apps/api/migrations/002_query_indexes.sql
```

Do not rerun `001_init.sql` against a database where its tables already exist; it is the initial schema, not an idempotent migration.

Run backend checks with the Go version declared in `apps/api/go.mod`:

```powershell
cd apps/api
go test ./...
go vet ./...
```

Run frontend checks:

```powershell
cd apps/web
npm ci
npx tsc --noEmit
npm run build
```

A successful build or test result must be confirmed in the target Windows development environment before treating this project as production-ready.
