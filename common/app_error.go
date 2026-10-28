package common

import (
	"errors"
	"net/http"
)

type AppError struct {
	Code         int    `json:"statusCode"`
	RootErr      error  `json:"-"`
	Message      string `json:"message"`
	MessageTrans string `json:"messageTrans"`
	Log          string `json:"log"`
	Key          string `json:"errorKey"`
}

func NewErrorResponse(root error, msg, log, key string) *AppError {
	return &AppError{
		Code:    int(http.StatusBadRequest),
		RootErr: root,
		Message: msg,
		Log:     log,
		Key:     key,
	}
}

// Đệ quy
func (e *AppError) RootError() error {
	if err, ok := e.RootErr.(*AppError); ok {
		return err.RootError()
	}
	return e.RootErr
}

func (e *AppError) Error() string {
	return e.RootError().Error()
}

func NewFullErrorResponse(statusCode int, root error, msg, log, key string) *AppError {
	return &AppError{
		Code:    statusCode,
		RootErr: root,
		Message: msg,
		Log:     log,
		Key:     key,
	}
}

func NewUnauthorized(root error, msg string, key string) *AppError {
	return &AppError{
		Code:    http.StatusUnauthorized,
		RootErr: root,
		Message: msg,
		Key:     key,
	}
}

func NewCustomError(root error, msg, key string) *AppError {
	if root != nil {
		return NewErrorResponse(root, msg, root.Error(), key)
	}
	return NewErrorResponse(errors.New(msg), msg, msg, key)
}

// ErrWrongAuthHeader: thiếu hoặc sai định dạng "Authorization: Bearer <token>" → 401.
func ErrWrongAuthHeader(err error) *AppError {
	if err == nil {
		err = errors.New("wrong authen header")
	}
	return NewUnauthorized(err, "thiếu hoặc sai header Authorization: Bearer <token>", "ErrWrongAuthHeader")
}

func ErrDB(err error) *AppError {
	return NewFullErrorResponse(http.StatusInternalServerError, err, "something went wrong with DB", err.Error(), ErrorDb)
}

func ErrDBWithMsg(err error) *AppError {
	return NewFullErrorResponse(http.StatusInternalServerError, err, "something went wrong with DB", err.Error(), ErrorDb)
}

func ErrInvalidRequest(err error) *AppError {
	return NewErrorResponse(err, "invalid request", err.Error(), ErrInvalid)
}

func ErrInvalidRequestWithMsg(err error) *AppError {
	return NewErrorResponse(err, "invalid request", err.Error(), ErrInvalid)
}

func ErrInternal(err error) *AppError {
	return NewFullErrorResponse(http.StatusInternalServerError, err, "something went wrong in the server", err.Error(), ErrInternalCode)
}

func ErrInternalWithMsg(err error) *AppError {
	return NewFullErrorResponse(http.StatusInternalServerError, err, "something went wrong in the server", err.Error(), ErrInternalCode)
}

func ErrNoPermission(err error) *AppError {
	return NewFullErrorResponse(http.StatusForbidden, err, "user invalid", err.Error(), ErrNoPermissionCode)
}

// ErrNotFound là 404 thống nhất cho mọi tài nguyên. what là tên hiển thị, vd "nhóm".
func ErrNotFound(what string) *AppError {
	return NewFullErrorResponse(
		http.StatusNotFound,
		errors.New(what+" not found"),
		"không tìm thấy "+what,
		what+" not found",
		"ErrNotFound",
	)
}

// ErrForbidden là 403 kèm lý do cụ thể cho người dùng.
func ErrForbidden(msg string) *AppError {
	return NewFullErrorResponse(http.StatusForbidden, errors.New(msg), msg, msg, ErrNoPermissionCode)
}

// ErrBadRequest là 400 nghiệp vụ (không phải lỗi validate body).
func ErrBadRequest(msg, key string) *AppError {
	return NewCustomError(errors.New(msg), msg, key)
}

// ErrConflict là 409.
func ErrConflict(msg, key string) *AppError {
	return NewFullErrorResponse(http.StatusConflict, errors.New(msg), msg, msg, key)
}
