// Package handler là tầng duy nhất của module stats biết đến gin.
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/middlewares"
	"github.com/longlbh/appdist-be/modules/stats/dto"
	"github.com/longlbh/appdist-be/modules/stats/services"
)

type StatsHandler struct {
	svc *services.StatsService
}

func NewStatsHandler(svc *services.StatsService) *StatsHandler {
	return &StatsHandler{svc: svc}
}

// Summary godoc
//
//	@Summary		Số liệu tổng quan (lọc theo scope App)
//	@Description	totalDownloads = số lần tải file thật, không tính mở trang tải. totalTesters không lọc scope.
//	@Tags			stats
//	@Produce		json
//	@Success		200	{object}	common.SuccessRes{data=dto.SummaryDTO}
//	@Failure		401	{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/stats/summary [get]
func (h *StatsHandler) Summary() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		return h.svc.Summary(c.Request.Context(), user)
	})
}

// Downloads godoc
//
//	@Summary		Lượt tải theo ngày (giờ Việt Nam, lọc theo scope App)
//	@Description	Bỏ trống from/to = 7 ngày gần nhất. Tối đa 366 ngày.
//	@Tags			stats
//	@Produce		json
//	@Param			from	query		string	false	"YYYY-MM-DD"	format(date)
//	@Param			to		query		string	false	"YYYY-MM-DD"	format(date)
//	@Success		200		{object}	common.SuccessRes{data=[]dto.DailyDownloadsDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/stats/downloads [get]
func (h *StatsHandler) Downloads() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		var q dto.DownloadsQuery
		if err := c.ShouldBindQuery(&q); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.Downloads(c.Request.Context(), user, q)
	})
}
