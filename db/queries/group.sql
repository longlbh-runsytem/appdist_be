-- name: CreateGroup :one
INSERT INTO groups (name, parent_id, created_by)
VALUES (sqlc.arg(name), sqlc.narg(parent_id), sqlc.arg(created_by))
RETURNING *;

-- name: GetGroup :one
SELECT * FROM groups WHERE id = $1;

-- name: ListGroups :many
SELECT * FROM groups
WHERE (status = 'deleted') = sqlc.arg(deleted)::bool
ORDER BY created_at DESC;

-- name: UpdateGroupStatus :one
UPDATE groups SET
  status     = sqlc.arg(status),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: AddGroupMember :exec
INSERT INTO group_members (group_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveGroupMember :exec
DELETE FROM group_members WHERE group_id = $1 AND user_id = $2;

-- name: ListGroupMembers :many
-- Thành viên của nhiều nhóm trong một lần gọi — tránh N+1 khi list nhóm.
SELECT gm.group_id, u.id, u.full_name, u.email, u.role
FROM group_members gm
JOIN users u ON u.id = gm.user_id
WHERE gm.group_id = ANY(sqlc.arg(group_ids)::uuid[])
  AND u.status <> 'deleted'
ORDER BY gm.added_at;

-- name: RemoveUserFromAllGroups :exec
-- Dùng khi hạ tài khoản xuống tester: tester không được quản lý App.
DELETE FROM group_members WHERE user_id = $1;

-- name: IsGroupMember :one
SELECT EXISTS (
  SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2
)::bool;

-- name: ListUserGroups :many
-- Các nhóm (chưa xoá) mà user là thành viên — 1 user thuộc được nhiều nhóm.
SELECT g.id, g.name, g.status
FROM group_members gm
JOIN groups g ON g.id = gm.group_id
WHERE gm.user_id = $1
  AND g.status <> 'deleted'
ORDER BY g.name;
