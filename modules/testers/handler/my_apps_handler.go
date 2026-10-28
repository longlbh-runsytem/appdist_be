package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/middlewares"
	appDTO "github.com/longlbh/appdist-be/modules/apps/dto"
	"github.com/longlbh/appdist-be/modules/testers/services"
)

type MyAppsHandler struct {
	svc *services.MyAppsService
}

func NewMyAppsHandler(svc *services.MyAppsService) *MyAppsHandler {
	return &MyAppsHandler{svc: svc}
}

// ListApps godoc
//
//	@Summary		App mà tester được mời xem (phân trang)
//	@Description	Chỉ App có ít nhất một build ready được gán cho tester group chứa tester (đang active).
//	@Tags			tester
//	@Produce		json
//	@Param			search		query		string	false	"Lọc theo tên"
//	@Param			from		query		string	false	"createdAt >= (RFC3339)"	format(date-time)
//	@Param			to			query		string	false	"createdAt <= (RFC3339)"	format(date-time)
//	@Param			page		query		int		false	"Mặc định 1"				minimum(1)
//	@Param			pageSize	query		int		false	"Mặc định 20"				minimum(1)	maximum(100)
//	@Success		200			{object}	common.SuccessRes{data=MyAppPage}
//	@Failure		400			{object}	common.AppError
//	@Failure		401			{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/tester/my-apps [get]
func (h *MyAppsHandler) ListApps() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		var q appDTO.ListMyAppsQuery
		if err := c.ShouldBindQuery(&q); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.ListApps(c.Request.Context(), user, q)
	})
}

// MyAppPage chỉ để Swagger mô tả common.Page[appDTO.AppSummaryDTO].
type MyAppPage struct {
	Items      []appDTO.AppSummaryDTO `json:"items"`
	Page       int32                  `json:"page"`
	PageSize   int32                  `json:"pageSize"`
	Total      int64                  `json:"total"`
	TotalPages int32                  `json:"totalPages"`
	HasMore    bool                   `json:"hasMore"`
}

// ListBuilds godoc
//
//	@Summary		Build ready của App mà tester được mời xem (lọc platform, sắp xếp, phân trang)
//	@Description	sort=buildNumber so theo thứ tự số ("9" < "45", "1.10" > "1.9"); cùng build number thì mới hơn trước.
//	@Description	counts = số build theo từng platform, KHÔNG phụ thuộc bộ lọc platform (để hiện số trên tab).
//	@Description	total/totalPages/hasMore tính theo bộ lọc platform đang áp.
//	@Tags			tester
//	@Produce		json
//	@Param			appId		path		string	true	"App ID"				format(uuid)
//	@Param			platform	query		string	false	"Lọc theo loại build"	Enums(ios, android)
//	@Param			sort		query		string	false	"Mặc định buildNumber"	Enums(buildNumber, uploadedAt)
//	@Param			order		query		string	false	"Mặc định desc"			Enums(asc, desc)
//	@Param			page		query		int		false	"Mặc định 1"			minimum(1)
//	@Param			pageSize	query		int		false	"Mặc định 20"			minimum(1)	maximum(100)
//	@Success		200			{object}	common.SuccessRes{data=appDTO.AppBuildPage}
//	@Failure		400			{object}	common.AppError
//	@Failure		401			{object}	common.AppError
//	@Failure		404			{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/tester/my-apps/{appId}/builds [get]
func (h *MyAppsHandler) ListBuilds() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		appID, err := common.ParamUUID(c, "appId", "App")
		if err != nil {
			return nil, err
		}
		var q appDTO.ListAppBuildsQuery
		if err := c.ShouldBindQuery(&q); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.ListBuilds(c.Request.Context(), user, appID, q)
	})
}
