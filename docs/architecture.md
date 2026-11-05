# KingsGambit2700 architecture

Verified against the source tree and local PostgreSQL on 2026-09-22. The local
database is at application migration **17**, with **95 public tables**: 88
application tables and seven migration/River infrastructure tables. This describes
the implementation in this workspace; it does not assert that production has
deployed the same revision.

The complete table inventory is in [Database entities](database-entities.md).
The feature walkthrough is in [Portal workspace implementation](portal-workspace-implementation.md).

## Runtime topology

```mermaid
flowchart TB
  PEOPLE["Visitors, guardians, students, coaches and academy staff"]
  WEB["Next.js 16 / React 19<br/>Public website and role portals"]
  ENGINE["Private chess preparation and review<br/>chess.js + Stockfish 19 browser worker"]
  API["Go / Fiber REST API<br/>/api/v1"]
  DOMAIN["Identity and permissions · Academy and scheduling<br/>Credits · Learning and reports · Trials and support"]
  RT["Go realtime service<br/>/ws/v1 · classroom protocol v2"]
  WORKER["Go worker<br/>River outbox and email dispatch"]
  PG[("PostgreSQL 18<br/>Business records, authentication, credit ledger,<br/>classroom history, audit and River jobs")]
  REDIS[("Redis 8<br/>Rate limits and classroom revision invalidations")]
  QUARANTINE[("Private S3 quarantine bucket")]
  SCAN["ClamAV<br/>API scans uploaded files"]
  CLEAN[("Private S3 clean bucket")]
  EMAIL["Email provider<br/>Local SMTP: Mailpit · Optional: Resend"]
  VIDEO["Zoom / Google Meet<br/>External meeting links"]
  CMS["Optional Sanity<br/>Editorial and blog content"]

  PEOPLE --> WEB
  WEB -->|API cookies and CSRF| API
  WEB --> ENGINE
  WEB -->|Session cookie and one-use join ticket| RT
  WEB -->|Open meeting link| VIDEO
  WEB -.-> CMS
  API --> DOMAIN
  DOMAIN --> PG
  API -->|Rate limiting| REDIS
  RT <-->|Commit, snapshots and replay| PG
  RT <-->|Committed revision notifications| REDIS
  API -->|Upload| QUARANTINE
  API --> SCAN
  API -->|Promote scanned file| CLEAN
  CLEAN -->|Authorized download through API| API
  WORKER <-->|Durable outbox and job records| PG
  WORKER --> EMAIL
  API -->|Sign-in and verification email| EMAIL
```

The root application and Go backend deploy independently. The Go module produces
three processes: `api`, `realtime`, and `worker`. They share PostgreSQL and domain
contracts. The browser never connects directly to PostgreSQL, Redis, or a storage
bucket.

## Local services and connection credentials

These are the verified local `.env`/Compose settings, not production credentials.
Host-run API/realtime use **8180/8181** here. Compose's optional `app` profile
instead exposes **8080/8081**. Inside Docker, services use Compose DNS names and
container ports; PostgreSQL is `postgres:5432` internally.

| Component | Host address | Local connection/authentication | Responsibility |
| --- | --- | --- | --- |
| Frontend | `http://localhost:3000` | Portal account; public pages need none | Website and four role workspaces |
| API | `http://localhost:8180/api/v1` | HttpOnly `kg_session`, CSRF on unsafe requests | Authorized operational reads and mutations |
| Realtime | `ws://localhost:8181/ws/v1` | Session cookie plus single-use join ticket | Synchronized classroom board |
| Worker | No public port | Backend database and email configuration | Email reminders and notification delivery |
| PostgreSQL | `localhost:15432` | Database `kg2700`; user `kg2700`; password `kg2700` | Durable application state |
| Redis | `localhost:6379`, database `0` | No password configured locally | Rate limits and revision invalidations |
| S3Mock | `http://localhost:9090`; HTTPS port `9191` | Access key `local-access-key`; secret `local-secret-key`; region `us-east-1` | Local private file storage emulation |
| Mailpit inbox | `http://localhost:8025` | No login configured locally | Read captured sign-in and notification emails |
| Mailpit SMTP | `localhost:1025` | No username/password configured | API/worker local email transport |
| ClamAV | `localhost:3310` | No application credentials configured | File malware scanning |
| Application migrations | One-shot CLI/container | Direct PostgreSQL URL | Versioned application schema |
| River migrations | One-shot CLI/container | Direct PostgreSQL URL | Independently versioned queue schema |

PostgreSQL connection string:

```text
postgres://kg2700:kg2700@localhost:15432/kg2700?sslmode=disable
```

