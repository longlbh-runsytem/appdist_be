package common

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler là chữ ký handler của project: nhận *gin.Context, trả (data, error).
//
// Trả lỗi thay vì panic. Panic là để xử lý sự cố ngoài dự kiến, không phải để
// điều khiển luồng — dùng panic cho lỗi nghiệp vụ thì stack trace mất sạch
// ngữ cảnh và mọi lỗi đều quy về một mức nghiêm trọng.
type Handler func(c *gin.Context) (any, error)

// Handle chuyển Handler thành gin.HandlerFunc, gom việc ghi response về một chỗ.
//
// Nhờ vậy handler không phải lặp lại c.JSON(...) cho case thành công và
// c.AbortWithStatusJSON(...) cho mọi nhánh lỗi.
func Handle(fn Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := fn(c)
		if err != nil {
			WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, SimpleSuccessResponse(data))
	}
}

// HandleCreated như Handle nhưng trả 201 — dùng cho endpoint tạo tài nguyên.
func HandleCreated(fn Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := fn(c)
		if err != nil {
			WriteError(c, err)
			return
		}
		c.JSON(http.StatusCreated, SimpleSuccessResponse(data))
	}
}

// NoContentHandler là handler không có gì để trả ngoài lỗi.
type NoContentHandler func(c *gin.Context) error

// HandleNoContent trả 204 không có body — dùng cho endpoint xoá.
func HandleNoContent(fn NoContentHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := fn(c); err != nil {
			WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ParamUUID đọc path param dạng uuid; sai định dạng thì coi như không tồn tại.
func ParamUUID(c *gin.Context, name, what string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, ErrNotFound(what)
	}
	return id, nil
}

// WriteError ghi lỗi ra response với đúng HTTP status.
//
// Lỗi không phải *AppError được bọc thành 500 và KHÔNG lộ nội dung gốc ra
// ngoài — chi tiết chỉ nằm ở trường Log dành cho logging phía server.
func WriteError(c *gin.Context, err error) {
	var appErr *AppError
	if !errors.As(err, &appErr) {
		appErr = ErrInternal(err)
	}
	c.AbortWithStatusJSON(appErr.Code, appErr)
}
