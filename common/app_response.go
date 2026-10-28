package common

// SuccessRes là vỏ bọc chuẩn cho mọi response thành công.
//
// Export (không phải successRes như trước) để swag tham chiếu được trong
// annotation: `{object} common.SuccessRes{data=dto.LoginResponse}`.
type SuccessRes struct {
	Code    int    `json:"code" example:"200"`
	Message string `json:"message" example:"success"`
	Data    any    `json:"data"`
	Paging  any    `json:"paging,omitempty"`
	Filter  any    `json:"filter,omitempty"`
}

func NewSuccessResponse(data, paging, filter any) *SuccessRes {
	return &SuccessRes{
		Code:    200,
		Message: "success",
		Data:    data,
		Paging:  paging,
		Filter:  filter,
	}
}

func SimpleSuccessResponse(data any) *SuccessRes {
	return NewSuccessResponse(data, nil, nil)
}
