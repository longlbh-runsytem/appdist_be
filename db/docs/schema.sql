-- SQL dump generated using DBML (dbml.dbdiagram.io)
-- Database: PostgreSQL
-- Generated at: 2026-10-09T02:22:03.755Z

CREATE TYPE "user_role" AS ENUM (
  'admin',
  'member',
  'owner',
  'tester'
);

CREATE TYPE "entity_status" AS ENUM (
  'active',
  'disabled',
  'deleted'
);

CREATE TYPE "platform" AS ENUM (
  'ios',
  'android'
);

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

CREATE TABLE "users" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "email" varchar UNIQUE NOT NULL,
  "hashed_password" varchar NOT NULL,
  "full_name" varchar NOT NULL,
  "role" user_role NOT NULL DEFAULT 'member',
  "status" entity_status NOT NULL DEFAULT 'active',
  "invited_by" uuid,
  "password_changed_at" timestamptz NOT NULL DEFAULT '0001-01-01 00:00:00Z',
  "updated_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "sessions" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "user_id" uuid NOT NULL,
  "refresh_token" varchar NOT NULL,
  "user_agent" varchar NOT NULL,
  "client_ip" varchar NOT NULL,
  "is_blocked" boolean NOT NULL DEFAULT false,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "invitations" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "user_id" uuid NOT NULL,
  "token_hash" varchar UNIQUE NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "accepted_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "groups" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "name" varchar NOT NULL,
  "parent_id" uuid,
  "status" entity_status NOT NULL DEFAULT 'active',
  "created_by" uuid NOT NULL,
  "updated_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "group_members" (
  "group_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "added_at" timestamptz NOT NULL DEFAULT (now()),
  PRIMARY KEY ("group_id", "user_id")
);

CREATE TABLE "tester_groups" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "name" varchar NOT NULL,
  "created_by" uuid NOT NULL,
  "updated_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "testers" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "tester_group_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "status" entity_status NOT NULL DEFAULT 'active',
  "updated_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "apps" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "name" varchar NOT NULL,
  "ios_bundle_id" varchar,
  "android_bundle_id" varchar,
  "icon_key" varchar,
  "is_public" bool NOT NULL DEFAULT false,
  "expires_at" timestamptz,
  "download_limit" int,
  "requires_approval" bool NOT NULL DEFAULT false,
  "auto_assign_tester_groups" bool NOT NULL DEFAULT false,
  "created_by" uuid NOT NULL,
  "updated_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "app_managing_groups" (
  "app_id" uuid NOT NULL,
  "group_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  PRIMARY KEY ("app_id", "group_id")
);

CREATE TABLE "app_default_tester_groups" (
  "app_id" uuid NOT NULL,
  "tester_group_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  PRIMARY KEY ("app_id", "tester_group_id")
);

CREATE TABLE "app_api_keys" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "app_id" uuid NOT NULL,
  "label" varchar NOT NULL,
  "token_hash" varchar UNIQUE NOT NULL,
  "token_preview" varchar NOT NULL,
  "created_by" uuid NOT NULL,
  "last_used_at" timestamptz,
  "revoked_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "builds" (
  "id" bigserial PRIMARY KEY,
  "token" varchar(16) UNIQUE NOT NULL,
  "app_id" uuid,
  "source" build_source NOT NULL DEFAULT 'app',
  "status" build_status NOT NULL DEFAULT 'ready',
  "platform" platform NOT NULL,
  "app_name" varchar NOT NULL,
  "bundle_id" varchar NOT NULL,
  "version" varchar NOT NULL,
  "build_number" varchar NOT NULL,
  "min_os_version" varchar,
  "release_notes" varchar,
  "file_key" varchar UNIQUE NOT NULL,
  "file_size" bigint NOT NULL,
  "icon_key" varchar,
  "metadata" jsonb NOT NULL DEFAULT '{}',
  "download_count" int NOT NULL DEFAULT 0,
  "expires_at" timestamptz,
  "uploaded_by" uuid,
  "updated_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "build_tester_groups" (
  "build_id" bigint NOT NULL,
  "tester_group_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  PRIMARY KEY ("build_id", "tester_group_id")
);

CREATE TABLE "approval_requests" (
  "id" uuid PRIMARY KEY DEFAULT (gen_random_uuid()),
  "build_id" bigint UNIQUE NOT NULL,
  "requested_by" uuid NOT NULL,
  "requested_at" timestamptz NOT NULL DEFAULT (now()),
  "decision" approval_decision,
  "note" varchar,
  "reviewed_by" uuid,
  "reviewed_at" timestamptz
);

