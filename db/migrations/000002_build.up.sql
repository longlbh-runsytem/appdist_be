-- 000002_build
-- Bản upload (IPA/APK) kèm link tải, và lịch sử từng lượt tải.
-- Sinh từ db/docs/db.dbml (nguồn sự thật). Đừng sửa tay lệch khỏi DBML.

CREATE TYPE "platform" AS ENUM (
  'ios',
  'android'
);

CREATE TABLE "builds" (
  "id"             bigserial PRIMARY KEY,
  "token"          varchar(16) UNIQUE NOT NULL,
  "platform"       platform NOT NULL,
  "app_name"       varchar NOT NULL,
  "bundle_id"      varchar NOT NULL,
  "version"        varchar NOT NULL,
  "build_number"   varchar NOT NULL,
  "min_os_version" varchar,
  "release_notes"  varchar,
  "file_key"       varchar UNIQUE NOT NULL,
  "file_size"      bigint NOT NULL,
  "icon_key"       varchar,
  "metadata"       jsonb NOT NULL DEFAULT '{}',
  "download_count" int NOT NULL DEFAULT 0,
  "active"         bool NOT NULL DEFAULT true,
  "expires_at"     timestamptz,
  "uploaded_by"    uuid NOT NULL,
  "updated_at"     timestamptz,
  "created_at"     timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "fk_builds_uploaded_by"
    FOREIGN KEY ("uploaded_by") REFERENCES "users" ("id") ON DELETE RESTRICT
);

COMMENT ON COLUMN "builds"."token" IS 'chuỗi ngẫu nhiên trên link tải — không dùng id để tránh bị dò link';
COMMENT ON COLUMN "builds"."platform" IS 'suy ra từ NỘI DUNG file, không từ đuôi file';
COMMENT ON COLUMN "builds"."file_key" IS 'object key trên B2 — unique để 1 file không bị tạo 2 bản ghi';
COMMENT ON COLUMN "builds"."expires_at" IS 'hạn của LINK — null = không hết hạn';

CREATE INDEX "idx_builds_uploaded_by_created" ON "builds" ("uploaded_by", "created_at");
CREATE INDEX "idx_builds_bundle_id_created" ON "builds" ("bundle_id", "created_at");

CREATE TABLE "downloads" (
  "id"         bigserial PRIMARY KEY,
  "build_id"   bigint NOT NULL,
  "client_ip"  varchar,
  "user_agent" varchar,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "fk_downloads_build"
    FOREIGN KEY ("build_id") REFERENCES "builds" ("id") ON DELETE CASCADE
);

COMMENT ON TABLE "downloads" IS 'Một dòng = một lần tải file IPA/APK thật. Mở trang tải hoặc tải manifest.plist KHÔNG tính.';

CREATE INDEX "idx_downloads_build_created" ON "downloads" ("build_id", "created_at");
