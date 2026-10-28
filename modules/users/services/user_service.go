package services

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/users/dto"
	"github.com/longlbh/appdist-be/utils"
)

// UserService quản lý tài khoản nội bộ (owner/admin/member) và mật khẩu của
// chính mình. Việc user là thành viên nhóm nào nằm ở module groups.
type UserService struct {
	store db.Store
}

func NewUserService(store db.Store) *UserService {
	return &UserService{store: store}
}

var (
	errEmailTaken = common.ErrConflict("email đã được sử dụng", "ErrEmailTaken")

	errUserNotFound = common.ErrNotFound("tài khoản")

	errWrongCurrentPassword = common.ErrBadRequest("mật khẩu hiện tại không đúng", "ErrWrongCurrentPassword")

	// Tự khoá / tự đổi quyền có thể làm hệ thống mất owner cuối cùng.
	errCannotModifySelf = common.ErrBadRequest("không thể tự khoá, tự xoá hoặc tự đổi quyền của chính mình", "ErrCannotModifySelf")

	errAdminOnlyMember = common.ErrForbidden("admin chỉ được tạo tài khoản role member hoặc tester")

	errCannotTouchOwner = common.ErrForbidden("admin không được thay đổi tài khoản owner")
)

// Create tạo tài khoản dùng được ngay. Owner tạo được mọi role, admin chỉ
// tạo được role member hoặc tester.
func (s *UserService) Create(ctx context.Context, actor sqlc.User, p dto.CreateUserPayload) (*dto.UserDTO, error) {
	if actor.Role != sqlc.UserRoleOwner && p.Role != sqlc.UserRoleMember && p.Role != sqlc.UserRoleTester {
		return nil, errAdminOnlyMember
	}

	hashed, err := utils.HashPassword(p.Password)
	if err != nil {
		return nil, common.ErrInternal(err)
	}

	user, err := s.store.CreateUser(ctx, sqlc.CreateUserParams{
		Email:          dto.NormalizeEmail(p.Email),
		HashedPassword: hashed,
		FullName:       strings.TrimSpace(p.Name),
		Role:           p.Role,
		InvitedBy:      &actor.ID,
	})
	if err != nil {
		if db.UniqueViolation(err) != "" {
			return nil, errEmailTaken
		}
		return nil, common.ErrDB(err)
	}

	res := dto.FromSQLC(user)
	return &res, nil
}

// List trả tài khoản chưa bị xoá (deleted=false) hoặc chỉ tài khoản đã xoá,
// lọc theo role nếu có.
func (s *UserService) List(ctx context.Context, deleted bool, q dto.ListUsersQuery) (*common.Page[dto.UserDTO], error) {
	q.Normalize()

	var role sqlc.NullUserRole
	if q.Role != "" {
		role = sqlc.NullUserRole{UserRole: sqlc.UserRole(q.Role), Valid: true}
	}
	search := db.Text(q.Search)
	sortBy := q.Sort
	if sortBy == "" {
		sortBy = "createdAt"
	}

	users, err := s.store.ListUsers(ctx, sqlc.ListUsersParams{
		Deleted:    deleted,
		Role:       role,
		Search:     search,
		SortBy:     sortBy,
		SortAsc:    q.Order == "asc",
		PageLimit:  q.Limit(),
		PageOffset: q.Offset(),
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	// Đếm riêng (không dùng count(*) OVER()) để total vẫn đúng khi trang vượt quá trang cuối.
	total, err := s.store.CountUsers(ctx, sqlc.CountUsersParams{Deleted: deleted, Role: role, Search: search})
	if err != nil {
		return nil, common.ErrDB(err)
	}

	page := common.NewPage(dto.FromSQLCList(users), q.PageQuery, total)
	return &page, nil
}

func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*dto.UserDTO, error) {
	user, err := s.loadUser(ctx, id)
	if err != nil {
		return nil, err
	}
	res := dto.FromSQLC(user)
	return &res, nil
}

// Groups trả các nhóm (chưa xoá) mà user là thành viên.
func (s *UserService) Groups(ctx context.Context, id uuid.UUID) ([]dto.UserGroupDTO, error) {
	if _, err := s.loadUser(ctx, id); err != nil {
		return nil, err
	}
	rows, err := s.store.ListUserGroups(ctx, id)
	if err != nil {
		return nil, common.ErrDB(err)
	}
	out := make([]dto.UserGroupDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.UserGroupDTO{ID: r.ID, Name: r.Name, Status: r.Status})
	}
	return out, nil
}

