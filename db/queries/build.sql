-- name: CreateBuild :one
INSERT INTO builds (
  token,
  app_id,
  source,
  status,
  platform,
  app_name,
  bundle_id,
  version,
  build_number,
  min_os_version,
  release_notes,
  file_key,
  file_size,
  icon_key,
  metadata,
  expires_at,
  uploaded_by
) VALUES (
  sqlc.arg(token),
  sqlc.narg(app_id),
  sqlc.arg(source),
  sqlc.arg(status),
  sqlc.arg(platform),
  sqlc.arg(app_name),
  sqlc.arg(bundle_id),
  sqlc.arg(version),
  sqlc.arg(build_number),
  sqlc.narg(min_os_version),
  sqlc.narg(release_notes),
  sqlc.arg(file_key),
  sqlc.arg(file_size),
  sqlc.narg(icon_key),
  sqlc.arg(metadata),
  sqlc.narg(expires_at),
  sqlc.narg(uploaded_by)
)
RETURNING *;

-- name: GetBuildDetail :one
-- Kèm email người upload (NULL với quick share ẩn danh) và icon/chế độ public
-- của App cha (NULL với quick share).
SELECT sqlc.embed(b), u.email AS uploader_email,
  a.icon_key  AS app_icon_key,
  a.is_public AS app_is_public
FROM builds b
LEFT JOIN users u ON u.id = b.uploaded_by
LEFT JOIN apps a ON a.id = b.app_id
WHERE b.id = $1;

-- name: ListBuilds :many
-- Chỉ build của App (không tính quick share). scope_user_id NULL = owner.
-- Bộ lọc phải giống hệt CountBuilds.
--
-- sort_by:
--   uploadedAt  — thời điểm upload.
--   version     — so kiểu semver: phần số "1.10.0" tách thành mảng số nên
--                 1.9.0 < 1.10.0 < 2.0.0; cùng phần số thì bản có hậu tố
--                 (1.2.0-beta) đứng trước bản chính thức (1.2.0).
--                 Version không bắt đầu bằng số xếp cuối.
--   buildNumber — so theo số ("9" < "45").
-- Cùng giá trị sắp xếp thì bản upload sau đứng trước.
SELECT sqlc.embed(b), u.email AS uploader_email,
  a.icon_key  AS app_icon_key,
  a.is_public AS app_is_public
FROM builds b
LEFT JOIN users u ON u.id = b.uploaded_by
LEFT JOIN apps a ON a.id = b.app_id
WHERE b.source = 'app'
  AND (sqlc.narg(app_id)::uuid IS NULL OR b.app_id = sqlc.narg(app_id)::uuid)
  AND (sqlc.narg(status)::build_status IS NULL OR b.status = sqlc.narg(status)::build_status)
  AND (sqlc.narg(platform)::platform IS NULL OR b.platform = sqlc.narg(platform)::platform)
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR b.created_at >= sqlc.narg(from_time)::timestamptz)
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR b.created_at <= sqlc.narg(to_time)::timestamptz)
  AND (sqlc.narg(search)::text IS NULL
    OR position(lower(sqlc.narg(search)::text) in lower(b.version)) > 0
    OR position(lower(sqlc.narg(search)::text) in lower(b.build_number)) > 0
    OR position(lower(sqlc.narg(search)::text) in lower(coalesce(b.release_notes, ''))) > 0)
  AND (sqlc.narg(scope_user_id)::uuid IS NULL OR EXISTS (
    SELECT 1
    FROM app_managing_groups amg
    JOIN groups g ON g.id = amg.group_id
    JOIN group_members gm ON gm.group_id = g.id
    WHERE amg.app_id = b.app_id
      AND gm.user_id = sqlc.narg(scope_user_id)::uuid
      AND g.status = 'active'
  ))
ORDER BY
  CASE WHEN sqlc.arg(sort_by)::text = 'version' AND sqlc.arg(sort_asc)::bool     THEN string_to_array(substring(b.version from '^[0-9]+(?:\.[0-9]+)*'), '.')::numeric[] END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'version' AND sqlc.arg(sort_asc)::bool     THEN (b.version ~ '^[0-9]+(\.[0-9]+)*$') END ASC,
  CASE WHEN sqlc.arg(sort_by)::text = 'version' AND NOT sqlc.arg(sort_asc)::bool THEN string_to_array(substring(b.version from '^[0-9]+(?:\.[0-9]+)*'), '.')::numeric[] END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'version' AND NOT sqlc.arg(sort_asc)::bool THEN (b.version ~ '^[0-9]+(\.[0-9]+)*$') END DESC,
  CASE WHEN sqlc.arg(sort_by)::text = 'buildNumber' AND sqlc.arg(sort_asc)::bool     AND b.build_number ~ '^[0-9]+(\.[0-9]+)*$' THEN string_to_array(b.build_number, '.')::numeric[] END ASC NULLS LAST,
  CASE WHEN sqlc.arg(sort_by)::text = 'buildNumber' AND NOT sqlc.arg(sort_asc)::bool AND b.build_number ~ '^[0-9]+(\.[0-9]+)*$' THEN string_to_array(b.build_number, '.')::numeric[] END DESC NULLS LAST,
  CASE WHEN sqlc.arg(sort_asc)::bool     THEN b.created_at END ASC,
  CASE WHEN NOT sqlc.arg(sort_asc)::bool THEN b.created_at END DESC,
  b.id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountBuilds :one
