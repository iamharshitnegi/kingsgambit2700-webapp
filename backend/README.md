# KingsGambit2700 backend

Independent Go 1.26 services for the academy platform. The HTTP API uses Fiber v3 with manual dependency injection and controller/service/repository boundaries. PostgreSQL is canonical; Redis carries cache/rate-limit data and realtime invalidation hints only.

## Local setup

1. Copy `.env.example` into your secret manager or shell environment.
2. Apply `migrations/*.up.sql` with `golang-migrate` against `KG_DATABASE_DIRECT_URL`.
3. Run `go run ./cmd/river-migrate up`, followed by
   `go run ./cmd/river-migrate validate`, before starting `cmd/worker`.
4. Run `go run ./cmd/api`, `go run ./cmd/worker`, and `go run ./cmd/realtime`.

The public API listens on `:8080`; realtime listens on `:8081`. Deploy both behind `api.kingsgambit2700.in`: route `/ws/v1` to realtime so the host-only cookie remains valid. Exact web origins are configured through `KG_ALLOWED_ORIGINS`.

## Contracts and generation

- HTTP source of truth: `api/openapi.yaml`
- Generated Fiber v3 models/server interface: `internal/contracts/fiber.gen.go`
- WebSocket JSON Schema: `api/ws/*.schema.json`
- SQL source of truth: `db/query/*.sql` and `migrations/`
- sqlc output: `internal/platform/dbgen/`

Run `go generate ./...`, then `go test ./...`. CI should fail when regeneration leaves a diff. oapi-codegen is pinned at v2.8.0 and sqlc at v1.30.0 in the generation directives.

## First super-admin

Authentication also supports verified password registration and Google sign-in.
See [authentication setup](../docs/authentication-setup.md) for configuration,
local account passwords, and verification behavior. Apply migration 10 before
using these methods.

There are no default credentials. After migrations, run once:

```sh
KG_DATABASE_DIRECT_URL='postgres://…' go run ./cmd/bootstrap-admin \
  -email admin@example.com -display-name 'Academy Administrator' -dry-run
```

Remove `-dry-run` only after reviewing the target database. The command serializes concurrent attempts, refuses to elevate an existing user or run after any super-admin grant has existed, and writes an audit event in the same transaction. Avoid `-database-url`; command-line URLs can leak through process listings.

## Containers

Use build context `backend/` with `backend/Dockerfile` targets `api`, `worker`,
`realtime`, or the migration utility `river-migrate`. The runtime entrypoints
are `/app/api`, `/app/worker`, `/app/realtime`, and `/app/river-migrate`.

Integration migration/repository verification is opt-in. Use a disposable
PostgreSQL database and apply the application migrations first, as in local
setup above. This also installs the shared `citext` extension in `public`;
individual regression tests create and remove isolated schemas. Set both
variables so the relationship and report integration cases also run:

```sh
export KG_TEST_DATABASE_URL='postgres://…'
export KG_DATABASE_URL="$KG_TEST_DATABASE_URL"
go test -p 2 -tags=integration ./...
```

The trial regressions include competing seat bookings and concurrent retries
of one request. Classroom tests recheck revoked assignments and authenticated
sessions; report response tests verify unpublished corrections stay private.
The package parallelism limit avoids exhausting PostgreSQL's default lock
table when many isolated schemas are dropped together; concurrency inside the
booking regression tests remains enabled.

The package layout and database schema intentionally cover more R1 domains
than the currently exposed HTTP surface. See
[`../docs/implementation-status.md`](../docs/implementation-status.md) before
planning a production rollout.
