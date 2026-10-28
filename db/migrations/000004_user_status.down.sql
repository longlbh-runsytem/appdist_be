DROP TABLE IF EXISTS "invitations";

ALTER TABLE "users" ADD COLUMN "active" bool NOT NULL DEFAULT true;
UPDATE "users" SET "active" = ("status" = 'active');

ALTER TABLE "users"
  DROP CONSTRAINT IF EXISTS "fk_users_invited_by",
  DROP COLUMN "invited_by",
  DROP COLUMN "status";

DROP TYPE IF EXISTS "entity_status";
