-- 000006_tester
-- Tester group: người ngoài được mời xem build, khác với nhóm quản trị.
-- Sinh từ db/docs/db.dbml (nguồn sự thật). Đừng sửa tay lệch khỏi DBML.

CREATE TABLE "tester_groups" (
  "id"         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  "name"       varchar NOT NULL,
  "created_by" uuid NOT NULL,
  "updated_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "fk_tester_groups_created_by"
    FOREIGN KEY ("created_by") REFERENCES "users" ("id") ON DELETE RESTRICT
);

CREATE TABLE "testers" (
  "id"              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  "tester_group_id" uuid NOT NULL,
  "user_id"         uuid NOT NULL,
  "status"          entity_status NOT NULL DEFAULT 'active',
  "updated_at"      timestamptz,
  "created_at"      timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "fk_testers_group"
    FOREIGN KEY ("tester_group_id") REFERENCES "tester_groups" ("id") ON DELETE CASCADE,
  CONSTRAINT "fk_testers_user"
    FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE
);

CREATE UNIQUE INDEX "uq_testers_group_user" ON "testers" ("tester_group_id", "user_id");
CREATE INDEX "idx_testers_user_id" ON "testers" ("user_id");
