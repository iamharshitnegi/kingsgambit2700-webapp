-- name: GetSessionIdentity :one
SELECT s.id, s.user_id, s.csrf_hash, s.csrf_expires_at, s.last_seen_at, s.idle_expires_at,
       s.absolute_expires_at, s.revoked_at,
       u.email::text AS email, u.display_name, u.status
FROM auth_sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1;

-- name: TouchActiveSession :execrows
UPDATE auth_sessions
SET last_seen_at = $2,
    idle_expires_at = LEAST($3, absolute_expires_at)
WHERE id = $1 AND revoked_at IS NULL;

-- name: ListActiveRoles :many
SELECT role
FROM role_grants
WHERE user_id = $1 AND revoked_at IS NULL
ORDER BY role;
