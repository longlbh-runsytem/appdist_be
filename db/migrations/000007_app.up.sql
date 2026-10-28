-- 000007_app
-- App gom các build cùng bundle id theo platform, kèm nhóm quản trị,
-- tester group mặc định và API key cho CI.
-- Sinh từ db/docs/db.dbml (nguồn sự thật). Đừng sửa tay lệch khỏi DBML.

CREATE TABLE "apps" (
  "id"                        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  "name"                      varchar NOT NULL,
  "ios_bundle_id"             varchar,
  "android_bundle_id"         varchar,
  "icon_key"                  varchar,
  "is_public"                 bool NOT NULL DEFAULT false,
  "expires_at"                timestamptz,
  "download_limit"            int,
  "requires_approval"         bool NOT NULL DEFAULT false,
  "auto_assign_tester_groups" bool NOT NULL DEFAULT false,
  "created_by"                uuid NOT NULL,
  "updated_at"                timestamptz,
  "created_at"                timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "ck_apps_has_platform"
    CHECK ("ios_bundle_id" IS NOT NULL OR "android_bundle_id" IS NOT NULL),
  CONSTRAINT "ck_apps_download_limit"
    CHECK ("download_limit" IS NULL OR "download_limit" > 0),
  CONSTRAINT "fk_apps_created_by"
    FOREIGN KEY ("created_by") REFERENCES "users" ("id") ON DELETE RESTRICT
);

COMMENT ON COLUMN "apps"."is_public" IS 'false = link tải chỉ mở được qua ticket do API cấp cho người có quyền';
COMMENT ON COLUMN "apps"."download_limit" IS 'số lượt tải tối đa của MỖI build';

CREATE UNIQUE INDEX "uq_apps_ios_bundle_id" ON "apps" ("ios_bundle_id") WHERE "ios_bundle_id" IS NOT NULL;
CREATE UNIQUE INDEX "uq_apps_android_bundle_id" ON "apps" ("android_bundle_id") WHERE "android_bundle_id" IS NOT NULL;
CREATE INDEX "idx_apps_created_at" ON "apps" ("created_at");

CREATE TABLE "app_managing_groups" (
  "app_id"     uuid NOT NULL,
  "group_id"   uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("app_id", "group_id"),
  CONSTRAINT "fk_app_managing_groups_app"
    FOREIGN KEY ("app_id") REFERENCES "apps" ("id") ON DELETE CASCADE,
  CONSTRAINT "fk_app_managing_groups_group"
    FOREIGN KEY ("group_id") REFERENCES "groups" ("id") ON DELETE CASCADE
);

CREATE INDEX "idx_app_managing_groups_group_id" ON "app_managing_groups" ("group_id");

CREATE TABLE "app_default_tester_groups" (
  "app_id"          uuid NOT NULL,
  "tester_group_id" uuid NOT NULL,
  "created_at"      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("app_id", "tester_group_id"),
  CONSTRAINT "fk_app_default_tester_groups_app"
    FOREIGN KEY ("app_id") REFERENCES "apps" ("id") ON DELETE CASCADE,
  CONSTRAINT "fk_app_default_tester_groups_tester_group"
    FOREIGN KEY ("tester_group_id") REFERENCES "tester_groups" ("id") ON DELETE CASCADE
);

CREATE TABLE "app_api_keys" (
  "id"            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  "app_id"        uuid NOT NULL,
  "label"         varchar NOT NULL,
  "token_hash"    varchar UNIQUE NOT NULL,
  "token_preview" varchar NOT NULL,
  "created_by"    uuid NOT NULL,
  "last_used_at"  timestamptz,
  "revoked_at"    timestamptz,
  "created_at"    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "fk_app_api_keys_app"
    FOREIGN KEY ("app_id") REFERENCES "apps" ("id") ON DELETE CASCADE,
  CONSTRAINT "fk_app_api_keys_created_by"
    FOREIGN KEY ("created_by") REFERENCES "users" ("id") ON DELETE RESTRICT
);

COMMENT ON COLUMN "app_api_keys"."token_hash" IS 'sha256 của key — plaintext chỉ trả về đúng 1 lần lúc tạo';

CREATE INDEX "idx_app_api_keys_app_created" ON "app_api_keys" ("app_id", "created_at");
