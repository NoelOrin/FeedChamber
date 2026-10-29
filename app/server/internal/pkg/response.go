package pkg

import (
	"net/http"

	"github.com/kataras/iris/v12"
)

// Response 统一响应体
//
//	成功: { "code": 0,     "msg": "success", "data": {...} }
//	失败: { "code": 1001,  "msg": "token ...", "data": null }
type Response struct {
	Code ErrCode     `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// PaginatedData 分页数据体
type PaginatedData struct {
	Items interface{} `json:"items"`
	Total int         `json:"total"`
}

func Success(ctx iris.Context, data interface{}) {
	ctx.JSON(Response{Code: SUCCESS, Msg: GetErrMsg(SUCCESS), Data: data})
}

// SuccessPaginated 带分页的成功响应，自动包装为 { items, total }
func SuccessPaginated(ctx iris.Context, data interface{}, total int) {
	Success(ctx, PaginatedData{Items: data, Total: total})
}

func Fail(ctx iris.Context, code ErrCode, msg ...string) {
	FailStatus(ctx, errToHTTPStatus(code), code, msg...)
}

// FailStatus writes the standard envelope but pins the HTTP status.
// Use it only where the status is part of the contract rather than derived
// from the business code — e.g. the gateway hides banned accounts behind 404
// so that a ban does not disclose that the resource exists.
func FailStatus(ctx iris.Context, status int, code ErrCode, msg ...string) {
	m := GetErrMsg(code)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	ctx.StatusCode(status)
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
	case code == RATE_LIMITED:
		return http.StatusTooManyRequests
	case code == BODY_TOO_LARGE:
		return http.StatusRequestEntityTooLarge
	case code >= 1001 && code <= 1016:
		return http.StatusUnauthorized
	case code >= 2001 && code <= 2999:
		return http.StatusBadRequest
	case code == NOT_FOUND:
		return http.StatusNotFound
	case code == ALREADY_EXISTS:
		return http.StatusConflict
	case code == REQUEST_TIMEOUT:
		return http.StatusGatewayTimeout
	case code == MAINTENANCE:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
