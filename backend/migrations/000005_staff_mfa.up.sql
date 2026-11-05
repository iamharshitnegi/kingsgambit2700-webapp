ALTER TABLE auth_sessions
    ADD COLUMN mfa_verified_at timestamptz,
    ADD COLUMN mfa_method text,
    ADD CONSTRAINT auth_sessions_mfa_assurance_consistent CHECK (
        (mfa_verified_at IS NULL AND mfa_method IS NULL) OR
        (mfa_verified_at IS NOT NULL AND mfa_method = 'totp')
    );

ALTER TABLE staff_mfa_methods
    ADD COLUMN last_totp_step bigint,
    ADD CONSTRAINT staff_mfa_methods_ciphertext_present CHECK (octet_length(encrypted_secret) >= 30);

CREATE UNIQUE INDEX staff_mfa_methods_one_active_totp_idx
    ON staff_mfa_methods (user_id)
    WHERE method = 'totp' AND disabled_at IS NULL;

CREATE TABLE staff_mfa_recovery_codes (
    id uuid PRIMARY KEY,
    method_id uuid NOT NULL REFERENCES staff_mfa_methods(id),
    code_hash bytea NOT NULL CHECK (octet_length(code_hash) = 32),
    created_at timestamptz NOT NULL,
    used_at timestamptz,
    UNIQUE (method_id, code_hash)
);

CREATE INDEX staff_mfa_recovery_codes_unused_idx
    ON staff_mfa_recovery_codes (method_id, code_hash)
    WHERE used_at IS NULL;
