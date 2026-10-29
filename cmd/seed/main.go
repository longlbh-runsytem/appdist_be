// Command seed nạp dữ liệu demo (tài khoản đủ 4 role, nhóm, nhóm tester, App)
// vào DB MỚI — để FE và tester có sẵn tài khoản đăng nhập ngay sau khi dựng
// môi trường.
//
// Chỉ chạy khi SEED_DEMO_DATA=true VÀ bảng users đang trống: DB đã có người
// dùng thì bỏ qua toàn bộ, nên không ghi đè mật khẩu đã đổi hay tạo lại tài
// khoản đã xoá. Mật khẩu demo rất yếu — KHÔNG bật trên production.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"

	"github.com/longlbh/appdist-be/db/connections"
	"github.com/longlbh/appdist-be/utils"
)

// ID cố định để FE/test có thể tham chiếu trực tiếp.
const (
	ownerID  = "a153e647-a8a7-499b-8bd0-a15e48ce5be7"
	adminID  = "b1b437c6-ce95-4418-85cf-97cefc2877b7"
	memberID = "2ee9ad48-79b6-4fc3-a485-abada92b279f"
	testerID = "b04eafa1-b3b9-4e2e-b7cd-688f7a977df8"

	groupID       = "6f1d2c3e-0a4b-4c5d-8e6f-7a8b9c0d1e01"
	testerGroupID = "6f1d2c3e-0a4b-4c5d-8e6f-7a8b9c0d1e02"
	appID         = "6f1d2c3e-0a4b-4c5d-8e6f-7a8b9c0d1e03"
)

type demoUser struct {
	id, email, password, name, role string
}

var users = []demoUser{
	{ownerID, "owner@example.com", "owner123", "Dung Phung", "owner"},
	{adminID, "admin@example.com", "admin123", "Admin User", "admin"},
	{memberID, "member@example.com", "member123", "Member User", "member"},
	{testerID, "qa1@example.com", "tester123", "QA Tester 1", "tester"},
}

func main() {
	if os.Getenv("SEED_DEMO_DATA") != "true" {
		fmt.Println("SEED_DEMO_DATA khác true — bỏ qua seed")
		return
	}
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "seed thất bại:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := utils.LoadConfig(".", "app")
	if err != nil {
		return err
	}

	pool, err := connections.NewPool(cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	var hasUsers bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&hasUsers); err != nil {
		return err
	}
	if hasUsers {
		fmt.Println("DB đã có người dùng — bỏ qua seed")
		return nil
	}

	// Băm trước khi mở transaction: bcrypt chậm, không giữ transaction lâu.
	hashes := make([]string, len(users))
	for i, u := range users {
		if hashes[i], err = utils.HashPassword(u.password); err != nil {
			return err
		}
	}

	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		for i, u := range users {
			var invitedBy any // owner không do ai mời
			if u.id != ownerID {
				invitedBy = ownerID
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO users (id, email, hashed_password, full_name, role, invited_by)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				u.id, u.email, hashes[i], u.name, u.role, invitedBy); err != nil {
				return fmt.Errorf("user %s: %w", u.email, err)
			}
		}

		// Admin và member quản lý Demo App qua nhóm Demo Team (quy tắc scope:
		// thành viên nhóm quản trị của App); qa1 nhận build qua QA Team.
		steps := []struct {
			name string
			sql  string
			args []any
		}{
			{"group", `INSERT INTO groups (id, name, created_by) VALUES ($1, 'Demo Team', $2)`,
				[]any{groupID, ownerID}},
			{"group members", `INSERT INTO group_members (group_id, user_id) VALUES ($1, $2), ($1, $3)`,
				[]any{groupID, adminID, memberID}},
			{"tester group", `INSERT INTO tester_groups (id, name, created_by) VALUES ($1, 'QA Team', $2)`,
				[]any{testerGroupID, ownerID}},
			{"tester", `INSERT INTO testers (tester_group_id, user_id) VALUES ($1, $2)`,
				[]any{testerGroupID, testerID}},
			{"app", `
				INSERT INTO apps (id, name, ios_bundle_id, android_bundle_id, auto_assign_tester_groups, created_by)
				VALUES ($1, 'Demo App', 'com.example.demo', 'com.example.demo', true, $2)`,
				[]any{appID, ownerID}},
			{"app managing group", `INSERT INTO app_managing_groups (app_id, group_id) VALUES ($1, $2)`,
				[]any{appID, groupID}},
			{"app default tester group", `INSERT INTO app_default_tester_groups (app_id, tester_group_id) VALUES ($1, $2)`,
				[]any{appID, testerGroupID}},
		}
		for _, s := range steps {
			if _, err := tx.Exec(ctx, s.sql, s.args...); err != nil {
				return fmt.Errorf("%s: %w", s.name, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, u := range users {
		fmt.Printf("Đã tạo %-6s %s (%s)\n", u.role, u.email, u.id)
	}
	fmt.Println("Đã tạo nhóm Demo Team, nhóm tester QA Team, App Demo App (com.example.demo)")
	return nil
}
