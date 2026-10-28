-- name: CreateInvitation :one
INSERT INTO invitations (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetInvitationByTokenHash :one
SELECT * FROM invitations WHERE token_hash = $1;

-- name: CloseInvitations :exec
-- Đóng mọi lời mời còn treo của user — dùng khi nhận lời mời hoặc cấp link
-- mới, để link cũ không dùng lại được.
UPDATE invitations SET accepted_at = now()
WHERE user_id = $1 AND accepted_at IS NULL;
