# Local development

## UI and workflow demo (no services required)

With Node.js and dependencies installed, run `npm run dev:demo` and open
`http://localhost:3000/demo`. There is no `.env` file requirement. This command
enables `NEXT_PUBLIC_DEMO_MODE=true` only for the development server. Run
`npm run dev` without that flag to use the real backend configuration below.

Sample academy records persist in localStorage. The selected account is stored
per tab in sessionStorage, so a coach and student can use separate tabs on the
same browser origin. The demo page provides a reset action. Use fictional data
in this mode. Production builds always disable the demo transport and return
404 for `/demo`, even if the demo environment flag is set.

See [demo walkthrough](demo-walkthrough.md) for accounts, scenarios, and limits.

## Prerequisites

- Node.js 24 and npm
- Go 1.26
- Docker with Compose v2

## Start the stack

1. Copy `.env.example` to `.env` and keep the example-safe local values.
2. Run `make bootstrap`.
3. Run `make infra-up` and wait for PostgreSQL, Redis, S3Mock, Mailpit, and
   ClamAV to become healthy.
4. Run `make migrate`.
5. In separate terminals run `make backend-api`, `make backend-worker`,
   `make backend-realtime`, and `make frontend`.

The web app is at `http://localhost:3000`, the REST API at
`http://localhost:8080`, realtime at `ws://localhost:8081/ws/v1`, and Mailpit at
`http://localhost:8025`.

Alternatively, `make app-up` builds and starts the three Go containers together
with infrastructure. Run the frontend on the host for the fastest refresh loop.

## Verification

- `make frontend-check` runs frontend lint, type checking, and unit tests.
- `make go-race` runs all non-integration Go tests with race detection.
- `make compose-check` validates the Compose model.
- `make generate` refreshes checked-in generated code; a clean Git diff is the
  drift check.

Integration tests use the `integration` build tag and require the local
infrastructure. Provider sandbox tests require their explicit environment
credentials; absence skips those tests instead of silently using production.

## Local safety

All exposed infrastructure ports bind to `127.0.0.1`. Example secrets are for a
single-developer machine only. Never copy them into shared staging or
production, and never expose the local Redis, database, object store, Mailpit,
or ClamAV ports publicly.

## Production workspace browser verification

The workspace journey uses real API accounts, MFA, PostgreSQL, realtime, private
uploads and Mailpit. It creates and removes an isolated database schema, leaving
normal academy records untouched. It also creates and removes a dedicated Redis
container using the existing Compose image, so test sign-ins cannot exhaust
normal rate limits. Start the Compose infrastructure first and use
an available local PostgreSQL URL in `KG_TEST_DATABASE_URL` (or `KG_DATABASE_URL`).

Build the separate verification output with its local service addresses:

```bash
KG_NEXT_DIST_DIR=.next-workspace NEXT_PUBLIC_DEMO_MODE=false \
NEXT_PUBLIC_SITE_URL=http://localhost:3005 \
NEXT_PUBLIC_API_BASE_URL=http://localhost:8085 \
NEXT_PUBLIC_REALTIME_URL=ws://localhost:8086/ws/v1 \
API_BASE_URL=http://localhost:8085 EDITORIAL_CONTENT_ENABLED=false npm run build
```

Load the root environment and run `npm run test:workspace`. This starts the API
on 8085, realtime on 8086, and the production frontend on 3005. Use
`PLAYWRIGHT_CHANNEL=chrome` if testing with installed Google Chrome, or install
Playwright's Chromium. `node scripts/workspace-e2e.mjs --serve` keeps the same
isolated workspace running for manual inspection until interrupted. Ports
3005, 8085 and 8086 must be free. Test-only authenticator state is stored in a
private temporary file keyed by the disposable schema; it is never checked in.

For the complete backend suite, use `go test -p 1 -race -tags=integration ./...`
from `backend` with `KG_TEST_DATABASE_URL` set. Sequential package execution
avoids exceeding PostgreSQL's local lock-table capacity during schema migration
and cleanup. `KG_TEST_FILES=1` additionally enables the S3Mock/ClamAV provider
smoke test. Demo browser tests use a separate output/build directory and run with
`npm run test:demo`.

## Sample homework for the real local API

If the Compose database already contains the active **Local Student**
(`student@example.test`) enrolled with **Local Coach** (`coach@example.test`),
run with Node.js 24:

```bash
node scripts/seed-homework.mjs
```

This inserts eight sample assignments, including one private coach draft, in
one transaction. It targets only the local Compose `postgres` service and
refuses production mode or missing fixture accounts. Repeat runs preserve
existing homework and attempts. It does not create accounts, change access, or
send notifications. Refresh `/student/homework` or `/coach/homework` to view
samples. The separate browser-only demo receives these examples automatically.
