-- 000001_user
-- Người dùng nội bộ (người upload): users, sessions.
-- Sinh từ db/docs/db.dbml (nguồn sự thật). Đừng sửa tay lệch khỏi DBML.

-- gen_random_uuid() có sẵn từ PostgreSQL 13. Guard cho bản cũ hơn.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE "user_role" AS ENUM (
  'admin',
  'member'
);

CREATE TABLE "users" (
  "id"                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  "email"               varchar UNIQUE NOT NULL,
  "hashed_password"     varchar NOT NULL,
  "full_name"           varchar NOT NULL,
  "role"                user_role NOT NULL DEFAULT 'member',
  "active"              bool NOT NULL DEFAULT true,
  "password_changed_at" timestamptz NOT NULL DEFAULT '0001-01-01 00:00:00Z',
  "updated_at"          timestamptz,
  "created_at"          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE "sessions" (
  "id"            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  "user_id"       uuid NOT NULL,
  "refresh_token" varchar NOT NULL,
  "user_agent"    varchar NOT NULL,
  "client_ip"     varchar NOT NULL,
  "is_blocked"    boolean NOT NULL DEFAULT false,
  "expires_at"    timestamptz NOT NULL,
  "created_at"    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "fk_sessions_user"
    FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE
);

CREATE INDEX "idx_sessions_user_id" ON "sessions" ("user_id");
