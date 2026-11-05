# Portal workspace delivery

Updated 22 September 2026. This records the application's connected portal
workflows, roles, curriculum, and verification procedures.

This document supersedes older scheduling, learning, upload and notification
coverage statements in `implementation-status.md` and `release-scope.md`.
Those documents still describe unrelated commerce and production launch inputs.

## Delivered workflows

| Area | Implemented behavior |
| --- | --- |
| Identity | Home/logo/academy links retain the workspace. A remembered browser restores short sessions; explicit logout and device revocation revoke it. Suspension/security resets are enforced. Staff retain TOTP and fresh-MFA checks. |
| Academy setup | Organization identity, versioned policies, staff/guardian/learner provisioning, guardian consent, single-use student codes, batches, dated enrollment and audited credit grants. |
| Scheduling | Agenda/day/week/month views, local timezone, single or bounded weekly classes, preview, conflict/capacity/availability/consent/funding checks, atomic series, class assignments and classroom creation. |
| Teaching operations | Edit one or future occurrences, HTTPS meeting links, attendance, notes, completion and policy-based cancellation. Academy failure restores credit and records a make-up. An existing reserved future class can fulfill it without another credit grant. |
| Trials and enquiries | Published public trial slots, capacity-safe bookings, stages/notes, outcomes, atomic rescheduling, enrolled-learner conversion and coach trial visibility. |
| Chess tools | Legal moves, position setup, PGN/FEN import/export, variations, annotations, timer/preferences and local Stockfish analysis. Approved lesson state can be sent to the authoritative classroom. |
| Learning | Personal/shared resources, publishing, curriculum metadata, resource snapshots in assignments, individual/batch homework, alternative puzzle solutions, hints/reveal, attempts, submission, return and review. |
| Game review | Student game records, replay, coach feedback and board analysis. No new matchmaking or rating system. |
| Progress | Versioned rubrics, assessment submission/return/approval, report publication and correction, guardian acknowledgement, certificates/PDF attachments and explicit versioned curriculum placement preserving legacy levels. |
| Communication | Guardian-visible individual conversations, batch announcements, unread state, in-app notifications and durable email. Private message/lesson bodies are excluded from email summaries. |
| Workspace | Profile/timezone, saved views, device sessions, search, load-more indicators, pending work and current/prior-month completed class/trial hours. |

Workload counts scheduled minutes of completed classes. Trial hours count a
slot once when at least one booking is attended, rather than once per attendee.
Monthly totals use UTC calendar months and all authorized records independently
of discovery limits. These are workload measures, not payroll or measured video
call duration.

## Roles

| Role | Scope |
| --- | --- |
| Coach | Assigned active learners/batches; scheduling, teaching, homework and feedback. |
| Operations | Academy setup, enrollment/scheduling, enquiries and trials. |
| Academic manager | Academy-wide teaching/homework, resource/report publication, curriculum placement and schedule visibility. |
| Super administrator | Operational and academic capabilities, plus staff/policy administration. |
| Finance | Authorized enrollment credit/billing views and audited grants. |
| Guardian | Their active learner relationships and individual permissions; consent and permitted family cancellation. |
| Student | Own schedule, published learning and submissions. |

Existing UUIDv4 account and record references remain supported alongside new
UUIDv7 identifiers across academy setup, learning, credit authorization, family
and report access, trials, support and classroom tickets. This compatibility does not grant permissions or change
stored IDs.

Server authorization uses current roles and relationships. Explicit global,
family and student denies take precedence. Guardians with profile access alone
do not obtain class schedules; notification delivery rechecks current access.

## Curriculum

The public curriculum and portal use the same five academy levels:

| Track | Level | Identifier |
| --- | --- | --- |
| Foundation | Board foundations | `foundation-1` |
| Foundation | Tactical foundations | `foundation-2` |
| Development | Calculation and planning | `development-1` |
| Development | Competitive habits | `development-2` |
| Performance | Advanced performance | `performance-1` |

Academic managers record placement against the published curriculum version,
with a published learner report and reason. Placement history is append-only;
legacy batch levels and older published records remain intact.

## Starting an empty academy

1. Apply migrations, bootstrap the first super administrator and complete MFA.
2. Save the actual academy legal/contact information and publish approved
   privacy and child-safety policies in Settings. Publishing requires fresh MFA.
3. Create coach/staff accounts and a learner/guardian relationship. The guardian
   signs in and accepts the current policies.
4. Create a batch with its coach, format, level, timezone and capacity. Enroll
   the learner with valid dates.
5. Finance or the super administrator grants approved credits with an auditable
   reference and fresh MFA. This is not payment checkout.
6. Preview a class or weekly schedule with an explicit end date, then confirm.
   Insufficient credit or any conflict leaves the whole series unchanged.
7. Publish learning material, assign homework, teach, record attendance/notes
   and review submitted work.

Groups have at most six seats; individual batches have one. Scheduling reserves
existing credits. Family absence from a group consumes a reserved credit while
leaving classmates' session active. A later academy cancellation reverses that
consumption when the academy cannot deliver the group class.

## Persistence and retries