SELECT count(*)
FROM builds b
WHERE b.source = 'app'
  AND (sqlc.narg(app_id)::uuid IS NULL OR b.app_id = sqlc.narg(app_id)::uuid)
  AND (sqlc.narg(status)::build_status IS NULL OR b.status = sqlc.narg(status)::build_status)
  AND (sqlc.narg(platform)::platform IS NULL OR b.platform = sqlc.narg(platform)::platform)
  AND (sqlc.narg(from_time)::timestamptz IS NULL OR b.created_at >= sqlc.narg(from_time)::timestamptz)
  AND (sqlc.narg(to_time)::timestamptz IS NULL OR b.created_at <= sqlc.narg(to_time)::timestamptz)
  AND (sqlc.narg(search)::text IS NULL
    OR position(lower(sqlc.narg(search)::text) in lower(b.version)) > 0
    OR position(lower(sqlc.narg(search)::text) in lower(b.build_number)) > 0
    OR position(lower(sqlc.narg(search)::text) in lower(coalesce(b.release_notes, ''))) > 0)
  AND (sqlc.narg(scope_user_id)::uuid IS NULL OR EXISTS (
    SELECT 1
    FROM app_managing_groups amg
    JOIN groups g ON g.id = amg.group_id
    JOIN group_members gm ON gm.group_id = g.id
    WHERE amg.app_id = b.app_id
      AND gm.user_id = sqlc.narg(scope_user_id)::uuid
      AND g.status = 'active'
  ));

-- name: UpdateBuildStatus :exec
UPDATE builds SET status = $2, updated_at = now() WHERE id = $1;

-- name: AddBuildTesterGroup :exec
INSERT INTO build_tester_groups (build_id, tester_group_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveBuildTesterGroup :exec
DELETE FROM build_tester_groups WHERE build_id = $1 AND tester_group_id = $2;

-- name: CopyDefaultTesterGroups :exec
-- Build mới kế thừa tester group mặc định của App.
INSERT INTO build_tester_groups (build_id, tester_group_id)
SELECT sqlc.arg(build_id)::bigint, tester_group_id
FROM app_default_tester_groups
WHERE app_id = sqlc.arg(app_id)
ON CONFLICT DO NOTHING;

-- name: ListBuildTesterGroups :many
SELECT btg.build_id, tg.id, tg.name
FROM build_tester_groups btg
JOIN tester_groups tg ON tg.id = btg.tester_group_id
WHERE btg.build_id = ANY(sqlc.arg(build_ids)::bigint[])
ORDER BY btg.created_at;

-- name: GetReleaseByToken :one
-- Build kèm cấu hình phân phối của App cha (NULL với quick share).
SELECT sqlc.embed(b),
  a.name           AS app_display_name,
  a.icon_key       AS app_icon_key,
  a.is_public      AS app_is_public,
  a.expires_at     AS app_expires_at,
  a.download_limit AS app_download_limit
FROM builds b
LEFT JOIN apps a ON a.id = b.app_id
WHERE b.token = $1;

-- name: RecordDownload :exec
-- Ghi một lượt tải và tăng bộ đếm trong CÙNG một câu lệnh — không cần mở
-- transaction, và hai request song song không làm lệch download_count.
WITH inserted AS (
  INSERT INTO downloads (build_id, client_ip, user_agent)
  VALUES (sqlc.arg(build_id), sqlc.arg(client_ip), sqlc.arg(user_agent))
)
UPDATE builds
SET download_count = download_count + 1
WHERE builds.id = sqlc.arg(build_id);

-- name: ListExpiredQuickShares :many
SELECT id, file_key, icon_key
FROM builds
WHERE source = 'quick_share' AND expires_at < now()
ORDER BY expires_at
LIMIT 100;

-- name: DeleteBuild :exec
DELETE FROM builds WHERE id = $1;
