-- Postgres không xoá được giá trị enum: dựng lại kiểu cũ.
-- owner → admin, tester → member để không mất tài khoản.
ALTER TYPE "user_role" RENAME TO "user_role_old";
CREATE TYPE "user_role" AS ENUM ('admin', 'member');

ALTER TABLE "users" ALTER COLUMN "role" DROP DEFAULT;
ALTER TABLE "users" ALTER COLUMN "role" TYPE "user_role" USING (
  CASE "role"::text
    WHEN 'owner'  THEN 'admin'
    WHEN 'tester' THEN 'member'
    ELSE "role"::text
  END
)::"user_role";
ALTER TABLE "users" ALTER COLUMN "role" SET DEFAULT 'member';

DROP TYPE "user_role_old";
