package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/builds/dto"
)

// ApprovalService duyệt build của App bật requiresApproval.
//
// Build chờ duyệt có status pending_approval: link tải chưa chạy, tester chưa
// thấy. Duyệt → ready; từ chối → rejected (không phân phối được nữa).
type ApprovalService struct {
	store db.Store
	apps  AppAccess
}

func NewApprovalService(store db.Store, apps AppAccess) *ApprovalService {
	return &ApprovalService{store: store, apps: apps}
}

var (
	errApprovalNotFound = common.ErrNotFound("yêu cầu duyệt")
	errAlreadyReviewed  = common.ErrConflict("yêu cầu này đã được duyệt", "ErrAlreadyReviewed")
)

// ListPending trả yêu cầu đang chờ của các App trong scope, cũ nhất trước.
func (s *ApprovalService) ListPending(ctx context.Context, user sqlc.User) ([]dto.ApprovalDTO, error) {
	var scope *uuid.UUID
	if user.Role != sqlc.UserRoleOwner {
		scope = &user.ID
	}
	rows, err := s.store.ListPendingApprovals(ctx, scope)
	if err != nil {
		return nil, common.ErrDB(err)
	}
	out := make([]dto.ApprovalDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.ApprovalDTO{
			ID:          r.ID,
			BuildID:     r.BuildID,
			AppName:     r.AppName,
			Version:     r.Version,
			BuildNumber: r.BuildNumber,
			RequestedBy: r.RequestedByEmail,
			RequestedAt: r.RequestedAt,
		})
	}
	return out, nil
}

// Review duyệt hoặc từ chối. Hai người duyệt cùng lúc thì người sau nhận 409.
func (s *ApprovalService) Review(ctx context.Context, user sqlc.User, id uuid.UUID, p dto.ReviewPayload) (*dto.ReviewResultDTO, error) {
	req, err := s.store.GetApprovalRequest(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errApprovalNotFound
		}
		return nil, common.ErrDB(err)
	}
	if req.AppID == nil {
		return nil, errApprovalNotFound
	}
	if _, err := s.apps.Access(ctx, user, *req.AppID); err != nil {
		return nil, errApprovalNotFound
	}
	if req.ApprovalRequest.Decision.Valid {
		return nil, errAlreadyReviewed
	}

	status := sqlc.BuildStatusReady
	if p.Decision == sqlc.ApprovalDecisionReject {
		status = sqlc.BuildStatusRejected
	}

	var reviewed sqlc.ApprovalRequest
	err = s.store.ExecTx(ctx, func(q *sqlc.Queries) error {
		var err error
		reviewed, err = q.ReviewApprovalRequest(ctx, sqlc.ReviewApprovalRequestParams{
			ID:         id,
			Decision:   sqlc.NullApprovalDecision{ApprovalDecision: p.Decision, Valid: true},
			Note:       db.Text(p.Note),
			ReviewedBy: &user.ID,
		})
		if err != nil {
			if db.IsNotFound(err) {
				return errAlreadyReviewed
			}
			return err
		}
		return q.UpdateBuildStatus(ctx, sqlc.UpdateBuildStatusParams{ID: reviewed.BuildID, Status: status})
	})
	if err != nil {
		return nil, wrapDB(err)
	}

	return &dto.ReviewResultDTO{
		ID:         reviewed.ID,
		BuildID:    reviewed.BuildID,
		Decision:   p.Decision,
		Note:       reviewed.Note.String,
		ReviewedBy: user.Email,
		ReviewedAt: reviewed.ReviewedAt.Time,
	}, nil
}