Storage buckets are `kg2700-quarantine` and `kg2700-clean`, with path-style
addressing enabled. Docker volumes retain PostgreSQL, Redis, S3Mock, Mailpit,
and ClamAV data across ordinary container restarts.

At inspection, `NEXT_PUBLIC_REALTIME_URL` was unset. For the separate host-run
services, set it to `ws://localhost:8181/ws/v1` when running Next.js. Without an
override, the browser derives `/ws/v1` on the API origin, which requires a proxy
to the realtime service. `NEXT_PUBLIC_DEMO_MODE` is also unset in this `.env`.

### Optional integrations and application keys

| Configuration | Purpose | Local state at inspection |
| --- | --- | --- |
| `KG_GOOGLE_CLIENT_ID` | Google Identity Services sign-in | Unconfigured; no Google client secret is used by this flow |
| Sanity project/dataset/read token | Optional editorial content | `EDITORIAL_CONTENT_ENABLED=false`; no project/read token configured |
| `KG_RESEND_API_KEY` | Hosted email delivery | Unconfigured; SMTP selected instead |
| `KG_RAZORPAY_KEY_ID`, `KG_RAZORPAY_KEY_SECRET` | Payment provider adapter | Unconfigured; checkout is outside the enabled workflows |
| `KG_DAILY_API_KEY` | Video provider adapter | Embedded Daily is outside the enabled workflows |
| Provider webhook secrets | Verify Razorpay/Resend/Daily webhook receipts | Environment configuration; webhook support does not enable checkout or embedded video |
| `KG_SESSION_HASH_KEY` | Protect session/token digests | Backend key, not a portal password |
| `KG_CSRF_HASH_KEY` | Protect CSRF tokens | Backend key, not a portal password |
| `KG_TOTP_ENCRYPTION_KEY` | Encrypt authenticator secrets at rest | Backend key, not an MFA code |

Environment-specific keys belong in `.env` or the deployment secret store. Their
names are documented here; their values are not needed to connect to the local
services listed above. No production secret inventory was inspected.

## Portal accounts and authorization

The seven seeded local accounts currently exist, are active, and have password
credentials set. Login accepts either the username or email.

| Role | Username | Email | Portal | MFA enrolled at inspection |
| --- | --- | --- | --- | --- |
| Guardian | `guardian` | `guardian@example.test` | `/guardian` | No; staff MFA not required |
| Student | `student` | `student@example.test` | `/student` | No; staff MFA not required |
| Coach | `coach` | `coach@example.test` | `/coach` | Yes |
| Academic manager | `academics` | `academics@example.test` | `/admin` | No; setup required at staff sign-in |
| Operations admin | `operations` | `operations@example.test` | `/admin` | No; setup required at staff sign-in |
| Finance admin | `finance` | `finance@example.test` | `/admin` | Yes |
| Super-admin | `admin` | `admin@example.test` | `/admin` | Yes |

Passwords are individually salted Argon2id hashes in
`auth_password_credentials.password_hash`. There is no recoverable shared
password in the database, and `KG_LOCAL_TEST_PASSWORD` is not set in the current
`.env`. Use the password originally assigned to these accounts, or choose
**Email me a sign-in link** at `/sign-in` and open the message in Mailpit.
Email-link access still requires staff MFA. Guardians can also issue student
access codes for linked learners.

The database also contains one additional active guardian account without a
password credential, and eight integration fixture users with no active role
grants or passwords. Fixture display names such as “Integration Coach” do not
grant a role. Those users have no authorized portal until properly provisioned.

[Authentication setup](authentication-setup.md) documents the local password
provisioning command. Running it changes all seven test passwords and revokes
their existing sessions/devices; this inventory did not run it.

`/portal` resolves an authorized destination. Home/logo navigation and successful
sign-in use current roles rather than retaining an inaccessible staff URL.
Multi-role users retain access to their permitted workspaces. Backend access
checks also require current active relationships and assignments, with explicit
denies taking precedence. Role labels and record IDs alone do not grant access.

Authentication uses finite access sessions plus revocable remembered-device
credentials. HttpOnly `kg_session_device` can restore an expired access session
without losing the previous MFA assurance timestamp. Sensitive staff mutations
still require recent MFA. Logout, device revocation, security reset, and account
suspension block the associated restoration path. CSRF state is shared safely
across restoration/navigation; service outages are retryable errors.

## Student credits: authoritative storage and lifecycle

Credits belong to an **enrollment**, not directly to a mutable `credits` column
on the student or family. A learner with multiple enrollments has separate
credit accounts.

