# Implementation status

The September 2026 portal expansion supersedes this earlier coverage snapshot
for authentication/navigation, provisioning, scheduling, learning, uploads and
notifications. See [the current portal delivery record](portal-workspace-implementation.md)
for implemented workflows, remaining boundaries and verification.

This status is the handoff boundary for the current R0 + R1 build. A database
table, route placeholder, provider port, or dashboard shell is not counted as a
complete product workflow.

The approved brown Open Play design is integrated across public pages, account
access, role workspaces, support, and the classroom. Public copy and existing API,
permission, and demo workflows are retained. The original logo uses a shared
circular crop. See `design/BRAND.md` for the shared tokens and asset guidance.
This UI integration does not change the production workflow coverage below.

## Ready for continued integration

| Area | Delivered |
| --- | --- |
| Public site | Responsive acquisition pages, programs/curriculum/coaches/pricing/policies routes, five retained city routes, blog URLs, redirects, metadata, sitemap/robots, accessible shared components, and a brand social card. |
| Editorial | Sanity is optional and limited to blog/resources; production enablement validates its environment and publication timestamps. |
| Public API | PostgreSQL-backed organization/program/coach/curriculum/pricing/policy reads, normalized persistent leads, and direct trial-slot booking with future-slot/age validation, exact request replay, and serialized seat allocation. |
| Identity | Atomic single-use magic-link and student-code exchanges, strict mailbox identity parsing, hashed opaque sessions, rotation with original absolute expiry, synchronizer CSRF, `/me`, device listing/revocation, enforced staff TOTP enrollment/challenge, replay-safe encrypted factors, hashed single-use recovery codes, and audited super-admin reset. |
| Authorization | Live database role, family relationship, explicit grant, and assigned-coach checks with deny precedence; guardian-grant mutations are audited. |
| Reports | Coach draft/edit, academic publication, correction versions, guardian view, and acknowledgement with immutable published rows; viewer/acknowledgement responses exclude unpublished correction drafts. |
| Classroom core | Separate realtime process, cookie/origin/subprotocol validation, first-frame one-use tickets, legal move/FEN/PGN adapter, expected revisions, PostgreSQL ordering, Redis fanout, snapshots, replay, and browser reconnect/resume. Commands recheck current sessions, roles, assignments, and class availability; frame sizes and connection deadlines are bounded. Private student answers cannot enter the shared board event stream. |
| Entitlements | PostgreSQL-backed available/reserved/consumed transfer ledger, account-scoped idempotent grants, nonnegative and overflow-safe balances, immutable history pagination, relationship/staff authorization, audited finance writes, and recent-MFA enforcement. |
| Family support | Create/list/view/reply/assign/status REST workflow with optimistic versions, SLA timestamps, append-only events and audits, independent operator permission, SQL-level internal-note redaction, and guardian/operations portal workspaces. |
| Persistence | PostgreSQL 18 migrations for the planned domains, UUIDv7, append-only evidence ledgers, published-record immutability, pgx/sqlc repositories, and River migration validation/transactional enqueue boundary. |
| Operations | Separate API/worker/realtime container targets, local infrastructure, structured logs, request IDs, health/readiness, metrics, bounded Fiber settings, strict JSON, exact CORS/origin middleware, route rate-limit infrastructure, CI, and generation drift checks. |

## Local demo workflow coverage

Run `npm run dev:demo` and open `/demo` to explore the sample academy without
PostgreSQL, Redis, email, payment, or video credentials. This mode requires
`NEXT_PUBLIC_DEMO_MODE=true` and is disabled in production builds. Its
implementation lives in `app/lib/demo/`, with the classroom and portal views
using the same API client request/error paths through a browser-only mock
transport.

| Area | Available local workflow |
| --- | --- |
| Sample identity | Switch among guardian, student, coach, academic manager, operations, finance, and super-admin accounts; sample email sign-in and student code `DEMO2700`. |
| Acquisition | Submit contact enquiries, deduplicate sample leads, choose group or individual trial slots, book available seats, and view/update leads through operations. |
| Family | Add a child, record sample versioned guardian consent, review classes and published progress reports, and acknowledge a published report. |
| Classroom | Join the current sample lesson, play legal chess moves, transfer student board control, undo/reset as coach, import FEN/PGN, and export PGN. Browser tabs share board updates and reject stale revisions. An explicitly joined demo board resumes after a tab reload; leaving clears that tab’s join preference. |
| Teaching | Complete a started class after recording every student's attendance and lesson notes; submit homework as student and review it as coach; publish a sample draft report as academic manager. |
| Billing and scheduling | Simulate an invoice payment success or failure, prevent duplicate invoice credits, consume a credit on lesson completion, and cancel a scheduled class before the sample 24-hour cutoff. |
| Support and operations | Open/reply to family support cases, manage assignment/status as operations, redact internal notes from guardian views, and inspect local activity history. |
| Repeatable scenarios | Reset the sample academy; shared records persist in localStorage and the selected account is kept separately in each tab's sessionStorage. |

These previews use one fictional family and simplified sample policies. Sample
prices, class credits, consent, account switching, and activity history are
browser state; they do not establish real payments, legally valid consent,
server authorization, immutable audit evidence, or complete production policy
behavior. The classroom preview has no video/audio service or networked
WebSocket backend. It shares a board across tabs on one browser only.

`npm run test:demo` exercises local browser journeys. Backend regression tests
separately cover trial request replay/capacity, live classroom revocation,
shared-board privacy, and unpublished report privacy. This local coverage does
not remove the production workflow gaps below.

## Scaffolded, but not a complete workflow

| Area | Existing foundation | Missing before enablement |
| --- | --- | --- |
| Scheduling and attendance | Domain schema, policy services, occurrence/attendance tables | Recurrence worker, exception APIs, DST/conflict integration, seat holds, presence-to-attendance completion, cancellation preview/commit |
| Billing and entitlements | Provider adapter, webhook receipt boundary, durable balance/history/grant ledger API | Checkout, subscriptions, invoice/refund/offline-payment controllers, receipt consumers, reconciliation/dunning, and scheduling-driven reservation/consume/reverse orchestration |
| Provider communications | Resend/SMTP/fake ports, Mailpit, raw-byte signature verification, immutable digest receipts, and replay/conflict handling for Razorpay, Resend, and Daily | Versioned template administration, River delivery handlers, bounce-state projection, payment/classroom receipt consumers, emergency broadcasts |
| Video classroom | Daily adapter with private/time/role bounds and recording/transcription disabled | Room lifecycle, waiting/admission controls, browser embed and provider-state reconciliation |
| Learning | Assignment/progress/resource schema and monthly-report vertical slice | Assignment-type controllers, attempts, alternative solutions, review/resubmission, skill-evidence APIs |
| Files | S3Mock and ClamAV local services, quarantine/clean schema | Upload validation controller, malware worker, promotion, authorization-checked signed downloads, evidence holds |
| Staff and operations | Permission catalog, audit model, enforced staff MFA, role dashboard shells, and family-scoped support-case operation | Coach activation/suspension, CRM operations, imports/exports, bulk execution, global support queue/discovery, safety workflows, finance/admin command center, restricted impersonation |

## Production gates

No checkout, minor onboarding, staff portal, upload, or provider-driven class
workflow should be enabled merely because its schema exists. Complete the
missing workflow, authorization/security tests, provider sandbox scenario,
accessibility run, observability and discard handling, backup/restore drill,
load test where applicable, and the legal/safeguarding/pilot approvals listed
in `release-scope.md` first.
