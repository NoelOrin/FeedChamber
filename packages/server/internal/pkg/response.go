package pkg

import (
	"net/http"

	"github.com/kataras/iris/v12"
)

// Response 统一响应体
//   成功: { "code": 0,     "msg": "success", "data": {...} }
//   失败: { "code": 1001,  "msg": "token ...", "data": null }
type Response struct {
	Code ErrCode     `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// PaginatedResponse 分页响应体
type PaginatedResponse struct {
	Code       ErrCode     `json:"code"`
	Msg        string      `json:"msg"`
	Data       interface{} `json:"data"`
	Pagination *Pagination `json:"pagination"`
}

// Pagination 分页信息
type Pagination struct {
	CurrentPage int `json:"currentPage"`
	PageSize    int `json:"pageSize"`
	TotalItems  int `json:"totalItems"`
	TotalPages  int `json:"totalPages"`
}

func Success(ctx iris.Context, data interface{}) {
	ctx.JSON(Response{Code: SUCCESS, Msg: GetErrMsg(SUCCESS), Data: data})
}

// SuccessPaginated 带分页的成功响应
func SuccessPaginated(ctx iris.Context, data interface{}, page, pageSize, totalItems int) {
	totalPages := 0
	if pageSize > 0 {
		totalPages = (totalItems + pageSize - 1) / pageSize
	}
	ctx.JSON(PaginatedResponse{
		Code: SUCCESS,
		Msg:  GetErrMsg(SUCCESS),
		Data: data,
		Pagination: &Pagination{
			CurrentPage: page,
			PageSize:    pageSize,
			TotalItems:  totalItems,
			TotalPages:  totalPages,
		},
	})
}

func Fail(ctx iris.Context, code ErrCode, msg ...string) {
	m := GetErrMsg(code)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	ctx.StatusCode(errToHTTPStatus(code))
	ctx.JSON(Response{Code: code, Msg: m, Data: nil})
}

func HandleError(ctx iris.Context, err error) {
	if appErr, ok := err.(*AppError); ok {
		Fail(ctx, appErr.Code, appErr.Message)
		return
	}
	Fail(ctx, INTERNAL_ERROR)
}

func errToHTTPStatus(code ErrCode) int {
	switch {
	case code == FORBIDDEN || code == USER_BANNED:
		return http.StatusForbidden
	case code >= 1001 && code <= 1016:
		return http.StatusUnauthorized
	case code >= 2001 && code <= 2999:
		return http.StatusBadRequest
	case code == NOT_FOUND:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}