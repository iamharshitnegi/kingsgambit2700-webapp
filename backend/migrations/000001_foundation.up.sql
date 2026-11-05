CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id uuid PRIMARY KEY,
    email citext UNIQUE,
    display_name text NOT NULL,
    status text NOT NULL CHECK (status IN ('invited', 'pending_verification', 'active', 'suspended', 'archived')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE role_grants (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    role text NOT NULL CHECK (role IN ('guardian', 'student', 'coach', 'academic_manager', 'operations_admin', 'finance_admin', 'super_admin')),
    granted_at timestamptz NOT NULL,
    revoked_at timestamptz,
    UNIQUE (user_id, role)
);
CREATE INDEX role_grants_active_user_idx ON role_grants (user_id) WHERE revoked_at IS NULL;

CREATE TABLE auth_magic_links (
    id uuid PRIMARY KEY,
    email citext NOT NULL,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    redirect_path text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL
);
CREATE INDEX auth_magic_links_expiry_idx ON auth_magic_links (expires_at) WHERE consumed_at IS NULL;

CREATE TABLE auth_sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    csrf_hash bytea NOT NULL CHECK (octet_length(csrf_hash) = 32),
    csrf_expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    idle_expires_at timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    rotated_from_id uuid REFERENCES auth_sessions(id),
    user_agent_hash bytea NOT NULL,
    ip_address_hash bytea NOT NULL,
    CHECK (idle_expires_at <= absolute_expires_at),
    CHECK (csrf_expires_at <= absolute_expires_at)
);
CREATE INDEX auth_sessions_active_user_idx ON auth_sessions (user_id, absolute_expires_at) WHERE revoked_at IS NULL;

CREATE TABLE student_login_codes (
    id uuid PRIMARY KEY,
    student_user_id uuid NOT NULL REFERENCES users(id),
    code_hash bytea NOT NULL UNIQUE CHECK (octet_length(code_hash) = 32),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL
);

CREATE TABLE staff_mfa_methods (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    method text NOT NULL CHECK (method = 'totp'),
    encrypted_secret bytea NOT NULL,
    verified_at timestamptz,
    created_at timestamptz NOT NULL,
    disabled_at timestamptz
);

CREATE TABLE organization_settings (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    name text NOT NULL,
    legal_name text,
    email citext,
    phone text,
    whatsapp text,
    address text,
    canonical_url text NOT NULL,
    social_links jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL
);
INSERT INTO organization_settings (name, canonical_url, updated_at)
VALUES ('KingsGambit2700', 'https://kingsgambit2700.in', now());

