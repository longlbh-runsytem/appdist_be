-- 000009_anonymous_quick_share
-- Quick share không cần đăng nhập: builds.uploaded_by được NULL, nhưng chỉ
-- với quick share — build của App vẫn bắt buộc có người upload.
-- Sinh từ db/docs/db.dbml (nguồn sự thật). Đừng sửa tay lệch khỏi DBML.

ALTER TABLE "builds" ALTER COLUMN "uploaded_by" DROP NOT NULL;

ALTER TABLE "builds" ADD CONSTRAINT "ck_builds_uploader"
  CHECK ("source" = 'quick_share' OR "uploaded_by" IS NOT NULL);

COMMENT ON COLUMN "builds"."uploaded_by" IS 'null = quick share ẩn danh (không đăng nhập); build của App luôn có';