```mermaid
flowchart LR
  USER["users"] --> STUDENT["student_profiles"]
  STUDENT --> ENROLL["enrolments"]
  BATCH["batches"] --> ENROLL
  ENROLL --> ACCOUNT["entitlement_accounts<br/>one per enrollment"]
  ACCOUNT --> ENTRY["entitlement_entries<br/>append-only ledger"]
  CLASS["class_sessions"] -->|Reservation or completion reference| ENTRY
  ENTRY --> BALANCE["SUM of available_delta,<br/>reserved_delta and consumed_delta"]
```

| Table/field | Meaning |
| --- | --- |
| `student_profiles.id` | Learner identity |
| `enrolments.student_profile_id` / `batch_id` | Learner's membership and enrollment policy snapshot |
| `entitlement_accounts.enrolment_id` | Unique account for that enrollment; unit is `session_credit` |
| `entitlement_entries.account_id` | Account affected by an immutable operation |
| `available_delta` | Change in credits available to schedule |
| `reserved_delta` | Change in credits held for scheduled classes |
| `consumed_delta` | Change in used credits, including any later reversals |
| `available_balance`, `reserved_balance`, `consumed_balance` | Resulting balances recorded with each entry |
| `entry_type`, `reason`, `actor_user_id`, timestamps | What happened, why, who authorized it, and when |
| `idempotency_key`, `request_digest`, reference fields | Safe retries and links to the business operation/class |

| Operation | Available | Reserved | Consumed |
| --- | --- | --- | --- |
| Grant eight credits | +8 | 0 | 0 |
| Schedule one class | -1 | +1 | 0 |
| Complete that class normally | 0 | -1 | +1 |
| Release a reservation when policy permits | +1 | -1 | 0 |
| Reverse a consumed credit | +1 | 0 | -1 |

Balances are sums of ledger deltas. Database triggers prevent updating/deleting
ledger entries. Mutations lock the entitlement account, reject negative balances,
record an audit event, and enforce per-account idempotency. Class creation, roster,
classroom access/state, and credit reservations commit together. A series that
lacks credits rolls back as a whole. Cancellation/attendance outcomes follow the
enrollment policy and can create make-up obligations; not every absence refunds
a credit.

At inspection, the only local enrollment was **Local Student**, active, with
**0 available / 0 reserved / 0 consumed**. It has account
`01a0c89a-7dbd-758a-b734-89ca8f91e24a` and enrollment
`01a0c89a-7dbd-7aa2-814b-5886a1351aba`. These are local snapshot identifiers.

Finance and super-admin can grant credits through **Admin → Settings → Enrollment
class credits**, subject to current permission checks and recent MFA. The portal
also displays enrollment balances in its learner records.

Relevant REST endpoints:

```text
GET  /api/v1/entitlements/accounts/{accountId}
GET  /api/v1/entitlements/accounts/{accountId}/entries
POST /api/v1/entitlements/accounts/{accountId}/grants
```

Read-only SQL to inspect all enrollment balances:

```sql
SELECT s.preferred_name AS learner,
       e.id AS enrolment_id,
       e.status,
       a.id AS credit_account_id,
       COALESCE(SUM(l.available_delta), 0) AS available,
       COALESCE(SUM(l.reserved_delta), 0) AS reserved,
       COALESCE(SUM(l.consumed_delta), 0) AS consumed
FROM enrolments e
JOIN student_profiles s ON s.id = e.student_profile_id
LEFT JOIN entitlement_accounts a ON a.enrolment_id = e.id
LEFT JOIN entitlement_entries l ON l.account_id = a.id
GROUP BY e.id, s.preferred_name, a.id
ORDER BY s.preferred_name;
```

Open the local SQL prompt with:

```bash
docker compose exec postgres psql -U kg2700 -d kg2700
```

## Application boundaries and data ownership

| Boundary | Responsibilities and persistence |
| --- | --- |
| Identity and permissions | Users, roles, scoped grants/denies, passwords, magic links, Google identities, MFA, sessions and remembered devices in PostgreSQL |
| Academy operations | Families, learner profiles, guardian relationships, consent/policies, coaches, batches and enrollments |
| Scheduling | Availability, single/weekly classes, previews, recurrence exceptions, rosters, attendance, lesson notes, meeting links, cancellations and make-ups |
| Learning | Assignments, immutable assigned content, individual attempts, review/resubmission, puzzle validation and practice metrics |
| Library and game review | Shared/personal resources, published versions, imported games, annotations and feedback; private file metadata in PostgreSQL, bytes in S3 |
| Curriculum and reports | Versioned curricula/placements/rubrics, assessments and history, published monthly reports, acknowledgements and certificates |
| Communication | Learner conversations, batch announcements, per-user unread state, in-app notifications and durable email outbox |
| Acquisition and support | Leads, trial slots/bookings, coach/outcome/follow-up/conversion records, family support cases and timelines |
| Realtime classroom | Authorized session access, board state, ordered events, snapshots, coach controls, annotations, checkpoints and countdowns |
| Infrastructure | Audit events, mutation receipts, webhook receipts, queue jobs and schema versions |

