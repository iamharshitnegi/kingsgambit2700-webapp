-- name: GetOrganization :one
SELECT name, COALESCE(legal_name, '') AS legal_name,
       COALESCE(email::text, '') AS email, COALESCE(phone, '') AS phone,
       COALESCE(whatsapp, '') AS whatsapp, COALESCE(address, '') AS address,
       canonical_url, social_links, updated_at
FROM organization_settings
WHERE singleton = true;

-- name: ListActiveProgramJSON :many
SELECT public_json
FROM programs
WHERE status = 'active'
ORDER BY display_order, slug;

-- name: ListPublishedPolicyJSON :many
SELECT jsonb_build_object(
    'slug', p.slug,
    'title', p.title,
    'version', v.version,
    'effectiveAt', v.effective_at,
    'purpose', v.purpose
)
FROM policies p
JOIN policy_versions v ON v.policy_id = p.id
WHERE v.status = 'published' AND v.effective_at <= now()
ORDER BY p.slug, v.effective_at DESC;
