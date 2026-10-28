// Command createadmin tạo tài khoản owner đầu tiên.
//
// Không có đăng ký công khai, nên phải có một đường ngoài API để sinh ra
// owner khởi đầu; các tài khoản sau đó owner/admin tạo qua POST /api/v1/users.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/longlbh/appdist-be/db/connections"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/users/dto"
	"github.com/longlbh/appdist-be/utils"
)

func main() {
	email := flag.String("email", "", "email đăng nhập")
	name := flag.String("name", "Admin", "họ tên")
	password := flag.String("password", "", "mật khẩu, tối thiểu 8 ký tự")
	flag.Parse()

	if *email == "" || len(*password) < 8 || len(*password) > 72 {
		fmt.Fprintln(os.Stderr, "cần -email và -password (8-72 ký tự)")
		os.Exit(2)
	}

	if err := run(*email, *name, *password); err != nil {
		fmt.Fprintln(os.Stderr, "tạo admin thất bại:", err)
		os.Exit(1)
	}
}

func run(email, name, password string) error {
	cfg, err := utils.LoadConfig(".", "app")
	if err != nil {
		return err
	}

	pool, err := connections.NewPool(cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	hashed, err := utils.HashPassword(password)
	if err != nil {
		return err
	}

	user, err := sqlc.New(pool).CreateUser(context.Background(), sqlc.CreateUserParams{
		Email:          dto.NormalizeEmail(email),
		HashedPassword: hashed,
		FullName:       strings.TrimSpace(name),
		Role:           sqlc.UserRoleOwner,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Đã tạo owner %s (%s)\n", user.Email, user.ID)
	return nil
}