// UpdateRole đổi role. Route đã chặn chỉ owner gọi được.
//
// Hạ xuống tester thì gỡ luôn khỏi mọi nhóm quản trị: tester không được quản
// lý App, để membership cũ lại thì nhóm vẫn liệt kê họ như thành viên.
func (s *UserService) UpdateRole(ctx context.Context, actor sqlc.User, id uuid.UUID, role sqlc.UserRole) (*dto.UserDTO, error) {
	if actor.ID == id {
		return nil, errCannotModifySelf
	}
	if _, err := s.loadUser(ctx, id); err != nil {
		return nil, err
	}

	var user sqlc.User
	err := s.store.ExecTx(ctx, func(q *sqlc.Queries) error {
		var err error
		user, err = q.UpdateUserRole(ctx, sqlc.UpdateUserRoleParams{ID: id, Role: role})
		if err != nil {
			return err
		}
		if role == sqlc.UserRoleTester {
			return q.RemoveUserFromAllGroups(ctx, id)
		}
		return nil
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	res := dto.FromSQLC(user)
	return &res, nil
}

// SetStatus khoá / mở khoá / xoá / khôi phục một tài khoản.
//
// Owner tác động được mọi người; admin không đụng được owner. Không ai tự
// tác động lên chính mình. Mọi trạng thái khác active đều chặn luôn các phiên
// đang mở — khoá mà phiên cũ vẫn chạy thì khoá cũng như không.
func (s *UserService) SetStatus(ctx context.Context, actor sqlc.User, id uuid.UUID, status sqlc.EntityStatus) (*dto.UserDTO, error) {
	if actor.ID == id {
		return nil, errCannotModifySelf
	}
	target, err := s.loadUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if actor.Role != sqlc.UserRoleOwner && target.Role == sqlc.UserRoleOwner {
		return nil, errCannotTouchOwner
	}

	var user sqlc.User
	err = s.store.ExecTx(ctx, func(q *sqlc.Queries) error {
		var err error
		user, err = q.UpdateUserStatus(ctx, sqlc.UpdateUserStatusParams{ID: id, Status: status})
		if err != nil {
			return err
		}
		if status != sqlc.EntityStatusActive {
			return q.BlockAllUserSessions(ctx, id)
		}
		return nil
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}

	res := dto.FromSQLC(user)
	return &res, nil
}

// ChangePassword đổi mật khẩu của chính mình và đăng xuất mọi phiên khác —
// đổi mật khẩu thường vì nghi bị lộ, giữ phiên cũ thì đổi cũng như không.
func (s *UserService) ChangePassword(ctx context.Context, user sqlc.User, p dto.ChangePasswordPayload) error {
	if err := utils.CheckPassword(p.CurrentPassword, user.HashedPassword); err != nil {
		return errWrongCurrentPassword
	}

	hashed, err := utils.HashPassword(p.NewPassword)
	if err != nil {
		return common.ErrInternal(err)
	}

	err = s.store.ExecTx(ctx, func(q *sqlc.Queries) error {
		if _, err := q.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
			ID:             user.ID,
			HashedPassword: hashed,
		}); err != nil {
			return err
		}
		return q.BlockAllUserSessions(ctx, user.ID)
	})
	if err != nil {
		return common.ErrDB(err)
	}
	return nil
}

func (s *UserService) loadUser(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return sqlc.User{}, errUserNotFound
		}
		return sqlc.User{}, common.ErrDB(err)
	}
	return user, nil
}

// wrapDB giữ nguyên lỗi nghiệp vụ (*AppError) trả từ trong transaction, còn
// lại coi là lỗi DB.
func wrapDB(err error) error {
	var appErr *common.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return common.ErrDB(err)
}
