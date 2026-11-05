# Security model

## Sessions and credentials

- Adult users receive single-use email magic links with a 15-minute lifetime.
- Students exchange guardian-generated, high-entropy, single-use access codes
  with a 10-minute lifetime for restricted sessions.
- Coaches and staff are blocked after the primary email factor until they enroll
  or complete TOTP MFA. Already verified staff can repeat the challenge to
  refresh the short recent-MFA window before sensitive actions. TOTP secrets
  use AES-256-GCM at rest; confirmation and
  challenge rotate the session atomically, reject reused TOTP time steps, and
  issue hashed single-use recovery codes. Super-admin resets require recent
  MFA, a reason, an active different staff target, audit evidence, and revoke
  the target's sessions.
- Only hashes of opaque session tokens are persisted. Production uses
  `__Host-kg_session` with `Secure`, `HttpOnly`, `SameSite=Lax`, and `Path=/`.
- Session rotation, idle and absolute expiry, device listing, device revocation,
  and revoke-all are server-owned. Immediate session invalidation hooks for
  every suspension and permission-change workflow remain a production gate.

The current MFA ciphertext format is versioned, but only one encryption key is
configured at a time. A reviewed dual-read/key-rewrap migration must exist
before rotating a production `KG_TOTP_ENCRYPTION_KEY`.

Magic-link tokens are consumed on the API origin. The API sets the cookie and
redirects to the web application without the token, preventing the credential
from entering frontend logs, analytics, or referrers.

## Browser request defenses

Credentialed CORS uses exact origins. Mutations must pass both strict `Origin`
validation and a session-bound synchronizer CSRF token. The CSRF value is held
only in browser memory. WebSocket handshakes independently validate the origin,
session, subprotocol, and first-frame classroom ticket.

Security headers, bounded Fiber body/read/write limits, provider and database
deadlines, rate limits, and generic outward-facing problem details provide
defense in depth. Internal errors are logged once with request correlation and
without credentials or child-sensitive data.

## Authorization invariants

- Every resource check is relationship-scoped on the server; route visibility
  is not authorization.
- Primary guardians control consent, billing, invitations, and child recovery.
- Secondary guardians start with schedule, classroom, assignment, report, and
  family-thread access; every additional permission is explicit and audited.
- Coaches cannot create private threads with minors. Family and class threads
  remain guardian-visible.
- Private coach notes, safeguarding incidents, financial operations, consent,
  exports, and impersonation have independent permission checks.
- Entitlement reads are scoped to live staff permissions or the guardian's
  billing grant. Manual finance grants require a separate permission, CSRF,
  recent MFA, account-scoped idempotency, and an audit row in the same database
  transaction as the immutable ledger entry.
- Family support replies and staff-only notes share an append-only timeline,
  but customer reads include a SQL visibility predicate as well as service
  redaction. Operations access is independent from the family communication
  grant, and every assignment or status transition is version checked.
- Staff impersonation is not exposed in this implementation. Its 15-minute,
  reason, recent-MFA, banner, audit, and restricted-operation rules remain
  mandatory acceptance criteria before it may be added.

## Files and providers

The local stack provides S3Mock and ClamAV for the intended private-quarantine
pipeline, but upload controllers, promotion, and authorized download issuance
are not yet implemented. No production upload feature should be enabled until
that closed pipeline and its malicious-file tests pass. Provider secrets come
from the deployment secret store and are never browser-exposed.

Webhook receipt endpoints verify raw request bytes and durably deduplicate
provider events before downstream work. State-transition/reconciliation
consumers must still be completed before payments or notification delivery are
enabled. The Daily provider adapter creates private, room-bound, time-bound
tokens and explicitly disables recording and transcription; full room lifecycle
orchestration and its UI are still launch work.

## Required verification before production

Run the threat-model review, IDOR and CSRF suite, malicious-file scenarios,
dependency/license scans, `go test -race ./...`, backup restoration drill, and
provider sandbox tests. Rotate all example secrets and use separate values for
staging and production.