Migrations 11–17 add remembered browsers, academy operations, learning,
quarantined files, shared lesson state, curriculum placement and immutable
published resource versions. API, worker,
realtime and frontend should deploy from one revision. River migrations are
separate; `make migrate` runs both sets.

`GET /api/v1/academy/workspace?limit=N`, `/academy/operations?limit=N` and
`/learning/workspace?limit=N` return
`pagination: {limit, hasMore, nextLimit}`. The initial limit is 500; load more
until `hasMore` is false. Search applies to loaded records. Authorization IDs and
workload totals do not inherit that limit. Class discovery includes older and
future classes; load more to navigate beyond the initially loaded records.

Send stable `Idempotency-Key` headers for retried JSON creations, learning actions
and uploads. Actor/operation/payload digests bind receipts; changed payloads
using a previous key conflict. Receipts commit with mutations. Learning retries
return a freshly authorized workspace. Scheduling, cancellation, completion,
make-up fulfillment and trial rescheduling also carry explicit request keys.
Database locks and optimistic revisions protect concurrent transitions.

## Runtime services

Meeting links open HTTPS Zoom/Meet rooms separately from the shared chess board.
Embedded Daily video, recording and transcription are not part of this delivery.
Stockfish runs in a browser worker; its self-hosted license/source assets must
remain available. See [deployment](deployment.md).

Uploads use private quarantine/clean buckets and ClamAV. Files become available
only after validation, scanning, promotion and authorized database publication.
PGN, FEN, PDF, PNG and JPEG have size/content checks. Every download rechecks
authorization; failed/rejected uploads remain unavailable.

Schedule/homework/message events persist `notifications` in their transaction.
The River outbox sweeper runs at startup and every 30 seconds. SMTP/Resend
dispatch records attempts and backs off failures, suppressing stale reminders,
submitted homework reminders and revoked recipients. Class/trial reminders use
24 hours and one hour before the event when still future. Homework reminders
use the due date minus 24 hours. In-app records survive email provider failure.

## Verification record

Verified locally against PostgreSQL 18 and a production Next.js build. Test
accounts and schemas are isolated from normal academy data.

| Check | Result |
| --- | --- |
| `npm run check` | Passed ESLint, TypeScript and all 152 frontend tests. |
| `npm run build` with isolated workspace configuration | Passed production compilation and static/dynamic route generation. |
| `go test -p 1 -race -tags=integration ./...` | Passed the full backend suite against PostgreSQL; final resource-version, communication authorization and auth-controller changes also passed focused race/integration reruns. |
| S3Mock + ClamAV (`KG_TEST_FILES=1`) | Passed clean upload/download and harmless malware-test rejection. File authorization, quarantine failures and concurrent replay also passed. |
| Demo Playwright | All 20 journeys passed: 10 desktop and 10 mobile, including keyboard/axe checks, lifecycle forms, files, per-class boards, privacy and staff suspension. |
| Production Playwright | Passed the complete real-account journey: staff/MFA, family/consent/enrollment/credits, scheduling, scanned resources and file responses, homework return/resubmission/review, reports/corrections/acknowledgement, placement/certificate download, realtime replay, attendance, Stockfish, navigation, remembered restoration and explicit logout. |
| Accessibility/layout | Four production desktop/mobile axe scans returned zero violations; classroom and guardian-workspace screenshots reviewed. Named chess pieces and keyboard-accessible resource previews also pass unit and demo browser checks. |
| Contract/assets | Shared frontend/backend types regenerated; all academy/learning routes and DTO fields reviewed. Stockfish WASM, JavaScript, source and NNUE checksums passed. |

The backend coverage includes remembered-device revocation and concurrent
restoration, MFA assurance, explicit permission denies, family/coach scope,
legacy UUIDv4 staff completing batch setup, credit grants, scheduling, attendance
and classroom joins, atomic recurrence/capacity/credit checks, daylight-saving
gaps/folds, cancellation
reversals and make-up concurrency. Learning coverage includes private puzzle
solutions, submitted-attempt metrics, immutable assigned content/reports,
legacy-draft adoption, authorized downloads after resource revisions, large shared-file
assignment sets and current enrollment dates for batch conversations. Worker
coverage exercises actual River migration/outbox insertion, dispatch retries,
stale reminder suppression and revoked family access.

Parallel package migrations exceeded the local PostgreSQL lock-table capacity;
the complete integration run passed with `-p 1`, also used in CI. The production
browser harness creates a dedicated disposable Redis instance so retries cannot
consume normal local users' sign-in rate limits. Application limits are retained.

Deployment still requires the configured production database, private buckets,
scanner, email delivery and realtime proxy/origin settings described in
[deployment.md](deployment.md). No production deployment or external Zoom/Meet
meeting was performed. Browser/provider tests use local services; they do not
establish production credentials, deliverability or backup/restore readiness.

## Boundaries

Weekly recurrence is bounded materialized occurrences, not external calendar
sync. Enrollment changes require resolving affected reservations; automatic
batch transfer/payment renewal is separate. Advanced substitution, payroll,
bulk imports, live matchmaking/ratings, checkout/subscriptions/refunds,
recording, transcription and multi-tenancy remain outside this expansion.
Approved policies, production credentials and backup/restore exercises are
operational inputs and cannot be inferred from sample data.
