# Deployment runbook

For the startup's first release on one Hostinger VPS, use
[the single-VPS runbook](hostinger-vps.md). It runs the production application,
database, queue coordination, private files and scanner on one server.
A second paid server or staging environment is not required; run pre-release
checks locally and keep backups outside the VPS.

## Frontend

Deploy the repository root as the Next.js application. Configure
`NEXT_PUBLIC_SITE_URL`, `NEXT_PUBLIC_API_BASE_URL`, and `API_BASE_URL` without
trailing slashes. Enable editorial content only when Sanity project, dataset,
and read credentials are present. Public pages may deploy without Sanity in
local and test environments.

Authenticated production builds must use `kingsgambit2700.in` or
`www.kingsgambit2700.in`; do not connect generic preview hosts to a production
credentialed API.

The approved portal design is a frontend-only release. Rebuild Next.js to serve
its updated shell, calendar and `/[role]/classes` routes. No additional migration
or backend restart is required for this design change. Keep the existing API and
Realtime URLs. See [portal design integration](portal-design-integration.md) for
local review steps and verification.

## Backend

Build the independent service from `backend/Dockerfile` using targets `api`,
`worker`, and `realtime`. Deploy all three from the same source revision and
contract version.

1. Provision PostgreSQL with direct or session-pooled URLs for migrations and
   workers. Do not use a transaction-only pooler for migration or River
   connectivity.
2. Apply application migrations from `backend/migrations` with the pinned
   migration tool. This portal release requires migrations through `000017`.
3. Apply and validate the independently versioned River migrations.
4. Start `api` and verify liveness and readiness before traffic.
5. Start `worker`, verify queue health and discarded-job visibility.
6. Start `realtime`, verify origin, subprotocol, ticket exchange, persistence,
   Pub/Sub, snapshot, and replay.
7. Route HTTPS API and WebSocket traffic through
   `api.kingsgambit2700.in`; preserve WebSocket upgrade headers.

Run the first-super-admin command once with an approved identity. Capture the
resulting audit event and remove the one-off deployment capability afterward.

| Migration | Purpose |
| --- | --- |
| 000011 | Remembered browsers and session/device relationships |
| 000012 | Academy provisioning, schedules, request receipts and make-ups |
| 000013 | Learning resources, assignments, assessments and notifications |
| 000014 | Quarantined learning files and attachments |
| 000015 | Persistent shared classroom lesson tools |
| 000016 | Versioned curriculum placement history |
| 000017 | Immutable teaching resource versions and historical file references |

## Sessions, proxy and CORS

Use HTTPS and `KG_COOKIE_SECURE=true` in production. Session and device cookies
are HttpOnly, host-only, path `/` and SameSite=Lax. Keep frontend/API hosts under
the approved same-site academy domain. Match the public web/API URL settings and
exact `KG_ALLOWED_ORIGINS`; do not use wildcard credentialed origins. Preserve
`Cookie`, `Set-Cookie`, `Origin`, `X-CSRF-Token` and `Idempotency-Key` through the
proxy. Never cache authenticated API responses or rewrite the device cookie to
a public parent domain. Preserve WebSocket upgrade and subprotocol headers.

By default the browser connects to `/ws/v1` on the API origin. For separate local
services, set `NEXT_PUBLIC_REALTIME_URL=ws://localhost:8081/ws/v1` before starting
or building Next.js. A separate production endpoint must use `wss://` with a
valid TLS certificate and allow the exact frontend origin in the realtime
service's `KG_ALLOWED_ORIGINS`. Use the same hostname as the cookie-issuing API;
ports may differ, but its host-only session cookie cannot reach another hostname.
Keep credentials, tickets, queries and fragments
out of this URL; the socket exchanges its short-lived ticket in the authenticated
protocol. When sharing the API proxy, leave the override unset and route `/ws/v1`
to the realtime service with upgrade/subprotocol headers preserved.

Short sessions keep idle/absolute limits: `KG_SESSION_IDLE_TTL` defaults to 30
minutes and `KG_SESSION_ABSOLUTE_TTL` to seven days. A remembered browser restores
expired short sessions through the exact-origin, CSRF-protected restore route.
Its cookie renews with a 400-day lifetime; browser policy/manual deletion may
remove it earlier. Server device records have no scheduled expiry and remain
revocable. Logout revokes the browser and its derived sessions. Security resets,
suspension and device revocation block restoration. Provider/API outages must
not clear remembered state. Sensitive mutations still require fresh MFA.

