CREATE TABLE auth_devices (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    revoked_at timestamptz,
    mfa_verified_at timestamptz,
    mfa_method text,
    CHECK ((mfa_verified_at IS NULL AND mfa_method IS NULL) OR
           (mfa_verified_at IS NOT NULL AND mfa_method = 'totp'))
);
CREATE INDEX auth_devices_active_user_idx ON auth_devices(user_id) WHERE revoked_at IS NULL;
ALTER TABLE auth_sessions ADD COLUMN device_id uuid REFERENCES auth_devices(id);
CREATE INDEX auth_sessions_device_idx ON auth_sessions(device_id) WHERE device_id IS NOT NULL;
