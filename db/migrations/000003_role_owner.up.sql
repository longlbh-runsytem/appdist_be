-- 000003_role_owner
-- Thêm role owner và tester. Tách migration riêng: giá trị enum vừa ADD
-- không dùng được trong cùng transaction, mà 000004 cần dùng 'owner'.

ALTER TYPE "user_role" ADD VALUE IF NOT EXISTS 'owner';
ALTER TYPE "user_role" ADD VALUE IF NOT EXISTS 'tester';
