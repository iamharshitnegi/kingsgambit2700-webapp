DROP TABLE IF EXISTS staff_mfa_recovery_codes;
DROP INDEX IF EXISTS staff_mfa_methods_one_active_totp_idx;
ALTER TABLE staff_mfa_methods
    DROP CONSTRAINT IF EXISTS staff_mfa_methods_ciphertext_present,
    DROP COLUMN IF EXISTS last_totp_step;
ALTER TABLE auth_sessions
    DROP CONSTRAINT IF EXISTS auth_sessions_mfa_assurance_consistent,
    DROP COLUMN IF EXISTS mfa_method,
    DROP COLUMN IF EXISTS mfa_verified_at;
