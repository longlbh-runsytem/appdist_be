// Package handler là tầng duy nhất của module groups biết đến gin.
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/middlewares"
	"github.com/longlbh/appdist-be/modules/groups/dto"
	"github.com/longlbh/appdist-be/modules/groups/services"
	userDTO "github.com/longlbh/appdist-be/modules/users/dto"
)

type GroupHandler struct {
	svc *services.GroupService
}

func NewGroupHandler(svc *services.GroupService) *GroupHandler {
	return &GroupHandler{svc: svc}
}

// Create godoc
//
//	@Summary	Tạo nhóm quản trị (owner/admin)
//	@Tags		groups
//	@Accept		json
//	@Produce	json
//	@Param		body	body		dto.CreateGroupPayload	true	"Tên nhóm, nhóm cha (tuỳ chọn)"
//	@Success	201		{object}	common.SuccessRes{data=dto.GroupDTO}
//	@Failure	400		{object}	common.AppError
//	@Failure	401		{object}	common.AppError
//	@Failure	403		{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/groups [post]
func (h *GroupHandler) Create() gin.HandlerFunc {
	return common.HandleCreated(func(c *gin.Context) (any, error) {
		actor, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		var p dto.CreateGroupPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.Create(c.Request.Context(), actor, p)
	})
}

// List godoc
//
//	@Summary	Danh sách nhóm (không gồm nhóm đã xoá)
//	@Tags		groups
//	@Produce	json
//	@Success	200	{object}	common.SuccessRes{data=[]dto.GroupDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/groups [get]
func (h *GroupHandler) List() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		return h.svc.List(c.Request.Context(), false)
	})
}

// ListDeleted godoc
//
//	@Summary	Danh sách nhóm đã xoá (owner/admin)
//	@Tags		groups
//	@Produce	json
//	@Success	200	{object}	common.SuccessRes{data=[]dto.GroupDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/groups/deleted [get]
func (h *GroupHandler) ListDeleted() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		return h.svc.List(c.Request.Context(), true)
	})
}

// Get godoc
//
//	@Summary	Chi tiết nhóm
//	@Tags		groups
//	@Produce	json
//	@Param		id	path		string	true	"Group ID"	format(uuid)
//	@Success	200	{object}	common.SuccessRes{data=dto.GroupDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/groups/{id} [get]
func (h *GroupHandler) Get() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		id, err := common.ParamUUID(c, "id", "nhóm")
		if err != nil {
			return nil, err
		}
		return h.svc.Get(c.Request.Context(), id)
	})
}

// AddMember godoc
//
//	@Summary		Thêm 1 user làm thành viên nhóm (owner/admin)
//	@Description	User phải có tài khoản sẵn (POST /api/v1/users/create). 1 user là thành viên được nhiều nhóm.
//	@Description	Chỉ ảnh hưởng phạm vi nhóm, không đụng trạng thái tài khoản. Đã có thì không làm gì.
//	@Tags			groups
//	@Produce		json
//	@Param			id		path		string	true	"Group ID"	format(uuid)
//	@Param			userId	path		string	true	"User ID"	format(uuid)
//	@Success		200		{object}	common.SuccessRes{data=dto.GroupDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		404		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/groups/{id}/members/{userId} [post]
func (h *GroupHandler) AddMember() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		id, err := common.ParamUUID(c, "id", "nhóm")
		if err != nil {
			return nil, err
		}
		userID, err := common.ParamUUID(c, "userId", "tài khoản")
		if err != nil {
			return nil, err
		}
		return h.svc.AddMember(c.Request.Context(), id, userID)
	})
}

// RemoveMember godoc
//
//	@Summary	Bỏ 1 thành viên khỏi nhóm (owner/admin) — tài khoản giữ nguyên
//	@Tags		groups
//	@Produce	json
//	@Param		id		path		string	true	"Group ID"	format(uuid)
//	@Param		userId	path		string	true	"User ID"	format(uuid)
//	@Success	200		{object}	common.SuccessRes{data=dto.GroupDTO}
//	@Failure	401		{object}	common.AppError
//	@Failure	403		{object}	common.AppError
//	@Failure	404		{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/groups/{id}/members/{userId} [delete]
func (h *GroupHandler) RemoveMember() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		id, err := common.ParamUUID(c, "id", "nhóm")
		if err != nil {
			return nil, err
		}
		userID, err := common.ParamUUID(c, "userId", "tài khoản")
		if err != nil {
			return nil, err
		}
		return h.svc.RemoveMember(c.Request.Context(), id, userID)
	})
}

// UpdateStatus godoc
//
//	@Summary		Khoá / mở khoá nhóm (owner/admin)
//	@Description	Khoá nhóm → member trong nhóm mất quyền quản lý App do nhóm phụ trách ngay; member vẫn còn trong nhóm.
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"Group ID"	format(uuid)
//	@Param			body	body		userDTO.UpdateStatusPayload	true	"active hoặc disabled"
//	@Success		200		{object}	common.SuccessRes{data=dto.GroupDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		404		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/groups/{id}/status [patch]
func (h *GroupHandler) UpdateStatus() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		id, err := common.ParamUUID(c, "id", "nhóm")
		if err != nil {
			return nil, err
		}
		var p userDTO.UpdateStatusPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.SetStatus(c.Request.Context(), id, p.Status)
	})
}

// Delete godoc
//
//	@Summary	Xoá mềm nhóm (owner/admin)
//	@Tags		groups
//	@Param		id	path	string	true	"Group ID"	format(uuid)
//	@Success	204
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/groups/{id} [delete]
func (h *GroupHandler) Delete() gin.HandlerFunc {
	return common.HandleNoContent(func(c *gin.Context) error {
		id, err := common.ParamUUID(c, "id", "nhóm")
		if err != nil {
			return err
		}
		return h.svc.Delete(c.Request.Context(), id)
	})
}

// Restore godoc
//
//	@Summary	Mở khoá hoặc khôi phục nhóm đã xoá (owner/admin)
//	@Tags		groups
//	@Produce	json
//	@Param		id	path		string	true	"Group ID"	format(uuid)
//	@Success	200	{object}	common.SuccessRes{data=dto.GroupDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/groups/{id}/restore [post]
func (h *GroupHandler) Restore() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		id, err := common.ParamUUID(c, "id", "nhóm")
		if err != nil {
			return nil, err
		}
		return h.svc.Restore(c.Request.Context(), id)
	})
}
