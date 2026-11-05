# Database entity catalog

Verified on 22 September 2026 against application migrations 1–17, repository queries, and the local PostgreSQL `public` schema: **95 tables: 88 application tables and 7 migration/job infrastructure tables**.

This describes persisted entities, not individual customer records or login credentials. See [product architecture](architecture.md) for the component diagram and connection details.

**Runtime** means an application read/write path exists; it does not promise that an optional provider is configured or every administrative capability has a portal screen. **Foundation** means a schema exists but no runtime application read/write path was found; do not treat it as a completed feature. **Infrastructure** means migrations or River own the table. Seeded permission definitions remain part of runtime authorization through their related grants.

## Identity and sign-in

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `users` | Account identity, email, display name, and lifecycle status; referenced by roles, staff, guardians, students, and authored records. | Runtime |
| `role_grants` | Current/revoked roles per user; one user can have several roles. | Runtime |
| `auth_password_credentials` | Username and password hash per user; stores no recoverable plaintext password. | Runtime |
| `auth_password_registrations` | Expiring mailbox-verification requests carrying pending username/password hashes; consumed before credentials become usable. | Runtime |
| `auth_magic_links` | Hashed single-use email sign-in tokens, expiry, consumed state, and intended return path. | Runtime |
| `auth_sessions` | Hashed access-session/CSRF credentials, finite expiry, MFA assurance, revocation, and optional remembered-device reference. | Runtime |
| `auth_devices` | Hashed revocable remembered-device credentials linked to users; associated sessions reference the device. | Runtime |
| `student_login_codes` | Hashed, expiring, single-use learner access codes and the issuing guardian/staff user. | Runtime |
| `staff_mfa_methods` | Encrypted TOTP secrets, verification, and disabled state per staff user. | Runtime |
| `staff_mfa_recovery_codes` | Hashed single-use recovery codes associated with a staff MFA method. | Runtime |
| `auth_google_identities` | Google subject-to-local-user mapping. | Runtime; requires configured Google sign-in |
| `auth_google_challenges` | Hashed expiring Google sign-in nonces. | Runtime; requires configured Google sign-in |

## Authorization

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `permission_definitions` | Seeded registry of permission codes referenced by defaults and explicit grants. | Runtime |
| `role_permission_defaults` | Default permission codes for each academy role. | Runtime |
| `permission_grants` | Explicit user allow/deny grants scoped globally, to a family, or to a learner, including expiry and revocation. Deny takes precedence. | Runtime |

## Academy identity, policies, and catalog

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `organization_settings` | Single academy identity/contact record used by public pages and consent prerequisites. | Runtime |
| `policies` | Stable policy identity and slug; content belongs to its versions. | Runtime |
| `policy_versions` | Versioned policy content, locale, purpose, effective date, and publication status. | Runtime |
| `consent_records` | Acceptance/withdrawal evidence linked to an exact policy version, subject, and acting guardian/user. | Runtime |
| `programs` | Public and operational teaching programs, such as group and individual coaching. | Runtime |
| `plans` | Program pricing/catalog entries and cancellation-policy snapshots; does not itself implement checkout. | Runtime catalog/policy reads |
| `coach_profiles` | Coach profile attached to a user, with optional public biography/visibility. | Runtime |
| `curriculum_versions` | Versioned published curriculum JSON; preserves both original and newer curriculum versions. | Runtime |

## Families and learners

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `families` | Family grouping and lifecycle metadata. | Runtime |
| `student_profiles` | Learner-specific profile attached to a user: preferred name, age band, country, timezone, language, and lifecycle. | Runtime |
| `guardian_student_relationships` | Family/guardian/learner links with individual permissions and active/revoked status; governs family visibility. | Runtime |

## Leads and trials

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `leads` | Prospective family contact, learner experience, preferences, consent, pipeline stage, owner, follow-up notes, and converted learner reference. | Runtime |
| `lead_stage_events` | Timeline of pipeline stage changes for a lead. | Runtime |
| `campaign_touches` | Acquisition attribution attached to leads: campaign, source, landing page, and referrer. | Runtime |
| `trial_slots` | Published/draft trial availability, capacity, assessor, format, and timezone. | Runtime |
| `trial_bookings` | A lead's reservation of a trial slot, contact snapshot, outcome, timing, notes, and idempotency key. | Runtime |

