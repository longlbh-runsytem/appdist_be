-- name: StatsSummary :one
-- scope_user_id NULL = owner (không lọc). Tester group dùng chung toàn hệ
-- thống nên total_testers không lọc scope.
SELECT
  (SELECT count(*) FROM apps a
    WHERE sqlc.narg(scope_user_id)::uuid IS NULL OR EXISTS (
      SELECT 1 FROM app_managing_groups amg
      JOIN groups g ON g.id = amg.group_id
      JOIN group_members gm ON gm.group_id = g.id
      WHERE amg.app_id = a.id AND gm.user_id = sqlc.narg(scope_user_id)::uuid AND g.status = 'active'
    )) AS total_apps,
  (SELECT count(*) FROM builds b
    WHERE b.source = 'app' AND (sqlc.narg(scope_user_id)::uuid IS NULL OR EXISTS (
      SELECT 1 FROM app_managing_groups amg
      JOIN groups g ON g.id = amg.group_id
      JOIN group_members gm ON gm.group_id = g.id
      WHERE amg.app_id = b.app_id AND gm.user_id = sqlc.narg(scope_user_id)::uuid AND g.status = 'active'
    ))) AS total_builds,
  (SELECT count(*) FROM downloads d
    JOIN builds b ON b.id = d.build_id
    WHERE b.source = 'app' AND (sqlc.narg(scope_user_id)::uuid IS NULL OR EXISTS (
      SELECT 1 FROM app_managing_groups amg
      JOIN groups g ON g.id = amg.group_id
      JOIN group_members gm ON gm.group_id = g.id
      WHERE amg.app_id = b.app_id AND gm.user_id = sqlc.narg(scope_user_id)::uuid AND g.status = 'active'
    ))) AS total_downloads,
  (SELECT count(DISTINCT t.user_id) FROM testers t
    JOIN users u ON u.id = t.user_id
    WHERE t.status = 'active' AND u.status = 'active') AS total_testers;

-- name: StatsDownloadsByDay :many
-- Theo ngày giờ Việt Nam; ngày không có lượt tải vẫn có dòng với count = 0.
SELECT d.day::date AS date, count(x.id) AS count
FROM generate_series(sqlc.arg(from_date)::date, sqlc.arg(to_date)::date, interval '1 day') AS d(day)
LEFT JOIN (
  SELECT dl.id, (dl.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh')::date AS local_day
  FROM downloads dl
  JOIN builds b ON b.id = dl.build_id
  WHERE b.source = 'app'
    AND dl.created_at >= sqlc.arg(from_date)::date - interval '1 day'
    AND dl.created_at <  sqlc.arg(to_date)::date + interval '2 day'
    AND (sqlc.narg(scope_user_id)::uuid IS NULL OR EXISTS (
      SELECT 1 FROM app_managing_groups amg
      JOIN groups g ON g.id = amg.group_id
      JOIN group_members gm ON gm.group_id = g.id
      WHERE amg.app_id = b.app_id AND gm.user_id = sqlc.narg(scope_user_id)::uuid AND g.status = 'active'
    ))
) x ON x.local_day = d.day::date
GROUP BY d.day
ORDER BY d.day;
