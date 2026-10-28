-- name: CreateTesterGroup :one
INSERT INTO tester_groups (name, created_by)
VALUES ($1, $2)
RETURNING *;

-- name: GetTesterGroup :one
SELECT sqlc.embed(tg),
  (SELECT count(*) FROM testers t WHERE t.tester_group_id = tg.id AND t.status = 'active')::int AS member_count
FROM tester_groups tg
WHERE tg.id = $1;

-- name: ListTesterGroups :many
SELECT sqlc.embed(tg),
  (SELECT count(*) FROM testers t WHERE t.tester_group_id = tg.id AND t.status = 'active')::int AS member_count
FROM tester_groups tg
ORDER BY tg.created_at DESC;

-- name: UpsertTester :one
-- Thêm lại email đã có thì trả bản ghi cũ và kích hoạt lại.
INSERT INTO testers (tester_group_id, user_id)
VALUES ($1, $2)
ON CONFLICT (tester_group_id, user_id) DO UPDATE SET
  status     = 'active',
  updated_at = now()
RETURNING *;

-- name: ListTesters :many
SELECT t.id, t.user_id, t.status, u.email
FROM testers t
JOIN users u ON u.id = t.user_id
WHERE t.tester_group_id = sqlc.arg(tester_group_id)
  AND (t.status = 'deleted') = sqlc.arg(deleted)::bool
ORDER BY t.created_at;

-- name: GetTester :one
SELECT t.id, t.user_id, t.status, u.email
FROM testers t
JOIN users u ON u.id = t.user_id
WHERE t.tester_group_id = $1 AND t.id = $2;

-- name: UpdateTesterStatus :exec
UPDATE testers SET
  status     = sqlc.arg(status),
  updated_at = now()
WHERE tester_group_id = sqlc.arg(tester_group_id) AND id = sqlc.arg(id);

-- name: ListTesterApps :many
-- App có ít nhất một build ready được gán cho tester group chứa user này.
SELECT sqlc.embed(a), count(*) OVER () AS total
FROM apps a
WHERE EXISTS (
    SELECT 1
    FROM builds b
    JOIN build_tester_groups btg ON btg.build_id = b.id
    JOIN testers t ON t.tester_group_id = btg.tester_group_id
    WHERE b.app_id = a.id
      AND b.status = 'ready'
      AND t.user_id = sqlc.arg(user_id)
      AND t.status = 'active'
  )
  AND (sqlc.narg(search)::text IS NULL OR position(lower(sqlc.narg(search)::text) in lower(a.name)) > 0)
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR a.created_at >= sqlc.narg(from_time)::timestamptz)
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR a.created_at <= sqlc.narg(to_time)::timestamptz)
ORDER BY a.created_at DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: ListTesterAppBuilds :many
-- Build ready của App mà tester được mời xem, lọc platform, phân trang.
--
-- sort_by = 'buildNumber': build_number là chuỗi ("45", "1.2.3") nên tách
-- theo dấu chấm thành mảng số để so đúng thứ tự số ("9" < "45"); chuỗi không
-- phải dạng số (hiếm) xếp cuối. Cùng build number thì mới hơn trước.
SELECT b.*
FROM builds b
WHERE b.app_id = sqlc.arg(app_id)
  AND b.status = 'ready'
  AND (sqlc.narg(platform)::platform IS NULL OR b.platform = sqlc.narg(platform)::platform)
  AND EXISTS (
    SELECT 1
    FROM build_tester_groups btg
    JOIN testers t ON t.tester_group_id = btg.tester_group_id
    WHERE btg.build_id = b.id
      AND t.user_id = sqlc.arg(user_id)
      AND t.status = 'active'
  )
ORDER BY
  CASE WHEN sqlc.arg(sort_by)::text = 'buildNumber' AND sqlc.arg(sort_asc)::bool AND b.build_number ~ '^[0-9]+(\.[0-9]+)*$'
    THEN string_to_array(b.build_number, '.')::numeric[] END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'buildNumber' AND NOT sqlc.arg(sort_asc)::bool AND b.build_number ~ '^[0-9]+(\.[0-9]+)*$'
    THEN string_to_array(b.build_number, '.')::numeric[] END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_asc)::bool THEN b.created_at END ASC,
  CASE WHEN NOT sqlc.arg(sort_asc)::bool THEN b.created_at END DESC,
  b.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountTesterAppBuildsByPlatform :many
-- Số build tester thấy được theo từng platform — không áp bộ lọc platform,
-- để FE hiện số trên mọi tab.
SELECT b.platform, count(*) AS total
FROM builds b
WHERE b.app_id = sqlc.arg(app_id)
  AND b.status = 'ready'
  AND EXISTS (
    SELECT 1
    FROM build_tester_groups btg
    JOIN testers t ON t.tester_group_id = btg.tester_group_id
    WHERE btg.build_id = b.id
      AND t.user_id = sqlc.arg(user_id)
      AND t.status = 'active'
  )
GROUP BY b.platform;

-- name: CanTesterAccessBuild :one
SELECT EXISTS (
  SELECT 1
  FROM build_tester_groups btg
  JOIN testers t ON t.tester_group_id = btg.tester_group_id
  WHERE btg.build_id = sqlc.arg(build_id)
    AND t.user_id = sqlc.arg(user_id)
    AND t.status = 'active'
)::bool;

-- name: ListTesterGroupRefs :many
SELECT id, name FROM tester_groups WHERE id = ANY(sqlc.arg(ids)::uuid[]);
