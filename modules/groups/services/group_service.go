// Package services chứa nghiệp vụ nhóm quản trị: member nào quản lý App nào.
//
// Thêm/bớt member vào nhóm chỉ ảnh hưởng phạm vi quản lý App — không đụng tới
// trạng thái tài khoản của member (khoá/xoá member nằm ở module users).
package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/groups/dto"
	userDTO "github.com/longlbh/appdist-be/modules/users/dto"
)

type GroupService struct {
	store db.Store
}

func NewGroupService(store db.Store) *GroupService {
	return &GroupService{store: store}
}

var (
	errGroupNotFound  = common.ErrNotFound("nhóm")
	errParentNotFound = common.ErrBadRequest("nhóm cha không tồn tại hoặc đã bị xoá", "ErrParentGroupNotFound")
	errMemberInvalid  = common.ErrBadRequest("tài khoản không tồn tại, đã bị xoá hoặc là tester", "ErrUserNotFound")
)

func (s *GroupService) Create(ctx context.Context, actor sqlc.User, p dto.CreateGroupPayload) (*dto.GroupDTO, error) {
	if p.ParentID != nil {
		parent, err := s.store.GetGroup(ctx, *p.ParentID)
		if err != nil {
			if db.IsNotFound(err) {
				return nil, errParentNotFound
			}
			return nil, common.ErrDB(err)
		}
		if parent.Status == sqlc.EntityStatusDeleted {
			return nil, errParentNotFound
		}
	}

	g, err := s.store.CreateGroup(ctx, sqlc.CreateGroupParams{
		Name:      p.Name,
		ParentID:  p.ParentID,
		CreatedBy: actor.ID,
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	res := dto.FromSQLC(g, nil)
	return &res, nil
}

// List trả nhóm chưa xoá (deleted=false) hoặc chỉ nhóm đã xoá.
func (s *GroupService) List(ctx context.Context, deleted bool) ([]dto.GroupDTO, error) {
	groups, err := s.store.ListGroups(ctx, deleted)
	if err != nil {
		return nil, common.ErrDB(err)
	}

	ids := make([]uuid.UUID, 0, len(groups))
	for _, g := range groups {
		ids = append(ids, g.ID)
	}
	members, err := s.membersOf(ctx, ids)
	if err != nil {
		return nil, err
	}

	out := make([]dto.GroupDTO, 0, len(groups))
	for _, g := range groups {
		out = append(out, dto.FromSQLC(g, members[g.ID]))
	}
	return out, nil
}

func (s *GroupService) Get(ctx context.Context, id uuid.UUID) (*dto.GroupDTO, error) {
	g, err := s.load(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.toDTO(ctx, g)
}

// AddMember thêm 1 user làm thành viên nhóm. Đã có thì không làm gì.
func (s *GroupService) AddMember(ctx context.Context, groupID, userID uuid.UUID) (*dto.GroupDTO, error) {
	g, err := s.loadAlive(ctx, groupID)
	if err != nil {
		return nil, err
	}

	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errMemberInvalid
		}
		return nil, common.ErrDB(err)
	}
	if user.Role == sqlc.UserRoleTester || user.Status == sqlc.EntityStatusDeleted {
		return nil, errMemberInvalid
	}

	if err := s.store.AddGroupMember(ctx, sqlc.AddGroupMemberParams{GroupID: groupID, UserID: userID}); err != nil {
		return nil, common.ErrDB(err)
	}
	return s.toDTO(ctx, g)
}

// RemoveMember bỏ 1 thành viên khỏi nhóm. Không có trong nhóm thì không làm gì.
func (s *GroupService) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) (*dto.GroupDTO, error) {
	g, err := s.loadAlive(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if err := s.store.RemoveGroupMember(ctx, sqlc.RemoveGroupMemberParams{GroupID: groupID, UserID: userID}); err != nil {
		return nil, common.ErrDB(err)
	}
	return s.toDTO(ctx, g)
}

// SetStatus khoá/mở khoá (nhóm phải chưa bị xoá — khôi phục dùng Restore).
func (s *GroupService) SetStatus(ctx context.Context, id uuid.UUID, status sqlc.EntityStatus) (*dto.GroupDTO, error) {
	if _, err := s.loadAlive(ctx, id); err != nil {
		return nil, err
	}
	return s.updateStatus(ctx, id, status)
}

// Delete xoá mềm: nhóm và member giữ nguyên, chỉ mất quyền quản lý App.
func (s *GroupService) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.load(ctx, id); err != nil {
		return err
	}
	_, err := s.updateStatus(ctx, id, sqlc.EntityStatusDeleted)
	return err
}

// Restore đưa nhóm bị khoá hoặc đã xoá về active.
func (s *GroupService) Restore(ctx context.Context, id uuid.UUID) (*dto.GroupDTO, error) {
	if _, err := s.load(ctx, id); err != nil {
		return nil, err
	}
	return s.updateStatus(ctx, id, sqlc.EntityStatusActive)
}

func (s *GroupService) updateStatus(ctx context.Context, id uuid.UUID, status sqlc.EntityStatus) (*dto.GroupDTO, error) {
	g, err := s.store.UpdateGroupStatus(ctx, sqlc.UpdateGroupStatusParams{ID: id, Status: status})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	return s.toDTO(ctx, g)
}

func (s *GroupService) load(ctx context.Context, id uuid.UUID) (sqlc.Group, error) {
	g, err := s.store.GetGroup(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return sqlc.Group{}, errGroupNotFound
		}
		return sqlc.Group{}, common.ErrDB(err)
	}
	return g, nil
}

// loadAlive như load nhưng coi nhóm đã xoá là không tồn tại.
func (s *GroupService) loadAlive(ctx context.Context, id uuid.UUID) (sqlc.Group, error) {
	g, err := s.load(ctx, id)
	if err != nil {
		return sqlc.Group{}, err
	}
	if g.Status == sqlc.EntityStatusDeleted {
		return sqlc.Group{}, errGroupNotFound
	}
	return g, nil
}

func (s *GroupService) toDTO(ctx context.Context, g sqlc.Group) (*dto.GroupDTO, error) {
	members, err := s.membersOf(ctx, []uuid.UUID{g.ID})
	if err != nil {
		return nil, err
	}
	res := dto.FromSQLC(g, members[g.ID])
	return &res, nil
}

// membersOf lấy member của nhiều nhóm trong một query.
func (s *GroupService) membersOf(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]userDTO.MemberRefDTO, error) {
	out := make(map[uuid.UUID][]userDTO.MemberRefDTO, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.store.ListGroupMembers(ctx, ids)
	if err != nil {
		return nil, common.ErrDB(err)
	}
	for _, r := range rows {
		out[r.GroupID] = append(out[r.GroupID], userDTO.MemberRefDTO{
			ID:    r.ID,
			Name:  r.FullName,
			Email: r.Email,
			Role:  r.Role,
		})
	}
	return out, nil
}
