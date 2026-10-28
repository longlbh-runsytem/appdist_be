-- Quick share ẩn danh không có chỗ trong schema cũ (uploaded_by NOT NULL).
-- File trên storage của chúng thành rác — job dọn dẹp không còn thấy bản ghi.
DELETE FROM "builds" WHERE "uploaded_by" IS NULL;

ALTER TABLE "builds" DROP CONSTRAINT IF EXISTS "ck_builds_uploader";
ALTER TABLE "builds" ALTER COLUMN "uploaded_by" SET NOT NULL;
COMMENT ON COLUMN "builds"."uploaded_by" IS NULL;
