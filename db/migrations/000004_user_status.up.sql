-- 000004_user_status
-- users.active (bool) → users.status (active|disabled|deleted), thêm lời mời.
-- Sinh từ db/docs/db.dbml (nguồn sự thật). Đừng sửa tay lệch khỏi DBML.

CREATE TYPE "entity_status" AS ENUM (
  'active',
  'disabled',
  'deleted'
);

ALTER TABLE "users"
  ADD COLUMN "status" entity_status NOT NULL DEFAULT 'active',
  ADD COLUMN "invited_by" uuid,
  ADD CONSTRAINT "fk_users_invited_by"
    FOREIGN KEY ("invited_by") REFERENCES "users" ("id") ON DELETE SET NULL;

UPDATE "users" SET "status" = 'disabled' WHERE NOT "active";
ALTER TABLE "users" DROP COLUMN "active";

-- Hệ thống luôn cần ít nhất một owner: nâng admin tạo sớm nhất.
UPDATE "users" SET "role" = 'owner'
WHERE "id" = (SELECT "id" FROM "users" WHERE "role" = 'admin' ORDER BY "created_at" LIMIT 1)
  AND NOT EXISTS (SELECT 1 FROM "users" WHERE "role" = 'owner');

CREATE TABLE "invitations" (
  "id"          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  "user_id"     uuid NOT NULL,
  "token_hash"  varchar UNIQUE NOT NULL,
  "expires_at"  timestamptz NOT NULL,
  "accepted_at" timestamptz,
  "created_at"  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "fk_invitations_user"
    FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE
);

COMMENT ON COLUMN "invitations"."token_hash" IS 'sha256 của token trên link mời — không lưu plaintext';

CREATE INDEX "idx_invitations_user_id" ON "invitations" ("user_id");