CREATE TABLE policies (
    id uuid PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    title text NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE TABLE policy_versions (
    id uuid PRIMARY KEY,
    policy_id uuid NOT NULL REFERENCES policies(id),
    version text NOT NULL,
    locale text NOT NULL,
    purpose text NOT NULL,
    content_html text NOT NULL,
    content_hash bytea NOT NULL,
    status text NOT NULL CHECK (status IN ('draft', 'review', 'published', 'archived')),
    effective_at timestamptz NOT NULL,
    published_at timestamptz,
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL,
    UNIQUE (policy_id, version, locale)
);

CREATE TABLE consent_records (
    id uuid PRIMARY KEY,
    policy_version_id uuid NOT NULL REFERENCES policy_versions(id),
    subject_user_id uuid REFERENCES users(id),
    actor_user_id uuid NOT NULL REFERENCES users(id),
    purpose text NOT NULL,
    region text NOT NULL,
    locale text NOT NULL,
    evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
    accepted_at timestamptz NOT NULL,
    withdrawn_at timestamptz
);

CREATE TABLE programs (
    id uuid PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    name text NOT NULL,
    status text NOT NULL CHECK (status IN ('draft', 'active', 'archived')),
    display_order integer NOT NULL DEFAULT 0,
    public_json jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

INSERT INTO programs (id, slug, name, status, display_order, public_json, created_at, updated_at)
VALUES
('01900000-0000-7000-8000-000000000001', 'small-group-academy', 'Small-group academy', 'active', 10,
 '{"id":"01900000-0000-7000-8000-000000000001","slug":"small-group-academy","name":"Small-group academy","summary":"Structured live coaching in a small recurring batch.","formats":["group"],"ageBands":[],"levelSlugs":["beginner","intermediate","advanced"],"capacity":6}'::jsonb, now(), now()),
('01900000-0000-7000-8000-000000000002', 'one-to-one-coaching', 'One-to-one coaching', 'active', 20,
 '{"id":"01900000-0000-7000-8000-000000000002","slug":"one-to-one-coaching","name":"One-to-one coaching","summary":"Individual live coaching aligned to the student’s goals.","formats":["one_to_one"],"ageBands":[],"levelSlugs":["beginner","intermediate","advanced"],"capacity":1}'::jsonb, now(), now());

CREATE TABLE coach_profiles (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL UNIQUE REFERENCES users(id),
    slug text NOT NULL UNIQUE,
    status text NOT NULL,
    is_public boolean NOT NULL DEFAULT false,
    verified_at timestamptz,
    display_order integer NOT NULL DEFAULT 0,
    public_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE curriculum_versions (
    id uuid PRIMARY KEY,
    version text NOT NULL UNIQUE,
    status text NOT NULL CHECK (status IN ('draft', 'review', 'published', 'archived')),
    effective_at timestamptz NOT NULL,
    public_json jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    published_at timestamptz
);
INSERT INTO curriculum_versions (id, version, status, effective_at, public_json, created_at, published_at)
VALUES ('01900000-0000-7000-8000-000000000010', '1.0', 'published', now(),
 '{"version":"1.0","tracks":[{"slug":"academy","name":"Academy","summary":"A progressive live-coaching pathway.","levels":[{"slug":"beginner","name":"Beginner","summary":"Build reliable foundations.","outcomes":[]},{"slug":"intermediate","name":"Intermediate","summary":"Develop calculation, strategy and game review habits.","outcomes":[]},{"slug":"advanced","name":"Advanced","summary":"Prepare deliberately for competitive play.","outcomes":[]}]}]}'::jsonb,
 now(), now());

CREATE TABLE plans (
    id uuid PRIMARY KEY,
    program_id uuid NOT NULL REFERENCES programs(id),
    name text NOT NULL,
    status text NOT NULL CHECK (status IN ('draft', 'active', 'archived')),
    currency char(3) NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    display_order integer NOT NULL DEFAULT 0,
    public_json jsonb NOT NULL,
    policy_snapshot jsonb NOT NULL,
    effective_at timestamptz NOT NULL,
    expires_at timestamptz
);

CREATE TABLE leads (
    id uuid PRIMARY KEY,
    guardian_name text NOT NULL,
    normalized_email citext,
    normalized_phone text,
    student_age_band text NOT NULL,
    experience text,
    rating integer CHECK (rating BETWEEN 0 AND 4000),
    country char(2) NOT NULL,
    time_zone text NOT NULL,
    preferred_format text,
    preferred_times text[] NOT NULL DEFAULT '{}',
    contact_consent boolean NOT NULL,
    marketing_consent boolean NOT NULL DEFAULT false,
    owner_user_id uuid REFERENCES users(id),
    status text NOT NULL CHECK (status IN ('new', 'contacted', 'trial_booked', 'trial_attended', 'assessed', 'offered', 'won', 'unqualified', 'lost', 'duplicate', 'do_not_contact')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (normalized_email IS NOT NULL OR normalized_phone IS NOT NULL)
);
CREATE UNIQUE INDEX leads_email_unique_idx ON leads (normalized_email) WHERE normalized_email IS NOT NULL;
CREATE UNIQUE INDEX leads_phone_unique_idx ON leads (normalized_phone) WHERE normalized_phone IS NOT NULL;

CREATE TABLE lead_stage_events (
    id uuid PRIMARY KEY,
    lead_id uuid NOT NULL REFERENCES leads(id),
    stage text NOT NULL,
    source text NOT NULL,
    actor_user_id uuid REFERENCES users(id),
    occurred_at timestamptz NOT NULL
);
CREATE INDEX lead_stage_events_lead_idx ON lead_stage_events (lead_id, occurred_at);

CREATE TABLE campaign_touches (
    id uuid PRIMARY KEY,
    lead_id uuid NOT NULL REFERENCES leads(id),
    source text,
    campaign text,
    medium text,
    term text,
    content text,
    landing_page text,
    referrer text,
    occurred_at timestamptz NOT NULL
);

CREATE TABLE trial_slots (
    id uuid PRIMARY KEY,
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    source_time_zone text NOT NULL,
    format text NOT NULL CHECK (format IN ('group', 'one_to_one')),
    age_bands text[] NOT NULL DEFAULT '{}',
    level_slugs text[] NOT NULL DEFAULT '{}',
    capacity integer NOT NULL CHECK (capacity BETWEEN 1 AND 6),
    assessor_user_id uuid REFERENCES users(id),
    status text NOT NULL CHECK (status IN ('draft', 'published', 'cancelled')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (ends_at > starts_at)
);
CREATE INDEX trial_slots_public_idx ON trial_slots (starts_at) WHERE status = 'published';

CREATE TABLE trial_bookings (
    id uuid PRIMARY KEY,
    slot_id uuid NOT NULL REFERENCES trial_slots(id),
    lead_id uuid NOT NULL REFERENCES leads(id),
    guardian_name text NOT NULL,
    email citext NOT NULL,
    phone text,
    student_name text,
    student_age_band text NOT NULL,
    display_time_zone text NOT NULL,
    contact_consent boolean NOT NULL,
    idempotency_key text NOT NULL UNIQUE,
    status text NOT NULL CHECK (status IN ('confirmed', 'cancelled', 'rescheduled', 'attended', 'no_show')),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);
CREATE INDEX trial_bookings_slot_active_idx ON trial_bookings (slot_id) WHERE status IN ('confirmed', 'attended');

CREATE TABLE families (
    id uuid PRIMARY KEY,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    archived_at timestamptz
);

CREATE TABLE student_profiles (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL UNIQUE REFERENCES users(id),
    preferred_name text NOT NULL,
    age_band text NOT NULL,
    birth_month smallint CHECK (birth_month BETWEEN 1 AND 12),
    birth_year smallint,
    country char(2) NOT NULL,
    time_zone text NOT NULL,
    locale text NOT NULL,
    language text NOT NULL,
    status text NOT NULL CHECK (status IN ('prospect', 'trial', 'offered', 'active', 'paused', 'inactive', 'archived')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE guardian_student_relationships (
    id uuid PRIMARY KEY,
    family_id uuid NOT NULL REFERENCES families(id),
    guardian_user_id uuid NOT NULL REFERENCES users(id),
    student_profile_id uuid NOT NULL REFERENCES student_profiles(id),
    permissions text[] NOT NULL DEFAULT '{}',
    status text NOT NULL CHECK (status IN ('invited', 'active', 'revoked')),
    created_at timestamptz NOT NULL,
    revoked_at timestamptz,
    UNIQUE (guardian_user_id, student_profile_id)
);

CREATE TABLE batches (
    id uuid PRIMARY KEY,
    program_id uuid NOT NULL REFERENCES programs(id),
    level_slug text NOT NULL,
    coach_user_id uuid REFERENCES users(id),
    format text NOT NULL CHECK (format IN ('group', 'one_to_one')),
    source_time_zone text NOT NULL,
    target_capacity integer NOT NULL CHECK (target_capacity BETWEEN 1 AND 6),
    hard_capacity integer NOT NULL CHECK (hard_capacity BETWEEN 1 AND 6),
    status text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (target_capacity <= hard_capacity)
);

CREATE TABLE enrolments (
    id uuid PRIMARY KEY,
    student_profile_id uuid NOT NULL REFERENCES student_profiles(id),
    plan_id uuid REFERENCES plans(id),
    batch_id uuid REFERENCES batches(id),
    status text NOT NULL CHECK (status IN ('pending', 'active', 'paused', 'depleted', 'expired', 'cancelled')),
    policy_snapshot jsonb NOT NULL,
    starts_at timestamptz NOT NULL,
    ends_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE class_sessions (
    id uuid PRIMARY KEY,
    batch_id uuid REFERENCES batches(id),
    kind text NOT NULL CHECK (kind IN ('trial', 'regular', 'one_to_one', 'make_up')),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    source_time_zone text NOT NULL,
    source_local_start timestamp NOT NULL,
    join_opens_at timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('draft', 'scheduled', 'join_open', 'live', 'ended', 'completion_pending', 'completed', 'cancelled_academy', 'cancelled_family', 'rescheduled', 'no_show', 'technical_failure')),
    revision bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (ends_at > starts_at)
);
CREATE INDEX class_sessions_schedule_idx ON class_sessions (starts_at, status);

CREATE TABLE session_assignments (
    id uuid PRIMARY KEY,
    class_session_id uuid NOT NULL REFERENCES class_sessions(id),
    user_id uuid NOT NULL REFERENCES users(id),
    role text NOT NULL CHECK (role IN ('student', 'guardian', 'coach', 'observer', 'operations_host')),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz,
    UNIQUE (class_session_id, user_id, role)
);

CREATE TABLE attendance (
    id uuid PRIMARY KEY,
    class_session_id uuid NOT NULL REFERENCES class_sessions(id),
    student_profile_id uuid NOT NULL REFERENCES student_profiles(id),
    status text NOT NULL CHECK (status IN ('on_time', 'late', 'excused', 'unexcused', 'technical_failure')),
    resolved_by uuid NOT NULL REFERENCES users(id),
    resolved_at timestamptz NOT NULL,
    UNIQUE (class_session_id, student_profile_id)
);

CREATE TABLE entitlement_accounts (
    id uuid PRIMARY KEY,
    enrolment_id uuid NOT NULL UNIQUE REFERENCES enrolments(id),
    unit text NOT NULL CHECK (unit = 'session_credit'),
    created_at timestamptz NOT NULL
);

CREATE TABLE entitlement_entries (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES entitlement_accounts(id),
    class_session_id uuid REFERENCES class_sessions(id),
    entry_type text NOT NULL CHECK (entry_type IN ('purchase', 'grant', 'reserve', 'consume', 'reverse', 'expire', 'refund', 'adjust')),
    quantity integer NOT NULL CHECK (quantity <> 0),
    reason text NOT NULL,
    idempotency_key text NOT NULL UNIQUE,
    actor_user_id uuid REFERENCES users(id),
    occurred_at timestamptz NOT NULL
);
CREATE INDEX entitlement_entries_account_idx ON entitlement_entries (account_id, occurred_at);

CREATE TABLE progress_reports (
    id uuid PRIMARY KEY,
    student_profile_id uuid NOT NULL REFERENCES student_profiles(id),
    reporting_period_start date NOT NULL,
    reporting_period_end date NOT NULL,
    status text NOT NULL CHECK (status IN ('draft', 'review', 'published')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (student_profile_id, reporting_period_start, reporting_period_end)
);

CREATE TABLE progress_report_versions (
    id uuid PRIMARY KEY,
    progress_report_id uuid NOT NULL REFERENCES progress_reports(id),
    version integer NOT NULL CHECK (version > 0),
    content jsonb NOT NULL,
    author_user_id uuid NOT NULL REFERENCES users(id),
    published_by uuid REFERENCES users(id),
    published_at timestamptz,
    created_at timestamptz NOT NULL,
    UNIQUE (progress_report_id, version)
);

CREATE TABLE classroom_states (
    class_session_id uuid PRIMARY KEY REFERENCES class_sessions(id),
    revision bigint NOT NULL DEFAULT 0,
    fen text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('demonstration', 'free_analysis', 'student_turn', 'private_response', 'game_review', 'engine_review')),
    active_actor_user_id uuid REFERENCES users(id),
    engine_enabled boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL
);

CREATE TABLE classroom_events (
    id uuid PRIMARY KEY,
    class_session_id uuid NOT NULL REFERENCES class_sessions(id),
    revision bigint NOT NULL,
    event_type text NOT NULL,
    actor_user_id uuid REFERENCES users(id),
    payload jsonb NOT NULL,
    occurred_at timestamptz NOT NULL,
    UNIQUE (class_session_id, revision)
);
CREATE INDEX classroom_events_replay_idx ON classroom_events (class_session_id, revision);

CREATE TABLE classroom_snapshots (
    id uuid PRIMARY KEY,
    class_session_id uuid NOT NULL REFERENCES class_sessions(id),
    revision bigint NOT NULL,
    state jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (class_session_id, revision)
);

CREATE TABLE idempotency_records (
    id uuid PRIMARY KEY,
    actor_scope text NOT NULL,
    operation text NOT NULL,
    idempotency_key text NOT NULL,
    request_hash bytea NOT NULL,
    response_status integer,
    response_body jsonb,
    state text NOT NULL CHECK (state IN ('processing', 'completed', 'failed')),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (actor_scope, operation, idempotency_key)
);

CREATE TABLE webhook_receipts (
    id uuid PRIMARY KEY,
    provider text NOT NULL,
    provider_account text NOT NULL DEFAULT '',
    provider_event_id text NOT NULL,
    payload_hash bytea NOT NULL,
    received_at timestamptz NOT NULL,
    processed_at timestamptz,
    processing_error text,
    UNIQUE (provider, provider_account, provider_event_id)
);

CREATE TABLE audit_events (
    id uuid PRIMARY KEY,
    occurred_at timestamptz NOT NULL,
    actor_type text NOT NULL,
    actor_id uuid,
    impersonator_id uuid,
    action text NOT NULL,
    object_type text NOT NULL,
    object_id text NOT NULL,
    scope_id uuid,
    before_digest bytea,
    after_digest bytea,
    reason_code text,
    reason_text text,
    request_id text,
    idempotency_key text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX audit_events_object_idx ON audit_events (object_type, object_id, occurred_at);
CREATE INDEX audit_events_scope_idx ON audit_events (scope_id, occurred_at) WHERE scope_id IS NOT NULL;
