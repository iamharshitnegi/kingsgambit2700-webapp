-- Independent staff permissions complement relationship-scoped guardian access.
-- Explicit deny grants take precedence over role defaults and allow grants.

CREATE TABLE permission_definitions (
    code text PRIMARY KEY,
    description text NOT NULL,
    CHECK (code ~ '^[a-z][a-z0-9_.]*$')
);

CREATE TABLE role_permission_defaults (
    role text NOT NULL CHECK (role IN (
        'guardian', 'student', 'coach', 'academic_manager',
        'operations_admin', 'finance_admin', 'super_admin'
    )),
    permission_code text NOT NULL REFERENCES permission_definitions(code),
    PRIMARY KEY (role, permission_code)
);

CREATE TABLE permission_grants (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    permission_code text NOT NULL REFERENCES permission_definitions(code),
    scope_type text NOT NULL CHECK (scope_type IN ('global', 'family', 'student')),
    scope_id uuid,
    effect text NOT NULL CHECK (effect IN ('allow', 'deny')),
    reason text NOT NULL,
    granted_by uuid NOT NULL REFERENCES users(id),
    granted_at timestamptz NOT NULL,
    expires_at timestamptz,
    revoked_at timestamptz,
    revoked_by uuid REFERENCES users(id),
    revoked_reason text,
    CHECK (
        (scope_type = 'global' AND scope_id IS NULL) OR
        (scope_type <> 'global' AND scope_id IS NOT NULL)
    ),
    CHECK (expires_at IS NULL OR expires_at > granted_at),
    CHECK (
        (revoked_at IS NULL AND revoked_by IS NULL AND revoked_reason IS NULL) OR
        (revoked_at IS NOT NULL AND revoked_by IS NOT NULL AND revoked_reason IS NOT NULL)
    )
);

CREATE INDEX permission_grants_active_lookup_idx
    ON permission_grants (user_id, permission_code, scope_type, scope_id)
    WHERE revoked_at IS NULL;

INSERT INTO permission_definitions (code, description)
VALUES
    ('family.guardian_relationship.manage', 'Grant, scope and revoke guardians within a family'),
    ('report.draft.manage', 'Create and edit monthly progress-report drafts'),
    ('report.publish', 'Approve and publish monthly progress reports'),
    ('report.view', 'View monthly progress reports outside family or student scope');

INSERT INTO role_permission_defaults (role, permission_code)
VALUES
    ('operations_admin', 'family.guardian_relationship.manage'),
    ('academic_manager', 'report.draft.manage'),
    ('academic_manager', 'report.publish'),
    ('academic_manager', 'report.view'),
    ('super_admin', 'family.guardian_relationship.manage'),
    ('super_admin', 'report.draft.manage'),
    ('super_admin', 'report.publish'),
    ('super_admin', 'report.view');
