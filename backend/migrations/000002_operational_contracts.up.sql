-- R1 operational persistence. Event, ledger, delivery-attempt, audit and
-- evidence tables are append-only by application policy; corrections are new
-- versioned rows rather than in-place rewrites.

ALTER TABLE classroom_states
    ADD COLUMN moves text[] NOT NULL DEFAULT '{}',
    ADD COLUMN board_status text NOT NULL DEFAULT 'open'
        CHECK (board_status IN ('open', 'closed'));

CREATE TABLE realtime_join_tickets (
    id uuid PRIMARY KEY,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    user_id uuid NOT NULL REFERENCES users(id),
    class_session_id uuid NOT NULL REFERENCES class_sessions(id),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL
);
CREATE INDEX realtime_join_tickets_expiry_idx
    ON realtime_join_tickets (expires_at) WHERE consumed_at IS NULL;

CREATE TABLE recurrence_rules (
    id uuid PRIMARY KEY,
    owner_type text NOT NULL CHECK (owner_type IN ('batch', 'coach_availability', 'trial_schedule')),
    owner_id uuid NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    rrule text NOT NULL,
    source_time_zone text NOT NULL,
    local_start_time time NOT NULL,
    duration_minutes integer NOT NULL CHECK (duration_minutes > 0),
    effective_from date NOT NULL,
    effective_until date,
    status text NOT NULL CHECK (status IN ('draft', 'active', 'superseded', 'cancelled')),
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL,
    CHECK (effective_until IS NULL OR effective_until >= effective_from),
    UNIQUE (owner_type, owner_id, version)
);
CREATE UNIQUE INDEX recurrence_rules_active_owner_idx
    ON recurrence_rules (owner_type, owner_id) WHERE status = 'active';

CREATE TABLE recurrence_exceptions (
    id uuid PRIMARY KEY,
    recurrence_rule_id uuid NOT NULL REFERENCES recurrence_rules(id),
    source_local_date date NOT NULL,
    action text NOT NULL CHECK (action IN ('skip', 'replace')),
    replacement_local_start timestamp,
    replacement_duration_minutes integer CHECK (replacement_duration_minutes > 0),
    reason text NOT NULL,
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL,
    CHECK ((action = 'skip' AND replacement_local_start IS NULL) OR
           (action = 'replace' AND replacement_local_start IS NOT NULL)),
    UNIQUE (recurrence_rule_id, source_local_date)
);

CREATE TABLE presence_intervals (
    id uuid PRIMARY KEY,
    class_session_id uuid NOT NULL REFERENCES class_sessions(id),
    user_id uuid NOT NULL REFERENCES users(id),
    participant_role text NOT NULL CHECK (participant_role IN ('student', 'guardian', 'coach', 'observer', 'operations_host')),
    joined_at timestamptz NOT NULL,
    left_at timestamptz,
    source text NOT NULL CHECK (source IN ('meeting_webhook', 'realtime', 'manual')),
    provider_event_id text,
    recorded_at timestamptz NOT NULL,
    CHECK (left_at IS NULL OR left_at >= joined_at),
    UNIQUE (source, provider_event_id)
);
CREATE INDEX presence_intervals_session_user_idx
    ON presence_intervals (class_session_id, user_id, joined_at);

CREATE TABLE learning_assignments (
    id uuid PRIMARY KEY,
    assignment_key uuid NOT NULL,
    student_profile_id uuid NOT NULL REFERENCES student_profiles(id),
    class_session_id uuid REFERENCES class_sessions(id),
    title text NOT NULL,
    instructions text NOT NULL,
    assignment_type text NOT NULL CHECK (assignment_type IN ('puzzle_set', 'game_review', 'study', 'file_upload', 'other')),
    version integer NOT NULL CHECK (version > 0),
    status text NOT NULL CHECK (status IN ('draft', 'assigned', 'closed', 'archived')),
    due_at timestamptz,
    authored_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL,
    UNIQUE (assignment_key, version)
);

