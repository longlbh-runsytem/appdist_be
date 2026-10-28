DROP TABLE IF EXISTS "approval_requests";
DROP TABLE IF EXISTS "build_tester_groups";

DROP INDEX IF EXISTS "idx_downloads_created";
DROP INDEX IF EXISTS "idx_builds_source_expires";
DROP INDEX IF EXISTS "idx_builds_app_created";

-- Quick share không có chỗ trong schema cũ.
DELETE FROM "builds" WHERE "source" = 'quick_share';

ALTER TABLE "builds" ADD COLUMN "active" bool NOT NULL DEFAULT true;
UPDATE "builds" SET "active" = ("status" NOT IN ('archived', 'rejected'));

ALTER TABLE "builds"
  DROP CONSTRAINT IF EXISTS "ck_builds_source_app",
  DROP CONSTRAINT IF EXISTS "fk_builds_app",
  DROP COLUMN "app_id",
  DROP COLUMN "source",
  DROP COLUMN "status";

-- App sinh từ backfill bị xoá cùng 000007_app.down.
DROP TYPE IF EXISTS "approval_decision";
DROP TYPE IF EXISTS "build_source";
DROP TYPE IF EXISTS "build_status";
