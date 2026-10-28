-- name: CreateUser :one
-- hashed_password rỗng = tài khoản được mời, chưa đặt mật khẩu.
INSERT INTO users (
  email,
  hashed_password,
  full_name,
  role,
  invited_by
) VALUES (
  sqlc.arg(email),
  sqlc.arg(hashed_password),
  sqlc.arg(full_name),
  sqlc.arg(role),
  sqlc.narg(invited_by)
)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
-- Email luôn lưu chữ thường (service chuẩn hoá trước khi ghi), lower() ở đây
-- chỉ là lưới an toàn cho dữ liệu nhập tay.
SELECT * FROM users WHERE lower(email) = lower(sqlc.arg(email)::text);

-- name: ListUsers :many
-- role / search NULL = không lọc. search tìm trong tên HOẶC email, không
-- phân biệt hoa thường (position thay cho LIKE để ký tự % _ trong từ khoá
-- không thành wildcard). Bộ lọc phải giống hệt CountUsers.
SELECT * FROM users
WHERE (status = 'deleted') = sqlc.arg(deleted)::bool
  AND (sqlc.narg(role)::user_role IS NULL OR role = sqlc.narg(role)::user_role)
  AND (sqlc.narg(search)::text IS NULL
    OR position(lower(sqlc.narg(search)::text) in lower(full_name)) > 0
    OR position(lower(sqlc.narg(search)::text) in lower(email)) > 0)
ORDER BY
  CASE WHEN sqlc.arg(sort_by)::text = 'name'  AND sqlc.arg(sort_asc)::bool     THEN lower(full_name) END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'name'  AND NOT sqlc.arg(sort_asc)::bool THEN lower(full_name) END DESC,
  CASE WHEN sqlc.arg(sort_by)::text = 'email' AND sqlc.arg(sort_asc)::bool     THEN lower(email) END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'email' AND NOT sqlc.arg(sort_asc)::bool THEN lower(email) END DESC,
  CASE WHEN sqlc.arg(sort_by)::text = 'role'  AND sqlc.arg(sort_asc)::bool     THEN role::text END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'role'  AND NOT sqlc.arg(sort_asc)::bool THEN role::text END DESC,
  CASE WHEN sqlc.arg(sort_asc)::bool     THEN created_at END ASC,
  CASE WHEN NOT sqlc.arg(sort_asc)::bool THEN created_at END DESC,
  id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountUsers :one
SELECT count(*) FROM users
WHERE (status = 'deleted') = sqlc.arg(deleted)::bool
  AND (sqlc.narg(role)::user_role IS NULL OR role = sqlc.narg(role)::user_role)
  AND (sqlc.narg(search)::text IS NULL
    OR position(lower(sqlc.narg(search)::text) in lower(full_name)) > 0
    OR position(lower(sqlc.narg(search)::text) in lower(email)) > 0);

-- name: UpdateUserRole :one
UPDATE users SET
  role       = sqlc.arg(role),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateUserStatus :one
UPDATE users SET
  status     = sqlc.arg(status),
  updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateUserPassword :one
UPDATE users SET
  hashed_password     = $2,
  password_changed_at = now(),
  updated_at          = now()
WHERE id = $1
RETURNING *;

-- name: ActivateInvitedUser :one
-- Người được mời đặt mật khẩu (và tên) lần đầu.
UPDATE users SET
  hashed_password     = sqlc.arg(hashed_password),
  full_name           = COALESCE(sqlc.narg(full_name), full_name),
  password_changed_at = now(),
  updated_at          = now()
WHERE id = sqlc.arg(id)
RETURNING *;
