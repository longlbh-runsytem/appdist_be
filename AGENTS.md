# Agent instructions

For every Go task, always load and follow `samber/cc-skills-golang@golang-how-to`.

This service follows Clean Architecture. Dependencies point inward: adapters depend on use cases and entities, never the reverse.

- `db/docs/db.dbml` là nguồn sự thật của schema. Sửa DBML trước, rồi viết migration khớp theo.
- sqlc đọc schema từ `db/migrations`, không phải `db/docs/schema.sql`.
- Thông tin app luôn đọc từ nội dung file (components/appinfo), không tin giá trị client gửi.
