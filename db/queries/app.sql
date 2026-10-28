-- name: CreateApp :one
INSERT INTO apps (name, ios_bundle_id, android_bundle_id, created_by)
VALUES (sqlc.arg(name), sqlc.narg(ios_bundle_id), sqlc.narg(android_bundle_id), sqlc.arg(created_by))
RETURNING *;

-- name: GetApp :one
SELECT * FROM apps WHERE id = $1;

-- name: IsAppInScope :one
-- Quy tắc scope: user là thành viên của ít nhất một nhóm quản trị đang active
-- của App. Owner không đi qua query này (thấy mọi App).
SELECT EXISTS (
  SELECT 1
  FROM app_managing_groups amg
  JOIN groups g ON g.id = amg.group_id
  JOIN group_members gm ON gm.group_id = g.id
  WHERE amg.app_id = sqlc.arg(app_id)
    AND gm.user_id = sqlc.arg(user_id)
    AND g.status = 'active'
)::bool;

-- name: ListApps :many
-- scope_user_id NULL = không lọc scope (owner).
SELECT sqlc.embed(a), count(*) OVER () AS total
FROM apps a
WHERE (sqlc.narg(scope_user_id)::uuid IS NULL OR EXISTS (
    SELECT 1
    FROM app_managing_groups amg
    JOIN groups g ON g.id = amg.group_id
    JOIN group_members gm ON gm.group_id = g.id
    WHERE amg.app_id = a.id
      AND gm.user_id = sqlc.narg(scope_user_id)::uuid
      AND g.status = 'active'
  ))
  AND (sqlc.narg(search)::text IS NULL OR position(lower(sqlc.narg(search)::text) in lower(a.name)) > 0)
  AND (sqlc.narg(platform)::platform IS NULL
    OR (sqlc.narg(platform)::platform = 'ios' AND a.ios_bundle_id IS NOT NULL)
    OR (sqlc.narg(platform)::platform = 'android' AND a.android_bundle_id IS NOT NULL))
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR a.created_at >= sqlc.narg(from_time)::timestamptz)
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR a.created_at <= sqlc.narg(to_time)::timestamptz)
ORDER BY a.created_at DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: UpdateAppInfo :one
UPDATE apps SET
  name              = sqlc.arg(name),
  ios_bundle_id     = sqlc.narg(ios_bundle_id),
  android_bundle_id = sqlc.narg(android_bundle_id),
  updated_at        = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateAppVisibility :one
UPDATE apps SET
  is_public      = sqlc.arg(is_public),
  expires_at     = sqlc.narg(expires_at),
  download_limit = sqlc.narg(download_limit),
  updated_at     = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateAppApproval :one
UPDATE apps SET
  requires_approval = sqlc.arg(requires_approval),
  updated_at        = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateAppAutoAssign :one
UPDATE apps SET
  auto_assign_tester_groups = sqlc.arg(auto_assign_tester_groups),
  updated_at                = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateAppIcon :exec
UPDATE apps SET icon_key = $2, updated_at = now() WHERE id = $1;

-- name: AddAppManagingGroup :exec
INSERT INTO app_managing_groups (app_id, group_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveAppManagingGroup :exec
DELETE FROM app_managing_groups WHERE app_id = $1 AND group_id = $2;

-- name: CountAppManagingGroups :one
-- Chỉ đếm nhóm chưa bị xoá: nhóm deleted không còn quản lý được App.
SELECT count(*)
FROM app_managing_groups amg
JOIN groups g ON g.id = amg.group_id
WHERE amg.app_id = $1 AND g.status <> 'deleted';

-- name: ListAppManagingGroups :many
SELECT amg.app_id, g.id, g.name
FROM app_managing_groups amg
JOIN groups g ON g.id = amg.group_id
WHERE amg.app_id = ANY(sqlc.arg(app_ids)::uuid[])
  AND g.status <> 'deleted'
ORDER BY amg.created_at;

-- name: AddAppDefaultTesterGroup :exec
INSERT INTO app_default_tester_groups (app_id, tester_group_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveAppDefaultTesterGroup :exec
DELETE FROM app_default_tester_groups WHERE app_id = $1 AND tester_group_id = $2;

-- name: ListAppDefaultTesterGroups :many
SELECT adt.app_id, tg.id, tg.name
FROM app_default_tester_groups adt
JOIN tester_groups tg ON tg.id = adt.tester_group_id
WHERE adt.app_id = ANY(sqlc.arg(app_ids)::uuid[])
ORDER BY adt.created_at;

-- name: ListPublicAppBuilds :many
-- Build trên trang App công khai: chỉ bản khách cài được ngay — ready, link
-- còn hạn, chưa hết lượt tải (download_limit của App, NULL = không giới hạn).
-- Lọc / sắp xếp / phân trang giống ListTesterAppBuilds.
SELECT b.*
FROM builds b
WHERE b.app_id = sqlc.arg(app_id)
  AND b.status = 'ready'
  AND (b.expires_at IS NULL OR b.expires_at > now())
  AND (sqlc.narg(download_limit)::int IS NULL OR b.download_count < sqlc.narg(download_limit)::int)
  AND (sqlc.narg(platform)::platform IS NULL OR b.platform = sqlc.narg(platform)::platform)
ORDER BY
  CASE WHEN sqlc.arg(sort_by)::text = 'buildNumber' AND sqlc.arg(sort_asc)::bool AND b.build_number ~ '^[0-9]+(\.[0-9]+)*$'
    THEN string_to_array(b.build_number, '.')::numeric[] END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'buildNumber' AND NOT sqlc.arg(sort_asc)::bool AND b.build_number ~ '^[0-9]+(\.[0-9]+)*$'
    THEN string_to_array(b.build_number, '.')::numeric[] END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_asc)::bool THEN b.created_at END ASC,
  CASE WHEN NOT sqlc.arg(sort_asc)::bool THEN b.created_at END DESC,
  b.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountPublicAppBuildsByPlatform :many
-- Số build trên trang App công khai theo từng platform (không áp bộ lọc platform).
SELECT b.platform, count(*) AS total
FROM builds b
WHERE b.app_id = sqlc.arg(app_id)
  AND b.status = 'ready'
  AND (b.expires_at IS NULL OR b.expires_at > now())
  AND (sqlc.narg(download_limit)::int IS NULL OR b.download_count < sqlc.narg(download_limit)::int)
GROUP BY b.platform;
