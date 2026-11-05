CREATE TABLE auth_password_credentials (
    user_id uuid PRIMARY KEY REFERENCES users(id),
    username citext NOT NULL UNIQUE CHECK (username::text ~ '^[a-z0-9][a-z0-9_.-]{2,31}$'),
    password_hash text NOT NULL,
    updated_at timestamptz NOT NULL
);

-- Credentials become usable only after the matching mailbox verifies this request.
CREATE TABLE auth_password_registrations (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    email citext NOT NULL,
    username citext NOT NULL,
    display_name text NOT NULL,
    password_hash text NOT NULL,
    redirect_path text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz
);
CREATE INDEX auth_password_registrations_expiry_idx ON auth_password_registrations(expires_at);

CREATE TABLE auth_google_identities (
    subject text PRIMARY KEY,
    user_id uuid NOT NULL UNIQUE REFERENCES users(id),
    created_at timestamptz NOT NULL
);

CREATE TABLE auth_google_challenges (
    nonce_hash bytea PRIMARY KEY CHECK (octet_length(nonce_hash) = 32),
    expires_at timestamptz NOT NULL
);
