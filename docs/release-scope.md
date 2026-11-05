# R0 + R1 release scope and gates

For current portal workflow coverage, use the
[September 2026 delivery record](portal-workspace-implementation.md). It supersedes
the older scheduling, learning, upload and notification gaps listed here; unrelated
commerce/provider capabilities and operational launch inputs remain separate.

This document prevents a broad product requirement from being mistaken for an
automatic production approval. Checked code supplies R0 and several end-to-end
R1 foundations. It does not yet supply every workflow in the PRD. The detailed
delivery boundary is recorded in `implementation-status.md`; legal content,
verified claims, provider credentials, unfinished workflows, and operational
drills remain explicit launch inputs.

## Implemented release slices

- Trustworthy responsive acquisition pages, optional editorial content,
  versioned public-policy reads, persistent lead capture, and direct trial-slot
  booking with resilient public fallbacks.
- Single-use adult magic links, student access-code exchange, opaque rotating
  sessions, session-bound CSRF, device listing/revocation, exact-origin CORS,
  enforced staff TOTP/recovery, relationship-scoped guardian grants, and
  database-backed authorization.
- Versioned PostgreSQL models and integrity triggers across the planned domains,
  plus immutable published policy, curriculum, and report records and
  append-only audit/consent/provider/payment/entitlement evidence.
- Board-first classroom UI, authenticated first-frame tickets, legal chess
  validation, optimistic durable revisions, snapshots/replay, and Redis fanout
  after PostgreSQL commit.
- Monthly report draft/edit/publish/correct/view/acknowledge transitions with
  immutable published versions and academic publication permission.
- Durable entitlement account balances and history with append-only bucket
  transfers, exact idempotent finance grants, audited writes, and fresh-MFA
  enforcement.
- Family-scoped support cases with SLA timestamps, optimistic transitions,
  guardian-visible replies, operator-only notes, structural SQL redaction, and
  guardian/operations portal entry points.
- Independently deployable API, worker, and realtime images, application and
  River migration utilities, checked-in REST/WebSocket contracts, and local
  PostgreSQL/Redis/S3Mock/Mailpit/ClamAV infrastructure.

## Not yet launch-ready

- Full coach lifecycle, consent/onboarding, and permission-change session
  invalidation beyond the implemented reset/revoke path.
- Expiring seat holds, reschedule/cancel/assessment/offer workflows, recurrence
  expansion, conflict handling, attendance completion, and cancellation
  entitlement outcomes through REST.
- Razorpay checkout/subscriptions, offline invoice operations, refunds,
  dunning, scholarships, and scheduling/payment-driven entitlement transfers.
- Daily room/embed lifecycle, uploads/quarantine/scanning, assignment and
  progress APIs, notification delivery, global support queue/discovery,
  safety/admin/import/export/bulk/impersonation workflows, and report-SLA workers.
- Provider sandbox journeys, accessibility browser runs, restore drills, and
  the 100-class/700-WebSocket load test.

## Locked policy behavior

- Group target: four. Hard maximum: six. Groups below two are flagged and are
  never automatically cancelled.
- Trials are free and use a session-scoped trial entitlement.
- Groups use monthly subscriptions and dated reservations; 1:1 classes use
  prepaid credits.
- A 1:1 cancellation at least 24 hours before class restores the credit. A late
  cancellation or no-show consumes it unless an approved technical exception
  applies.
- Family absence from a group consumes that session and creates no make-up.
  Academy or provider failure does not consume an entitlement and creates a
  make-up obligation.
- Reports are due on day five of the following month and require academic
  approval within three business days.
- Recording and transcription remain disabled in provider settings, API, and
  UI. Session artifacts contain only PGN, annotations, notes, and assignments.

## Excluded

Recording, transcription, Google Calendar OAuth, waitlists, advanced coach
substitution, AI features, tournaments, schools, marketplace capabilities,
native apps, and multi-tenancy are R1.1-R3 work and are not enabled by a feature
flag in this release.

## Production launch gates

Production activation remains blocked until all of the following are recorded
as approved configuration:

1. Legal entity, GST, canonical contact, refund destination, and invoicing data.
2. Active Terms, Privacy, Cancellation/Refund, Child Safety, Recording-Off, and
   retention policy versions approved by accountable owners.
3. Verified coach profiles, safeguarding evidence, and any published result or
   testimonial claims.
4. Production database, Redis, object storage, email, payment, meeting, malware
   scanning, monitoring, backup, and restore credentials.
5. Security review, accessibility checks, provider sandbox smoke tests, and the
   100-class/700-WebSocket load test.
6. One group and one 1:1 pilot cohort, followed by four weeks of reconciled
   class, payment, entitlement, support, and report records.
7. Approved dry-run imports and a signed spreadsheet-retirement decision.

Checkout and minor onboarding must fail closed while required policy or legal
settings are inactive.
