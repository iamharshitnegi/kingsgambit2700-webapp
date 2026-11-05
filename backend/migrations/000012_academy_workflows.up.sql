ALTER TABLE batches ADD COLUMN name text NOT NULL DEFAULT 'Academy batch';
ALTER TABLE leads ADD COLUMN academy_notes text NOT NULL DEFAULT '', ADD COLUMN converted_student_id uuid REFERENCES student_profiles(id);
ALTER TABLE trial_bookings ADD COLUMN academy_notes text NOT NULL DEFAULT '';
ALTER TABLE class_sessions
    ADD COLUMN title text NOT NULL DEFAULT 'Chess lesson',
    ADD COLUMN lesson_notes text NOT NULL DEFAULT '',
    ADD COLUMN meeting_url text NOT NULL DEFAULT '',
    ADD COLUMN recurrence_rule_id uuid REFERENCES recurrence_rules(id);

CREATE TABLE academy_class_participants (
    class_session_id uuid NOT NULL REFERENCES class_sessions(id),
    student_profile_id uuid NOT NULL REFERENCES student_profiles(id),
    enrolment_id uuid NOT NULL REFERENCES enrolments(id),
    status text NOT NULL CHECK (status IN ('reserved', 'consumed', 'released')),
    cancellation_reason text NOT NULL DEFAULT '',
    PRIMARY KEY (class_session_id, student_profile_id)
);
CREATE INDEX academy_participants_student_idx ON academy_class_participants(student_profile_id, class_session_id);
CREATE TABLE academy_user_preferences (
 user_id uuid PRIMARY KEY REFERENCES users(id),
 time_zone text NOT NULL DEFAULT 'UTC',
 saved_views jsonb NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(saved_views)='array'),
 updated_at timestamptz NOT NULL
);
CREATE TABLE academy_request_results (
    actor_id uuid NOT NULL REFERENCES users(id),
    operation text NOT NULL,
    idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 200),
    request_digest bytea NOT NULL CHECK (octet_length(request_digest) = 32),
    response jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (actor_id, operation, idempotency_key)
);
CREATE TABLE academy_make_up_obligations (
    id uuid PRIMARY KEY,
    class_session_id uuid NOT NULL REFERENCES class_sessions(id),
    student_profile_id uuid NOT NULL REFERENCES student_profiles(id),
    reason text NOT NULL,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'fulfilled', 'cancelled')),
    replacement_session_id uuid REFERENCES class_sessions(id),
    created_at timestamptz NOT NULL,
    UNIQUE(class_session_id, student_profile_id)
);

INSERT INTO permission_definitions(code, description) VALUES
 ('academy.manage', 'Manage academy families, batches and enrolments'),
 ('academy.staff.manage', 'Provision academy staff accounts'),
 ('academy.policy.manage', 'Configure organization and publish consent policies'),
 ('schedule.manage', 'Manage academy class scheduling'),
 ('schedule.view', 'View academy schedules'),
 ('teaching.manage', 'Manage academy teaching and attendance');
INSERT INTO role_permission_defaults(role, permission_code) VALUES
 ('operations_admin','academy.manage'), ('operations_admin','schedule.manage'), ('operations_admin','schedule.view'),
 ('academic_manager','teaching.manage'), ('academic_manager','schedule.view'),
 ('super_admin','academy.manage'), ('super_admin','academy.staff.manage'), ('super_admin','academy.policy.manage'),
 ('super_admin','schedule.manage'), ('super_admin','schedule.view'), ('super_admin','teaching.manage');