CREATE TABLE "downloads" (
  "id" bigserial PRIMARY KEY,
  "build_id" bigint NOT NULL,
  "client_ip" varchar,
  "user_agent" varchar,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX ON "sessions" ("user_id");

CREATE INDEX ON "invitations" ("user_id");

CREATE INDEX ON "group_members" ("user_id");

CREATE UNIQUE INDEX ON "testers" ("tester_group_id", "user_id");

CREATE INDEX ON "testers" ("user_id");

CREATE INDEX ON "app_managing_groups" ("group_id");

CREATE INDEX ON "app_api_keys" ("app_id", "created_at");

CREATE INDEX ON "builds" ("uploaded_by", "created_at");

CREATE INDEX ON "builds" ("bundle_id", "created_at");

CREATE INDEX ON "builds" ("app_id", "created_at");

CREATE INDEX ON "builds" ("source", "expires_at");

CREATE INDEX ON "build_tester_groups" ("tester_group_id");

CREATE INDEX ON "approval_requests" ("decision", "requested_at");

CREATE INDEX ON "downloads" ("build_id", "created_at");

CREATE INDEX ON "downloads" ("created_at");

COMMENT ON COLUMN "users"."hashed_password" IS 'rỗng = chưa nhận lời mời, không đăng nhập được';

COMMENT ON COLUMN "users"."created_at" IS 'cũng là thời điểm được mời (invitedAt)';

COMMENT ON TABLE "invitations" IS 'Chưa có hạ tầng gửi email: API trả link mời để người mời tự gửi.';

COMMENT ON COLUMN "invitations"."token_hash" IS 'sha256 của token trên link mời — không lưu plaintext';

COMMENT ON COLUMN "groups"."parent_id" IS 'chỉ để hiển thị cây — KHÔNG kế thừa quyền';

COMMENT ON COLUMN "groups"."status" IS 'disabled = thành viên mất quyền trên App của nhóm';

COMMENT ON COLUMN "testers"."user_id" IS 'tài khoản đăng nhập của tester (role tester, hoặc member nội bộ có sẵn)';

COMMENT ON TABLE "apps" IS 'check: ít nhất một trong ios_bundle_id / android_bundle_id khác null';

COMMENT ON COLUMN "apps"."ios_bundle_id" IS 'unique khi khác null';

COMMENT ON COLUMN "apps"."android_bundle_id" IS 'unique khi khác null';

COMMENT ON COLUMN "apps"."icon_key" IS 'icon của build mới nhất';

COMMENT ON COLUMN "apps"."is_public" IS 'false = link tải chỉ mở được qua ticket do API cấp cho người có quyền';

COMMENT ON COLUMN "apps"."expires_at" IS 'sau thời điểm này mọi link tải của App ngừng hoạt động';

COMMENT ON COLUMN "apps"."download_limit" IS 'số lượt tải tối đa của MỖI build';

COMMENT ON COLUMN "app_api_keys"."token_hash" IS 'sha256 của key — plaintext chỉ trả về đúng 1 lần lúc tạo';

COMMENT ON COLUMN "app_api_keys"."token_preview" IS '6 ký tự cuối, để nhận diện';

COMMENT ON TABLE "builds" IS 'check: (source = app) = (app_id is not null)
check: source = quick_share OR uploaded_by is not null
';

COMMENT ON COLUMN "builds"."token" IS 'chuỗi ngẫu nhiên trên link tải, vd /d/Ab3xK9pQ — không dùng id để tránh bị dò link';

COMMENT ON COLUMN "builds"."app_id" IS 'null khi source = quick_share';

COMMENT ON COLUMN "builds"."platform" IS 'suy ra từ NỘI DUNG file (có Payload/*.app hay AndroidManifest.xml), không từ đuôi file';

COMMENT ON COLUMN "builds"."app_name" IS 'ảnh chụp từ file lúc upload. iOS: CFBundleDisplayName → CFBundleName. Android: android:label';

COMMENT ON COLUMN "builds"."bundle_id" IS 'iOS: CFBundleIdentifier. Android: package — phải khớp bundle id của App';

COMMENT ON COLUMN "builds"."version" IS 'iOS: CFBundleShortVersionString. Android: versionName';

COMMENT ON COLUMN "builds"."build_number" IS 'iOS: CFBundleVersion. Android: versionCode';

COMMENT ON COLUMN "builds"."min_os_version" IS 'iOS: MinimumOSVersion (vd 15.0). Android: minSdkVersion (vd API 24)';

COMMENT ON COLUMN "builds"."release_notes" IS 'changelog người upload nhập, hiện trên trang tải';

COMMENT ON COLUMN "builds"."file_key" IS 'object key trên B2 — unique để 1 file không bị tạo 2 bản ghi';

COMMENT ON COLUMN "builds"."file_size" IS 'byte';

COMMENT ON COLUMN "builds"."icon_key" IS 'icon trích từ file, đã chuyển về PNG chuẩn — null nếu trích lỗi';

COMMENT ON COLUMN "builds"."metadata" IS 'thông tin phụ, vd iOS: team_name, distribution (in-house/ad-hoc...), profile_expires_at,
    device_family. Android: target_sdk';

COMMENT ON COLUMN "builds"."download_count" IS 'tăng khi file thực sự được tải; chi tiết từng lượt ở bảng downloads';

COMMENT ON COLUMN "builds"."expires_at" IS 'hạn của LINK — null = không hết hạn';

COMMENT ON COLUMN "builds"."uploaded_by" IS 'null = quick share ẩn danh (không đăng nhập); build của App luôn có';

COMMENT ON COLUMN "approval_requests"."decision" IS 'null = đang chờ duyệt';

COMMENT ON TABLE "downloads" IS 'Một dòng = một lần tải file IPA/APK thật (iOS gọi URL trong manifest.plist = máy đang cài).
Mở trang tải hoặc tải manifest.plist KHÔNG tính.
';

COMMENT ON COLUMN "downloads"."user_agent" IS 'đọc ra model máy / phiên bản OS để thống kê';

ALTER TABLE "users" ADD FOREIGN KEY ("invited_by") REFERENCES "users" ("id") ON DELETE SET NULL DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "sessions" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "invitations" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "groups" ADD FOREIGN KEY ("parent_id") REFERENCES "groups" ("id") ON DELETE SET NULL DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "groups" ADD FOREIGN KEY ("created_by") REFERENCES "users" ("id") ON DELETE RESTRICT DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "group_members" ADD FOREIGN KEY ("group_id") REFERENCES "groups" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "group_members" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "tester_groups" ADD FOREIGN KEY ("created_by") REFERENCES "users" ("id") ON DELETE RESTRICT DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "testers" ADD FOREIGN KEY ("tester_group_id") REFERENCES "tester_groups" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "testers" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "apps" ADD FOREIGN KEY ("created_by") REFERENCES "users" ("id") ON DELETE RESTRICT DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "app_managing_groups" ADD FOREIGN KEY ("app_id") REFERENCES "apps" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "app_managing_groups" ADD FOREIGN KEY ("group_id") REFERENCES "groups" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "app_default_tester_groups" ADD FOREIGN KEY ("app_id") REFERENCES "apps" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "app_default_tester_groups" ADD FOREIGN KEY ("tester_group_id") REFERENCES "tester_groups" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "app_api_keys" ADD FOREIGN KEY ("app_id") REFERENCES "apps" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "app_api_keys" ADD FOREIGN KEY ("created_by") REFERENCES "users" ("id") ON DELETE RESTRICT DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "builds" ADD FOREIGN KEY ("uploaded_by") REFERENCES "users" ("id") ON DELETE RESTRICT DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "builds" ADD FOREIGN KEY ("app_id") REFERENCES "apps" ("id") ON DELETE RESTRICT DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "build_tester_groups" ADD FOREIGN KEY ("build_id") REFERENCES "builds" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "build_tester_groups" ADD FOREIGN KEY ("tester_group_id") REFERENCES "tester_groups" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "approval_requests" ADD FOREIGN KEY ("build_id") REFERENCES "builds" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "approval_requests" ADD FOREIGN KEY ("requested_by") REFERENCES "users" ("id") ON DELETE RESTRICT DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "approval_requests" ADD FOREIGN KEY ("reviewed_by") REFERENCES "users" ("id") ON DELETE SET NULL DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "downloads" ADD FOREIGN KEY ("build_id") REFERENCES "builds" ("id") ON DELETE CASCADE DEFERRABLE INITIALLY IMMEDIATE;
