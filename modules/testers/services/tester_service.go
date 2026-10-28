// Package services chứa nghiệp vụ tester group và phía tester (my-apps).
//
// Tester là người ngoài, đăng nhập bằng tài khoản role tester (tạo khi được
// thêm vào nhóm, kèm link mời). Thêm/khoá/xoá tester mở cho mọi role nội bộ.
package services

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/testers/dto"
	userDTO "github.com/longlbh/appdist-be/modules/users/dto"
)

// UserInviter tạo (hoặc tìm) tài khoản theo email và cấp link mời — hiện
// thực bởi users/services.Inviter, khai báo ở đây để module không phụ thuộc
// ngược vào module users.
type UserInviter interface {
	EnsureUser(ctx context.Context, q *sqlc.Queries, email string, role sqlc.UserRole, invitedBy uuid.UUID) (sqlc.User, string, error)
}

type TesterService struct {
	store   db.Store
	inviter UserInviter
}

func NewTesterService(store db.Store, inviter UserInviter) *TesterService {
	return &TesterService{store: store, inviter: inviter}
}

var (
	errTesterGroupNotFound = common.ErrNotFound("tester group")
	errTesterNotFound      = common.ErrNotFound("tester")
)

func (s *TesterService) CreateGroup(ctx context.Context, actor sqlc.User, p dto.CreateTesterGroupPayload) (*dto.TesterGroupDTO, error) {
	g, err := s.store.CreateTesterGroup(ctx, sqlc.CreateTesterGroupParams{Name: p.Name, CreatedBy: actor.ID})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	return &dto.TesterGroupDTO{ID: g.ID, Name: g.Name}, nil
}

func (s *TesterService) ListGroups(ctx context.Context) ([]dto.TesterGroupDTO, error) {
	rows, err := s.store.ListTesterGroups(ctx)
	if err != nil {
		return nil, common.ErrDB(err)
	}
	out := make([]dto.TesterGroupDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.TesterGroupDTO{ID: r.TesterGroup.ID, Name: r.TesterGroup.Name, MemberCount: r.MemberCount})
	}
	return out, nil
}

func (s *TesterService) GetGroup(ctx context.Context, id uuid.UUID) (*dto.TesterGroupDTO, error) {
	r, err := s.store.GetTesterGroup(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errTesterGroupNotFound
		}
		return nil, common.ErrDB(err)
	}
	return &dto.TesterGroupDTO{ID: r.TesterGroup.ID, Name: r.TesterGroup.Name, MemberCount: r.MemberCount}, nil
}

// ListTesters trả tester chưa xoá (deleted=false) hoặc chỉ tester đã xoá.
func (s *TesterService) ListTesters(ctx context.Context, groupID uuid.UUID, deleted bool) ([]dto.TesterDTO, error) {
	if _, err := s.GetGroup(ctx, groupID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListTesters(ctx, sqlc.ListTestersParams{TesterGroupID: groupID, Deleted: deleted})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	out := make([]dto.TesterDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.TesterDTO{ID: r.ID, Email: r.Email, Status: r.Status})
	}
	return out, nil
}

// AddTesters thêm nhiều email một lần. Email chưa có tài khoản thì tạo tài
// khoản tester + link mời; email đã có trong nhóm thì trả bản ghi cũ (kích
// hoạt lại nếu đang khoá/xoá).
func (s *TesterService) AddTesters(ctx context.Context, actor sqlc.User, groupID uuid.UUID, p dto.AddTestersPayload) ([]dto.TesterDTO, error) {
	if _, err := s.GetGroup(ctx, groupID); err != nil {
		return nil, err
	}

	emails := make([]string, 0, len(p.Emails))
	for _, e := range p.Emails {
		e = userDTO.NormalizeEmail(e)
		if !slices.Contains(emails, e) {
			emails = append(emails, e)
		}
	}

	out := make([]dto.TesterDTO, 0, len(emails))
	err := s.store.ExecTx(ctx, func(q *sqlc.Queries) error {
		for _, email := range emails {
			user, link, err := s.inviter.EnsureUser(ctx, q, email, sqlc.UserRoleTester, actor.ID)
			if err != nil {
				return err
			}
			t, err := q.UpsertTester(ctx, sqlc.UpsertTesterParams{TesterGroupID: groupID, UserID: user.ID})
			if err != nil {
				return err
			}
			out = append(out, dto.TesterDTO{ID: t.ID, Email: user.Email, Status: t.Status, InviteURL: link})
		}
		return nil
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	return out, nil
}

// SetTesterStatus khoá/mở khoá (tester phải chưa bị xoá — khôi phục dùng Restore).
func (s *TesterService) SetTesterStatus(ctx context.Context, groupID, testerID uuid.UUID, status sqlc.EntityStatus) (*dto.TesterDTO, error) {
	t, err := s.loadTester(ctx, groupID, testerID)
	if err != nil {
		return nil, err
	}
	if t.Status == sqlc.EntityStatusDeleted {
		return nil, errTesterNotFound
	}
	return s.updateStatus(ctx, groupID, t, status)
}

func (s *TesterService) DeleteTester(ctx context.Context, groupID, testerID uuid.UUID) error {
	t, err := s.loadTester(ctx, groupID, testerID)
	if err != nil {
		return err
	}
	_, err = s.updateStatus(ctx, groupID, t, sqlc.EntityStatusDeleted)
	return err
}

func (s *TesterService) RestoreTester(ctx context.Context, groupID, testerID uuid.UUID) (*dto.TesterDTO, error) {
	t, err := s.loadTester(ctx, groupID, testerID)
	if err != nil {
		return nil, err
	}
	return s.updateStatus(ctx, groupID, t, sqlc.EntityStatusActive)
}

func (s *TesterService) updateStatus(ctx context.Context, groupID uuid.UUID, t sqlc.GetTesterRow, status sqlc.EntityStatus) (*dto.TesterDTO, error) {
	err := s.store.UpdateTesterStatus(ctx, sqlc.UpdateTesterStatusParams{TesterGroupID: groupID, ID: t.ID, Status: status})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	return &dto.TesterDTO{ID: t.ID, Email: t.Email, Status: status}, nil
}

func (s *TesterService) loadTester(ctx context.Context, groupID, testerID uuid.UUID) (sqlc.GetTesterRow, error) {
	t, err := s.store.GetTester(ctx, sqlc.GetTesterParams{TesterGroupID: groupID, ID: testerID})
	if err != nil {
		if db.IsNotFound(err) {
			return sqlc.GetTesterRow{}, errTesterNotFound
		}
		return sqlc.GetTesterRow{}, common.ErrDB(err)
	}
	return t, nil
}
