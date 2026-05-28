package pkg

// ErrCode 业务状态码
// 0     = 成功
// 1xxx  = 认证相关
// 2xxx  = 参数校验
// 3xxx  = 资源相关
// 5xxx  = 服务端内部
type ErrCode int

const (
	SUCCESS ErrCode = 0

	// Auth 1xxx
	TOKEN_NOT_EXIST  ErrCode = 1001
	TOKEN_WRONG      ErrCode = 1002
	TOKEN_EXPIRED    ErrCode = 1003
	INVALID_PASSWORD ErrCode = 1010
	USER_NOT_FOUND   ErrCode = 1011
	USERNAME_EXISTS  ErrCode = 1012
	FORBIDDEN        ErrCode = 1013
	TOKEN_REVOKED    ErrCode = 1014
	USER_BANNED      ErrCode = 1015
	EMAIL_EXISTS     ErrCode = 1016

	// Params 2xxx
	INVALID_PARAMS ErrCode = 2001

	// Resource 3xxx
	NOT_FOUND      ErrCode = 3001
	ALREADY_EXISTS ErrCode = 3002

	// Server 5xxx
	INTERNAL_ERROR ErrCode = 5001
)

var errMsg = map[ErrCode]string{
	SUCCESS:         "success",
	TOKEN_NOT_EXIST: "token does not exist",
	TOKEN_WRONG:     "token is wrong",
	TOKEN_EXPIRED:   "token has expired",
	TOKEN_REVOKED:   "token has been revoked",
	INVALID_PASSWORD: "invalid password",
	USER_NOT_FOUND:  "user not found",
	USERNAME_EXISTS: "username already exists",
	EMAIL_EXISTS:    "email already exists",
	FORBIDDEN:       "forbidden",
	USER_BANNED:     "user has been banned",
	INVALID_PARAMS:  "invalid parameters",
	NOT_FOUND:       "resource not found",
	ALREADY_EXISTS:  "resource already exists",
	INTERNAL_ERROR:  "internal server error",
}

func GetErrMsg(code ErrCode) string {
	if msg, ok := errMsg[code]; ok {
		return msg
	}
	return "unknown error"
}

// AppError 业务错误，service 层直接返回
type AppError struct {
	Code    ErrCode
	Message string
}

func (e *AppError) Error() string { return e.Message }

func NewAppError(code ErrCode, msg ...string) *AppError {
	m := GetErrMsg(code)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	return &AppError{Code: code, Message: m}
}