CREATE TABLE assignment_attempts (
    id uuid PRIMARY KEY,
    assignment_id uuid NOT NULL REFERENCES learning_assignments(id),
    student_profile_id uuid NOT NULL REFERENCES student_profiles(id),
    attempt_number integer NOT NULL CHECK (attempt_number > 0),
    submitted_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL CHECK (status IN ('started', 'submitted', 'reviewed', 'returned')),
    submitted_at timestamptz,
    reviewed_by uuid REFERENCES users(id),
    reviewed_at timestamptz,
    feedback text,
    created_at timestamptz NOT NULL,
    UNIQUE (assignment_id, student_profile_id, attempt_number)
);

CREATE TABLE assignment_evidence (
    id uuid PRIMARY KEY,
    assignment_attempt_id uuid NOT NULL REFERENCES assignment_attempts(id),
    object_key text NOT NULL,
    content_type text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    sha256 bytea NOT NULL CHECK (octet_length(sha256) = 32),
    scan_status text NOT NULL CHECK (scan_status IN ('quarantined', 'clean', 'infected', 'failed')),
    scanned_at timestamptz,
    created_at timestamptz NOT NULL,
    UNIQUE (assignment_attempt_id, object_key)
);

CREATE TABLE customer_orders (
    id uuid PRIMARY KEY,
    family_id uuid NOT NULL REFERENCES families(id),
    plan_id uuid NOT NULL REFERENCES plans(id),
    currency char(3) NOT NULL,
    subtotal_minor bigint NOT NULL CHECK (subtotal_minor >= 0),
    tax_minor bigint NOT NULL CHECK (tax_minor >= 0),
    total_minor bigint NOT NULL CHECK (total_minor >= 0),
    status text NOT NULL CHECK (status IN ('draft', 'pending_payment', 'paid', 'cancelled', 'refunded', 'partially_refunded')),
    idempotency_key text NOT NULL UNIQUE,
    policy_snapshot jsonb NOT NULL,
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (subtotal_minor + tax_minor = total_minor)
);

CREATE TABLE payment_transactions (
    id uuid PRIMARY KEY,
    order_id uuid NOT NULL REFERENCES customer_orders(id),
    provider text NOT NULL,
    provider_payment_id text NOT NULL,
    transaction_type text NOT NULL CHECK (transaction_type IN ('authorization', 'capture', 'failure', 'refund', 'chargeback')),
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    currency char(3) NOT NULL,
    status text NOT NULL,
    provider_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL,
    recorded_at timestamptz NOT NULL,
    UNIQUE (provider, provider_payment_id, transaction_type)
);
CREATE INDEX payment_transactions_order_idx ON payment_transactions (order_id, occurred_at);

CREATE TABLE invoices (
    id uuid PRIMARY KEY,
    order_id uuid NOT NULL REFERENCES customer_orders(id),
    invoice_number text NOT NULL UNIQUE,
    version integer NOT NULL CHECK (version > 0),
    currency char(3) NOT NULL,
    total_minor bigint NOT NULL CHECK (total_minor >= 0),
    tax_breakdown jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL CHECK (status IN ('issued', 'void', 'credit_note')),
    issued_at timestamptz NOT NULL,
    object_key text,
    UNIQUE (order_id, version)
);

CREATE TABLE refund_requests (
    id uuid PRIMARY KEY,
    order_id uuid NOT NULL REFERENCES customer_orders(id),
    requested_amount_minor bigint NOT NULL CHECK (requested_amount_minor > 0),
    currency char(3) NOT NULL,
    reason_code text NOT NULL,
    reason_text text,
    status text NOT NULL CHECK (status IN ('requested', 'approved', 'rejected', 'processing', 'succeeded', 'failed')),
    requested_by uuid NOT NULL REFERENCES users(id),
    decided_by uuid REFERENCES users(id),
    requested_at timestamptz NOT NULL,
    decided_at timestamptz,
    updated_at timestamptz NOT NULL
);

