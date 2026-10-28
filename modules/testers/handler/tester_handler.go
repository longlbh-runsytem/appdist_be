// Package handler là tầng duy nhất của module testers biết đến gin.
package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/middlewares"
	"github.com/longlbh/appdist-be/modules/testers/dto"
	"github.com/longlbh/appdist-be/modules/testers/services"
	userDTO "github.com/longlbh/appdist-be/modules/users/dto"
)

type TesterHandler struct {
	svc *services.TesterService
}

func NewTesterHandler(svc *services.TesterService) *TesterHandler {
	return &TesterHandler{svc: svc}
}

// CreateGroup godoc
//
//	@Summary	Tạo tester group
//	@Tags		tester-groups
//	@Accept		json
//	@Produce	json
//	@Param		body	body		dto.CreateTesterGroupPayload	true	"Tên nhóm"
//	@Success	201		{object}	common.SuccessRes{data=dto.TesterGroupDTO}
//	@Failure	400		{object}	common.AppError
//	@Failure	401		{object}	common.AppError
//	@Failure	403		{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/tester-groups [post]
func (h *TesterHandler) CreateGroup() gin.HandlerFunc {
	return common.HandleCreated(func(c *gin.Context) (any, error) {
		actor, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		var p dto.CreateTesterGroupPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.CreateGroup(c.Request.Context(), actor, p)
	})
}

// ListGroups godoc
//
//	@Summary	Danh sách tester group
//	@Tags		tester-groups
//	@Produce	json
//	@Success	200	{object}	common.SuccessRes{data=[]dto.TesterGroupDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/tester-groups [get]
func (h *TesterHandler) ListGroups() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		return h.svc.ListGroups(c.Request.Context())
	})
}

// GetGroup godoc
//
//	@Summary	Chi tiết tester group
//	@Tags		tester-groups
//	@Produce	json
//	@Param		id	path		string	true	"Tester group ID"	format(uuid)
//	@Success	200	{object}	common.SuccessRes{data=dto.TesterGroupDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/tester-groups/{id} [get]
func (h *TesterHandler) GetGroup() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		id, err := common.ParamUUID(c, "id", "tester group")
		if err != nil {
			return nil, err
		}
		return h.svc.GetGroup(c.Request.Context(), id)
	})
}

// ListTesters godoc
//
//	@Summary	Tester trong nhóm (không gồm tester đã xoá)
//	@Tags		tester-groups
//	@Produce	json
//	@Param		id	path		string	true	"Tester group ID"	format(uuid)
//	@Success	200	{object}	common.SuccessRes{data=[]dto.TesterDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/tester-groups/{id}/testers [get]
func (h *TesterHandler) ListTesters() gin.HandlerFunc {
	return h.listTesters(false)
}

// ListDeletedTesters godoc
//
//	@Summary	Tester đã xoá trong nhóm
//	@Tags		tester-groups
//	@Produce	json
//	@Param		id	path		string	true	"Tester group ID"	format(uuid)
//	@Success	200	{object}	common.SuccessRes{data=[]dto.TesterDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/tester-groups/{id}/testers/deleted [get]
func (h *TesterHandler) ListDeletedTesters() gin.HandlerFunc {
	return h.listTesters(true)
}

func (h *TesterHandler) listTesters(deleted bool) gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		id, err := common.ParamUUID(c, "id", "tester group")
		if err != nil {
			return nil, err
		}
		return h.svc.ListTesters(c.Request.Context(), id, deleted)
	})
}

// AddTesters godoc
//
//	@Summary		Thêm tester theo email (tối đa 100/lần)
//	@Description	Email chưa có tài khoản → tạo tài khoản tester, response kèm inviteUrl để gửi cho tester.
//	@Description	Email đã có trong nhóm → trả bản ghi cũ và kích hoạt lại.
//	@Tags			tester-groups
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Tester group ID"	format(uuid)
//	@Param			body	body		dto.AddTestersPayload	true	"Danh sách email"
//	@Success		201		{object}	common.SuccessRes{data=[]dto.TesterDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		404		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/tester-groups/{id}/testers [post]
func (h *TesterHandler) AddTesters() gin.HandlerFunc {
	return common.HandleCreated(func(c *gin.Context) (any, error) {
		actor, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		id, err := common.ParamUUID(c, "id", "tester group")
		if err != nil {
			return nil, err
		}
		var p dto.AddTestersPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.AddTesters(c.Request.Context(), actor, id, p)
	})
}

// UpdateTesterStatus godoc
//
//	@Summary	Khoá / mở khoá tester
//	@Tags		tester-groups
//	@Accept		json
//	@Produce	json
//	@Param		id			path		string						true	"Tester group ID"	format(uuid)
//	@Param		testerId	path		string						true	"Tester ID"			format(uuid)
//	@Param		body		body		userDTO.UpdateStatusPayload	true	"active hoặc disabled"
//	@Success	200			{object}	common.SuccessRes{data=dto.TesterDTO}
//	@Failure	400			{object}	common.AppError
//	@Failure	401			{object}	common.AppError
//	@Failure	404			{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/tester-groups/{id}/testers/{testerId}/status [patch]
func (h *TesterHandler) UpdateTesterStatus() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		groupID, testerID, err := testerParams(c)
		if err != nil {
			return nil, err
		}
		var p userDTO.UpdateStatusPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.SetTesterStatus(c.Request.Context(), groupID, testerID, p.Status)
	})
}

// DeleteTester godoc
//
//	@Summary	Xoá mềm tester khỏi nhóm
//	@Tags		tester-groups
//	@Param		id			path	string	true	"Tester group ID"	format(uuid)
//	@Param		testerId	path	string	true	"Tester ID"			format(uuid)
//	@Success	204
//	@Failure	401	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/tester-groups/{id}/testers/{testerId} [delete]
func (h *TesterHandler) DeleteTester() gin.HandlerFunc {
	return common.HandleNoContent(func(c *gin.Context) error {
		groupID, testerID, err := testerParams(c)
		if err != nil {
			return err
		}
		return h.svc.DeleteTester(c.Request.Context(), groupID, testerID)
	})
}

// RestoreTester godoc
//
//	@Summary	Mở khoá hoặc khôi phục tester
//	@Tags		tester-groups
//	@Produce	json
//	@Param		id			path		string	true	"Tester group ID"	format(uuid)
//	@Param		testerId	path		string	true	"Tester ID"			format(uuid)
//	@Success	200			{object}	common.SuccessRes{data=dto.TesterDTO}
//	@Failure	401			{object}	common.AppError
//	@Failure	404			{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/tester-groups/{id}/testers/{testerId}/restore [post]
func (h *TesterHandler) RestoreTester() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		groupID, testerID, err := testerParams(c)
		if err != nil {
			return nil, err
		}
		return h.svc.RestoreTester(c.Request.Context(), groupID, testerID)
	})
}

func testerParams(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	groupID, err := common.ParamUUID(c, "id", "tester group")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	testerID, err := common.ParamUUID(c, "testerId", "tester")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return groupID, testerID, nil
}
