-- 000005_group
-- Nhóm quản trị: quyết định member nào quản lý App nào.
-- Sinh từ db/docs/db.dbml (nguồn sự thật). Đừng sửa tay lệch khỏi DBML.

CREATE TABLE "groups" (
  "id"         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  "name"       varchar NOT NULL,
  "parent_id"  uuid,
  "status"     entity_status NOT NULL DEFAULT 'active',
  "created_by" uuid NOT NULL,
  "updated_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "fk_groups_parent"
    FOREIGN KEY ("parent_id") REFERENCES "groups" ("id") ON DELETE SET NULL,
  CONSTRAINT "fk_groups_created_by"
    FOREIGN KEY ("created_by") REFERENCES "users" ("id") ON DELETE RESTRICT
);

COMMENT ON COLUMN "groups"."parent_id" IS 'chỉ để hiển thị cây — KHÔNG kế thừa quyền';

CREATE TABLE "group_members" (
  "group_id" uuid NOT NULL,
  "user_id"  uuid NOT NULL,
  "added_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("group_id", "user_id"),
  CONSTRAINT "fk_group_members_group"
    FOREIGN KEY ("group_id") REFERENCES "groups" ("id") ON DELETE CASCADE,
  CONSTRAINT "fk_group_members_user"
    FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE
);

CREATE INDEX "idx_group_members_user_id" ON "group_members" ("user_id");