## Enrollment, scheduling, and attendance

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `batches` | Named teaching group/individual cohort with program, level, coach, format, timezone, and capacity. | Runtime |
| `enrolments` | Learner's enrollment in a batch/plan, lifecycle, dates, and fixed policy snapshot; owns an entitlement account. | Runtime |
| `class_sessions` | Saved class occurrence with batch, timing, topic/title, meeting URL, notes, lifecycle, optimistic revision, and optional recurrence rule. | Runtime |
| `session_assignments` | Users permitted to access a particular classroom, their participant role, and assignment validity interval. | Runtime |
| `academy_class_participants` | Per-class learner roster linked to the enrollment and its reserved/consumed/released credit state. | Runtime |
| `attendance` | Coach/staff-resolved attendance per class and learner, including resolution actor and time. | Runtime |
| `recurrence_rules` | Versioned local-time recurrence definitions with timezone and effective dates. | Runtime |
| `recurrence_exceptions` | Date-specific skipped or replaced occurrences for a recurrence rule. | Runtime |
| `coach_availability_windows` | Available/blocked coach intervals used by scheduling checks and operational management. | Runtime |
| `academy_make_up_obligations` | Make-up owed to one learner for an original class, with fulfillment/cancellation state and replacement-class reference. | Runtime |
| `presence_intervals` | Schema for join/leave telemetry by class, user, role, and source; distinct from coach-confirmed attendance. | Foundation; no presence ingestion path found |

## Student credits

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `entitlement_accounts` | One `session_credit` account per enrollment. The learner is reached through `enrolments.student_profile_id`. | Runtime |
| `entitlement_entries` | Append-only ledger of grants, reservations, consumption, releases, reversals, expiry, refunds, and adjustments. Stores available/reserved/consumed deltas and resulting balances, actor, reason, class/reference, and idempotency information. | Runtime |

Credits are not a field on the student profile. An enrollment's balance is derived from its ledger; scheduling and completion change credit buckets transactionally. See [the ledger migration](../backend/migrations/000007_entitlement_ledger.up.sql) and [repository](../backend/internal/domain/entitlement/postgres_repository.go).

## Live classroom

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `classroom_states` | Current authoritative shared board per class: FEN, moves, mode, revision, active actor, and lesson tools/annotations. | Runtime |
| `classroom_events` | Ordered shared classroom event log for synchronization/replay, keyed by class and revision. | Runtime |
| `classroom_snapshots` | Saved class state at a revision for recovery/replay. | Runtime |
| `realtime_join_tickets` | Hashed single-use, expiring WebSocket admission tickets bound to a user and class. | Runtime |

## Homework, resources, files, and games

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `learning_assignments` | Versioned learner assignment with instructions, type, content/puzzles, deadline, author, optional class, and draft/assigned/closed state. | Runtime |
| `assignment_attempts` | Numbered learner attempts with submitted responses, puzzle results/practice information, review state, feedback, and reviewer. | Runtime |
| `assignment_evidence` | File evidence associated with an attempt: storage metadata, digest, scan state, and teaching-file reference. | Runtime |
| `teaching_resources` | Coach/published academy resource metadata and current content; PGN, FEN, study text, PDF, or image, with visibility, level, tags, and version. | Runtime |
| `teaching_resource_versions` | Append-only snapshots of published resource versions, preserving material referenced by historical work. | Runtime |
| `teaching_files` | Private uploaded-object metadata, owner/optional learner, content digest, and quarantine/clean/rejected/failed scan status. File bytes reside in object storage. | Runtime |
| `student_games` | Saved/imported learner PGN, source, date, optional class, review text, sharing state, and version; supplied ratings remain in PGN metadata. | Runtime |

## Progress, reports, and curriculum placement

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `progress_reports` | Learner/reporting-period identity and draft/review/published lifecycle. | Runtime |
| `progress_report_versions` | Authored/published content versions belonging to a report. | Runtime |
| `learning_rubrics` | Level-specific, versioned assessment dimensions. | Runtime |
| `learning_assessments` | Assessment attached to a learner, rubric, report, and exact report version; contains ratings/evidence, review workflow, publication, and guardian acknowledgement. | Runtime |
| `learning_assessment_history` | Append-only assessment transition/content history with actor and timestamp. | Runtime |
| `learning_placements` | Explicit curriculum/level placement changes linked to the supporting report version and academic decision. | Runtime |
| `learning_certificates` | Certificate issuance/revocation linked to learner, qualifying assessment, level, issuer, and optional PDF file. | Runtime |

