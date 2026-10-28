-- name: CreateAPIKey :one
INSERT INTO app_api_keys (app_id, label, token_hash, token_preview, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListAPIKeys :many
SELECT sqlc.embed(k), u.email AS created_by_email
FROM app_api_keys k
JOIN users u ON u.id = k.created_by
WHERE k.app_id = $1 AND k.revoked_at IS NULL
ORDER BY k.created_at DESC;

-- name: RevokeAPIKey :execrows
UPDATE app_api_keys SET revoked_at = now()
WHERE app_id = $1 AND id = $2 AND revoked_at IS NULL;

-- name: GetActiveAPIKeyByHash :one
SELECT * FROM app_api_keys WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: TouchAPIKey :exec
UPDATE app_api_keys SET last_used_at = now() WHERE id = $1;
