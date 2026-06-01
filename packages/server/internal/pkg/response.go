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
	Code  ErrCode     `json:"code"`
	Msg   string      `json:"msg"`
	Data  interface{} `json:"data"`
	Total int         `json:"total"`
}

func Success(ctx iris.Context, data interface{}) {
	ctx.JSON(Response{Code: SUCCESS, Msg: GetErrMsg(SUCCESS), Data: data})
}

// SuccessPaginated 带分页的成功响应，只需提供 total
func SuccessPaginated(ctx iris.Context, data interface{}, total int) {
	ctx.JSON(PaginatedResponse{
		Code:  SUCCESS,
		Msg:   GetErrMsg(SUCCESS),
		Data:  data,
		Total: total,
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