The frontend loads authorized `/academy/workspace` and `/learning/workspace`
data. Controllers bind HTTP input; services implement authorization and transitions;
pgx repositories perform parameterized SQL. Operational mutations live in the Go
API; the old Next.js `/api/report-card` route is retired.

`backend/api/openapi.yaml` and `backend/api/ws/` define public contracts. Generated
REST types live in `app/lib/api/schema.d.ts` and
`backend/internal/contracts/fiber.gen.go`. APIs use camel-case JSON, RFC3339
timestamps, IANA timezones, optimistic versions and structured problem responses.
New IDs use UUIDv7; persisted UUIDv4 references remain compatible.

## Realtime, uploads, and background delivery

**Classroom:** the API issues a 60-second single-use ticket. The WebSocket service
at `/ws/v1` negotiates `kg2700.classroom.v2` (with a v1 compatibility projection),
verifies the session/assignment, then accepts commands with a unique command ID
and expected revision. PostgreSQL commits the ordered event/state before local
broadcast and Redis revision invalidation. Other instances signal a revision gap;
clients recover using authoritative snapshots/replay. Redis does not store the
durable event history. Private puzzle answers/solutions stay in learning storage.

**Preparation:** Stockfish 19 lite runs on demand in a browser worker. Private
analysis is separate from the live shared board; a coach explicitly applies a
prepared position. Engine license, source and build artifacts ship in
`public/engine/`. Zoom/Meet links open external meetings; attendance remains
coach-confirmed.

**Files:** API validation → private quarantine object/metadata → inline ClamAV
scan → clean object → authorized publication/evidence link. Downloads go through
the authenticated API and current access checks. Resource versions retain
historical file references. Scan errors prevent publication. Scanning is not a
River worker task in this implementation.

**Email:** domain transactions persist notification intent. A River sweeper runs
at startup and every 30 seconds, creates unique dispatch jobs in PostgreSQL, and
the worker checks current eligibility before delivery. Delivery attempts and
retry state are durable. The registered job types are `notification_outbox` and
`dispatch_notification`. SMTP may retry an already accepted message if a crash
occurs before its sent state commits. In-app unread state is stored separately.

## Deployment and implementation boundaries

The intended production topology serves Next.js on the academy web domain, and
proxies `/api/v1` and `/ws/v1` on `api.kingsgambit2700.in` to the corresponding Go
processes. Credentialed origins must match configuration. The API host owns the
session cookies, so protected Next.js pages bootstrap session state in the browser.
The worker has no public ingress. Health/readiness/metrics routes support operations.
See [Deployment](deployment.md) for migrations, storage, cookies and proxy setup.

The browser-only demo uses a separate localStorage/sessionStorage transport with
the same API-shaped contracts. Its fictional balances do not write PostgreSQL;
production workflows use the Go API and durable database authority.

Optional Sanity is editorial-only. Google sign-in and Resend require configuration.
Razorpay/Daily adapters and webhook receipt verification exist, but payment
checkout, subscriptions/refund automation, and embedded Daily video are not enabled
product workflows. Some tables are foundations for deferred operations; the
[entity catalog](database-entities.md) identifies those separately.

Live PvP, matchmaking, tournaments, calculated ratings, SMS/WhatsApp delivery,
recording, transcription, native applications, and multi-tenancy remain outside
this implementation. WebSocket presence/provider-state and exercise-specific
commands are not implemented; puzzle work uses private REST learning workflows.

## Source map

| Area | Source |
| --- | --- |
| Runtime wiring | `backend/internal/app/application.go`, `backend/cmd/` |
| Local topology | `compose.yaml`, `.env.example` and local `.env` overrides |
| Portal routing | `app/lib/role-routing.ts` |
| Frontend workspace | `app/components/portal/workspace/AcademyWorkspace.tsx` |
| Session/device flow | `backend/internal/auth/`, `app/lib/` session utilities |
| Academy and scheduling | `backend/internal/domain/academy/` |
| Credit ledger | `backend/internal/domain/entitlement/`, migrations `000001` and `000007` |
| Learning and private files | `backend/internal/domain/learning/` |
| Realtime protocol | `backend/internal/realtime/`, `backend/api/ws/`, `app/lib/realtime/` |
| Notification worker | `backend/internal/platform/jobs/` |
| Local password provisioning | `backend/cmd/local-passwords/main.go` |
| Full database catalog | [Database entities](database-entities.md) |