Session/device/audit retention must follow the approved academy retention policy.
No automatic device or audit purge job is introduced here. Do not delete active
devices as short-session cleanup. Revoked devices and request receipts require
a reviewed retention procedure respecting session references and retry windows.

## Teaching engine and meeting links

`npm ci` and the normal dev/build scripts prepare the Stockfish 19 single-thread
lite JavaScript/WASM assets. Deploy all of `public/engine/`, including the GPL
license, `NOTICE.txt`, `SHA256SUMS`, corresponding source archive and named NNUE
network. Preserve the teaching board's source/license download links. The notice
identifies source/build inputs; verify artifacts against the checksums during
packaging. Serve WASM as `application/wasm` and permit same-origin workers and
their WASM execution in any deployment CSP. Analysis runs locally in the browser.

Classes accept HTTPS meeting URLs for the academy's Zoom/Meet service. Meetings
open separately from the shared chess board; configure host and waiting-room
controls in the provider. Embedded Daily video, recording and transcription are
not enabled by this release.

## Storage and malware scanning

Configure `KG_S3_BUCKET_QUARANTINE`, `KG_S3_BUCKET_CLEAN`, region/credentials (or
the AWS credential chain), and `KG_CLAMAV_ADDR`. Alternate endpoints/path style
support local S3-compatible services. Keep both buckets private and distinct.
The API requires put/get/delete permissions; public bucket reads are unnecessary
because downloads pass through authorization checks.

`KG_UPLOAD_MAX_BYTES` is bounded by a 20 MB application ceiling. Images also
have 10 MB and 16-megapixel limits. Missing providers, scanner errors and detected
malware fail closed. Before enabling uploads, test a clean sample and the
scanner's standard harmless test signature in the deployment environment.
Successful publication removes quarantine data when possible; cleanup failures
are logged. Configure approved lifecycle retention for orphaned/failed quarantine
objects and retain clean evidence according to academy policy. This release does
not introduce a general evidence deletion/purge worker.

## Email outbox and worker

Run `worker` alongside the API. `KG_EMAIL_PROVIDER` selects SMTP, Resend or the
development fake sender; both processes use the same provider/public-web URL
settings. Local SMTP messages appear in Mailpit. Resend needs the configured
verified sender and API key. Apply and validate River migrations before startup.

The worker sweeps due `academy_event` rows at startup and every 30 seconds,
inserts unique River dispatch jobs, and records delivery attempts. Failures stay
durable with increasing retry delay. Monitor due unsent `notifications`,
failed/suppressed `notification_delivery_attempts`, and discarded River jobs.
Healthy API readiness alone does not prove email delivery. Resend uses a stable
provider idempotency key. SMTP is at-least-once: a crash after SMTP acceptance but
before the database commit can produce a duplicate retry.

Use disposable records to verify schedule creation/rescheduling/cancellation and
homework reminders. Stale reminders should suppress after changed schedules,
completion/submission or revoked access. Keep private conversation bodies,
meeting credentials and learner evidence out of operational email logs.

## Portal smoke test

Follow the [empty-academy journey](portal-workspace-implementation.md#starting-an-empty-academy)
with a guardian, student, assigned coach and administrators. Confirm Home/logo/
back navigation preserves sign-in and explicit logout prevents restoration.
Preview must change no records; an unfunded/conflicting series must roll back.
Teach/complete a class and submit/review homework. Call cross-family and
unassigned-coach APIs directly to verify server denials.

## Rollback

Application deployments are backward-compatible with the currently active
schema during a rollout. Prefer expand/migrate/contract changes. If an
application release must roll back, redeploy the last known-good images first;
do not automatically run a down migration against production data. Escalate a
schema rollback through a reviewed restore/migration plan.

## Release evidence

Archive the revision, generated-contract drift result, migrations, unit/race/
integration/security results, frontend lint/test/build output, provider smoke
tests, load-test report, restore drill, feature-flag state, and pilot approval.
