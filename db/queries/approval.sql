-- name: CreateApprovalRequest :exec
INSERT INTO approval_requests (build_id, requested_by)
VALUES ($1, $2);

-- name: ListPendingApprovals :many
SELECT r.id, r.build_id, r.requested_at, a.name AS app_name,
  b.version, b.build_number, u.email AS requested_by_email
FROM approval_requests r
JOIN builds b ON b.id = r.build_id
JOIN apps a ON a.id = b.app_id
JOIN users u ON u.id = r.requested_by
WHERE r.decision IS NULL
  AND (sqlc.narg(scope_user_id)::uuid IS NULL OR EXISTS (
    SELECT 1
    FROM app_managing_groups amg
    JOIN groups g ON g.id = amg.group_id
    JOIN group_members gm ON gm.group_id = g.id
    WHERE amg.app_id = a.id
      AND gm.user_id = sqlc.narg(scope_user_id)::uuid
      AND g.status = 'active'
  ))
ORDER BY r.requested_at;

-- name: GetApprovalRequest :one
SELECT sqlc.embed(r), b.app_id
FROM approval_requests r
JOIN builds b ON b.id = r.build_id
WHERE r.id = $1;

-- name: ReviewApprovalRequest :one
-- decision IS NULL: hai người duyệt cùng lúc thì chỉ một người thắng.
UPDATE approval_requests SET
  decision    = sqlc.arg(decision),
  note        = sqlc.narg(note),
  reviewed_by = sqlc.arg(reviewed_by),
  reviewed_at = now()
WHERE id = sqlc.arg(id) AND decision IS NULL
RETURNING *;