## Conversations, notifications, and preferences

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `learning_threads` | Conversation attached to either one learner or one batch; membership is authorized from current relationships/assignments. | Runtime |
| `learning_messages` | Durable messages within a conversation, authored by a user. | Runtime |
| `learning_thread_reads` | Per-user conversation read timestamp for durable unread state. | Runtime |
| `learning_notifications` | Recipient-specific in-app notification, portal link, optional learner, and read timestamp. | Runtime |
| `notifications` | Transactional scheduled delivery outbox; current delivery implementation uses email academy events. | Runtime |
| `notification_delivery_attempts` | Delivery history/retries with provider ID, outcome, and failure information for an outbox notification. | Runtime |
| `notification_templates` | Schema for versioned reusable channel/locale templates; current email renderer is implemented in code. | Foundation; no template-table loading path found |
| `academy_user_preferences` | Per-user timezone and saved portal list views. | Runtime |

## Support and operational foundations

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `support_cases` | Family-linked support case, assignee, severity, status, SLA, resolution, and optimistic version. | Runtime |
| `support_case_events` | Append-only support timeline with internal/customer-visible scope. | Runtime |
| `incidents` | Proposed incident investigation record linked to a class and/or support case, with severity and ownership. | Foundation |
| `evidence_holds` | Proposed retention hold/release on a referenced object. | Foundation |
| `coach_verifications` | Proposed versioned coach checks and evidence-object references with verification/expiry/revocation state. | Foundation |
| `data_jobs` | Proposed bulk import/export/action job metadata with input/output storage objects and progress. | Foundation |
| `data_job_row_results` | Proposed per-row result/error detail belonging to a data job. | Foundation |
| `feature_flag_versions` | Proposed audited feature-flag version and targeting rules. | Foundation; current environment flags are configured separately |
| `impersonation_sessions` | Proposed audited staff impersonation session, reason, ticket, approval, and expiry. | Foundation; does not establish a usable impersonation feature |

## Payment foundations

Credit administration works independently of these checkout tables. Their existence does not mean purchases, provider reconciliation, invoices, or refunds are implemented end to end.

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `customer_orders` | Proposed family/plan order, monetary totals, policy snapshot, status, and idempotency key. | Foundation |
| `payment_transactions` | Proposed provider authorization/capture/refund/chargeback events associated with an order. | Foundation |
| `invoices` | Proposed versioned order invoice or credit note, number, totals, tax breakdown, and optional document object. | Foundation |
| `refund_requests` | Proposed request/approval/processing lifecycle for refunding an order. | Foundation |
| `provider_ledger_entries` | Proposed provider settlement/reconciliation entries optionally linked to an order and payment transaction. | Foundation |

## Audit, webhooks, and request integrity

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `audit_events` | Append-only record of security/business transitions with actor, object, request/idempotency references, reason, and metadata. | Runtime |
| `webhook_receipts` | Durable deduplication/processing metadata for verified provider webhook deliveries; not itself a completed payment or meeting workflow. | Runtime |
| `academy_request_results` | Stored request digest/result keyed by actor, operation, and idempotency key; used by current academy/learning mutation protection. | Runtime |
| `idempotency_records` | Original generic request/result persistence schema. Current workspace code uses `academy_request_results` and operation-specific keys instead. | Foundation; no runtime table access found |

## Database and job infrastructure

| Table | Purpose and relationships | Status |
| --- | --- | --- |
| `schema_migrations` | Application migration version and dirty flag; currently version 17 with no dirty migration in the inspected local database. | Infrastructure |
| `river_migration` | River's separate job-schema migration history. | Infrastructure |
| `river_job` | Durable queued/scheduled/running/completed/failed jobs, arguments, attempts, deduplication, and errors. | Infrastructure |
| `river_queue` | River queue metadata and pause state. | Infrastructure |
| `river_leader` | River leader-election lease for coordinated maintenance. | Infrastructure |
| `river_client` | River-managed client metadata schema; not an academy account. | Infrastructure |
| `river_client_queue` | River-managed per-client queue metadata/counters, linked to `river_client`. | Infrastructure |

## Scope of the verification

The catalog reconciles every application `CREATE TABLE` in [backend migrations](../backend/migrations) with the seven infrastructure tables reported by the local database. Runtime/foundation labels were checked against non-test repository/service queries, not inferred from table names or the presence of test fixtures. Optional Google sign-in, email delivery, scanning, and object storage still depend on runtime configuration.

The schema defines **15 foundation tables without a runtime table-access path**. In particular, schema support for payment providers, automated presence, bulk data jobs, and impersonation must not be confused with enabled portal workflows. The operational code paths are in [academy](../backend/internal/domain/academy), [learning](../backend/internal/domain/learning), [authorization](../backend/internal/authorization), [authentication repositories](../backend/internal/auth/repository), and [notification workers](../backend/internal/platform/jobs/notifications.go).