CREATE TABLE provider_ledger_entries (
    id uuid PRIMARY KEY,
    provider text NOT NULL,
    provider_account text NOT NULL DEFAULT '',
    provider_entry_id text NOT NULL,
    order_id uuid REFERENCES customer_orders(id),
    payment_transaction_id uuid REFERENCES payment_transactions(id),
    entry_type text NOT NULL,
    amount_minor bigint NOT NULL,
    currency char(3) NOT NULL,
    available_at timestamptz,
    payload_hash bytea NOT NULL CHECK (octet_length(payload_hash) = 32),
    occurred_at timestamptz NOT NULL,
    recorded_at timestamptz NOT NULL,
    UNIQUE (provider, provider_account, provider_entry_id)
);

CREATE TABLE notification_templates (
    id uuid PRIMARY KEY,
    template_key text NOT NULL,
    channel text NOT NULL CHECK (channel IN ('email', 'sms', 'whatsapp', 'in_app')),
    locale text NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    subject_template text,
    body_template text NOT NULL,
    status text NOT NULL CHECK (status IN ('draft', 'active', 'retired')),
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL,
    UNIQUE (template_key, channel, locale, version)
);
CREATE UNIQUE INDEX notification_templates_active_idx
    ON notification_templates (template_key, channel, locale) WHERE status = 'active';

CREATE TABLE notifications (
    id uuid PRIMARY KEY,
    recipient_user_id uuid REFERENCES users(id),
    recipient_address text,
    template_key text NOT NULL,
    template_version integer NOT NULL,
    channel text NOT NULL CHECK (channel IN ('email', 'sms', 'whatsapp', 'in_app')),
    render_context jsonb NOT NULL DEFAULT '{}'::jsonb,
    deduplication_key text NOT NULL UNIQUE,
    scheduled_at timestamptz NOT NULL,
    cancelled_at timestamptz,
    created_at timestamptz NOT NULL,
    CHECK (recipient_user_id IS NOT NULL OR recipient_address IS NOT NULL)
);

CREATE TABLE notification_delivery_attempts (
    id uuid PRIMARY KEY,
    notification_id uuid NOT NULL REFERENCES notifications(id),
    attempt_number integer NOT NULL CHECK (attempt_number > 0),
    provider text NOT NULL,
    provider_message_id text,
    status text NOT NULL CHECK (status IN ('queued', 'sent', 'delivered', 'bounced', 'failed', 'suppressed')),
    error_code text,
    error_detail text,
    occurred_at timestamptz NOT NULL,
    UNIQUE (notification_id, attempt_number),
    UNIQUE (provider, provider_message_id)
);

CREATE TABLE support_cases (
    id uuid PRIMARY KEY,
    family_id uuid REFERENCES families(id),
    opened_by uuid REFERENCES users(id),
    category text NOT NULL,
    severity text NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    status text NOT NULL CHECK (status IN ('open', 'investigating', 'waiting', 'resolved', 'closed')),
    subject text NOT NULL,
    assigned_to uuid REFERENCES users(id),
    opened_at timestamptz NOT NULL,
    resolved_at timestamptz,
    updated_at timestamptz NOT NULL
);

CREATE TABLE support_case_events (
    id uuid PRIMARY KEY,
    support_case_id uuid NOT NULL REFERENCES support_cases(id),
    event_type text NOT NULL,
    actor_user_id uuid REFERENCES users(id),
    body text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL
);
CREATE INDEX support_case_events_case_idx ON support_case_events (support_case_id, occurred_at);

