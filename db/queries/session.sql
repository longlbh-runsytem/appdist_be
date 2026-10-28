-- name: CreateSession :one
INSERT INTO sessions (
  id,
  user_id,
  refresh_token,
  user_agent,
  client_ip,
  expires_at
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetSession :one
SELECT * FROM sessions WHERE id = $1;

-- name: BlockSession :exec
UPDATE sessions SET is_blocked = true WHERE id = $1;

-- name: BlockAllUserSessions :exec
UPDATE sessions SET is_blocked = true WHERE user_id = $1 AND is_blocked = false;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < now();
