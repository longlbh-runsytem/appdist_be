-- 000008_build_v2
-- Build thuộc về App, có trạng thái, gán tester group, và yêu cầu duyệt.
-- Sinh từ db/docs/db.dbml (nguồn sự thật). Đừng sửa tay lệch khỏi DBML.

CREATE TYPE "build_status" AS ENUM (
  'processing',
  'ready',
  'pending_approval',
  'rejected',
  'archived'
);

CREATE TYPE "build_source" AS ENUM (
  'app',
  'quick_share'
);

CREATE TYPE "approval_decision" AS ENUM (
  'approve',
  'reject'
);

ALTER TABLE "builds"
  ADD COLUMN "app_id" uuid,
  ADD COLUMN "source" build_source NOT NULL DEFAULT 'app',
  ADD COLUMN "status" build_status NOT NULL DEFAULT 'ready',
  ADD CONSTRAINT "fk_builds_app"
    FOREIGN KEY ("app_id") REFERENCES "apps" ("id") ON DELETE RESTRICT;

UPDATE "builds" SET "status" = 'archived' WHERE NOT "active";
ALTER TABLE "builds" DROP COLUMN "active";

-- Build cũ chưa có App: gom theo bundle id thành App. Để public như trước
-- (link cũ vẫn chạy); chưa gán nhóm quản trị nên chỉ owner thấy.
WITH latest AS (
  SELECT DISTINCT ON ("bundle_id") "bundle_id", "app_name", "icon_key", "uploaded_by"
  FROM "builds"
  ORDER BY "bundle_id", "created_at" DESC
), plats AS (
  SELECT "bundle_id",
         bool_or("platform" = 'ios')     AS has_ios,
         bool_or("platform" = 'android') AS has_android,
         min("created_at")               AS first_at
  FROM "builds"
  GROUP BY "bundle_id"
)
INSERT INTO "apps" ("name", "ios_bundle_id", "android_bundle_id", "icon_key", "is_public", "created_by", "created_at")
SELECT l."app_name",
       CASE WHEN p.has_ios     THEN l."bundle_id" END,
       CASE WHEN p.has_android THEN l."bundle_id" END,
       l."icon_key", true, l."uploaded_by", p.first_at
FROM latest l
JOIN plats p USING ("bundle_id");

UPDATE "builds" b SET "app_id" = a."id"
FROM "apps" a
WHERE (b."platform" = 'ios'     AND a."ios_bundle_id"     = b."bundle_id")
   OR (b."platform" = 'android' AND a."android_bundle_id" = b."bundle_id");

ALTER TABLE "builds" ADD CONSTRAINT "ck_builds_source_app"
  CHECK (("source" = 'app') = ("app_id" IS NOT NULL));

CREATE INDEX "idx_builds_app_created" ON "builds" ("app_id", "created_at");
CREATE INDEX "idx_builds_source_expires" ON "builds" ("source", "expires_at");
CREATE INDEX "idx_downloads_created" ON "downloads" ("created_at");

CREATE TABLE "build_tester_groups" (
  "build_id"        bigint NOT NULL,
  "tester_group_id" uuid NOT NULL,
  "created_at"      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("build_id", "tester_group_id"),
  CONSTRAINT "fk_build_tester_groups_build"
    FOREIGN KEY ("build_id") REFERENCES "builds" ("id") ON DELETE CASCADE,
  CONSTRAINT "fk_build_tester_groups_tester_group"
    FOREIGN KEY ("tester_group_id") REFERENCES "tester_groups" ("id") ON DELETE CASCADE
);

CREATE INDEX "idx_build_tester_groups_tester_group" ON "build_tester_groups" ("tester_group_id");

CREATE TABLE "approval_requests" (
  "id"           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  "build_id"     bigint UNIQUE NOT NULL,
  "requested_by" uuid NOT NULL,
  "requested_at" timestamptz NOT NULL DEFAULT now(),
  "decision"     approval_decision,
  "note"         varchar,
  "reviewed_by"  uuid,
  "reviewed_at"  timestamptz,
  CONSTRAINT "fk_approval_requests_build"
    FOREIGN KEY ("build_id") REFERENCES "builds" ("id") ON DELETE CASCADE,
  CONSTRAINT "fk_approval_requests_requested_by"
    FOREIGN KEY ("requested_by") REFERENCES "users" ("id") ON DELETE RESTRICT,
  CONSTRAINT "fk_approval_requests_reviewed_by"
    FOREIGN KEY ("reviewed_by") REFERENCES "users" ("id") ON DELETE SET NULL
);

COMMENT ON COLUMN "approval_requests"."decision" IS 'null = đang chờ duyệt';

CREATE INDEX "idx_approval_requests_decision_requested" ON "approval_requests" ("decision", "requested_at");