CREATE TABLE incidents (
    id uuid PRIMARY KEY,
    class_session_id uuid REFERENCES class_sessions(id),
    support_case_id uuid REFERENCES support_cases(id),
    incident_type text NOT NULL,
    severity text NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    status text NOT NULL CHECK (status IN ('reported', 'triaged', 'investigating', 'contained', 'resolved', 'closed')),
    reported_by uuid REFERENCES users(id),
    owner_user_id uuid REFERENCES users(id),
    summary text NOT NULL,
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE evidence_holds (
    id uuid PRIMARY KEY,
    object_type text NOT NULL,
    object_id text NOT NULL,
    reason text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'released')),
    placed_by uuid NOT NULL REFERENCES users(id),
    placed_at timestamptz NOT NULL,
    released_by uuid REFERENCES users(id),
    released_at timestamptz,
    UNIQUE (object_type, object_id, placed_at)
);
CREATE UNIQUE INDEX evidence_holds_active_object_idx
    ON evidence_holds (object_type, object_id) WHERE status = 'active';

CREATE TABLE coach_availability_windows (
    id uuid PRIMARY KEY,
    coach_user_id uuid NOT NULL REFERENCES users(id),
    recurrence_rule_id uuid REFERENCES recurrence_rules(id),
    starts_at timestamptz,
    ends_at timestamptz,
    source_time_zone text NOT NULL,
    status text NOT NULL CHECK (status IN ('available', 'blocked', 'cancelled')),
    created_at timestamptz NOT NULL,
    CHECK ((recurrence_rule_id IS NOT NULL) OR (starts_at IS NOT NULL AND ends_at > starts_at))
);
CREATE INDEX coach_availability_lookup_idx
    ON coach_availability_windows (coach_user_id, starts_at, ends_at) WHERE status = 'available';

CREATE TABLE coach_verifications (
    id uuid PRIMARY KEY,
    coach_user_id uuid NOT NULL REFERENCES users(id),
    verification_type text NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    status text NOT NULL CHECK (status IN ('pending', 'verified', 'rejected', 'expired', 'revoked')),
    evidence_object_key text,
    verified_by uuid REFERENCES users(id),
    verified_at timestamptz,
    expires_at timestamptz,
    created_at timestamptz NOT NULL,
    UNIQUE (coach_user_id, verification_type, version)
);

CREATE TABLE data_jobs (
    id uuid PRIMARY KEY,
    job_type text NOT NULL CHECK (job_type IN ('import', 'export', 'bulk_action')),
    resource_type text NOT NULL,
    requested_by uuid NOT NULL REFERENCES users(id),
    idempotency_key text NOT NULL UNIQUE,
    input_object_key text,
    output_object_key text,
    parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    total_rows integer CHECK (total_rows >= 0),
    processed_rows integer CHECK (processed_rows >= 0),
    error_summary jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL,
    started_at timestamptz,
    completed_at timestamptz
);

CREATE TABLE data_job_row_results (
    id uuid PRIMARY KEY,
    data_job_id uuid NOT NULL REFERENCES data_jobs(id),
    row_number integer NOT NULL CHECK (row_number > 0),
    status text NOT NULL CHECK (status IN ('succeeded', 'failed', 'skipped')),
    object_id text,
    errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    recorded_at timestamptz NOT NULL,
    UNIQUE (data_job_id, row_number)
);

CREATE TABLE feature_flag_versions (
    id uuid PRIMARY KEY,
    flag_key text NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    enabled boolean NOT NULL,
    rules jsonb NOT NULL DEFAULT '{}'::jsonb,
    reason text NOT NULL,
    changed_by uuid NOT NULL REFERENCES users(id),
    changed_at timestamptz NOT NULL,
    UNIQUE (flag_key, version)
);

CREATE TABLE impersonation_sessions (
    id uuid PRIMARY KEY,
    actor_user_id uuid NOT NULL REFERENCES users(id),
    subject_user_id uuid NOT NULL REFERENCES users(id),
    reason text NOT NULL,
    ticket_reference text NOT NULL,
    started_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    ended_at timestamptz,
    approved_by uuid REFERENCES users(id),
    CHECK (actor_user_id <> subject_user_id),
    CHECK (expires_at > started_at)
);
CREATE INDEX impersonation_sessions_active_actor_idx
    ON impersonation_sessions (actor_user_id, expires_at) WHERE ended_at IS NULL;
