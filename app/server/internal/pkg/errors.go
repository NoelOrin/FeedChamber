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

	// Rate limit 4xxx
	RATE_LIMITED   ErrCode = 4001
	BODY_TOO_LARGE ErrCode = 4002

	// Server 5xxx
	INTERNAL_ERROR  ErrCode = 5001
	REQUEST_TIMEOUT ErrCode = 5004
	MAINTENANCE     ErrCode = 5005
)

var errMsg = map[ErrCode]string{
	// "success" is a fixed protocol literal, not user-facing copy: openspec
	// api-style pins the envelope to { code: 0, msg: "success" } and no client
	// renders msg on the success path. Failure strings stay Chinese for the UI.
	SUCCESS:          "success",
	TOKEN_NOT_EXIST:  "令牌不存在",
	TOKEN_WRONG:      "令牌错误",
	TOKEN_EXPIRED:    "令牌已过期",
	TOKEN_REVOKED:    "令牌已被撤销",
	INVALID_PASSWORD: "密码错误",
	USER_NOT_FOUND:   "用户不存在",
	USERNAME_EXISTS:  "用户名已存在",
	EMAIL_EXISTS:     "邮箱已存在",
	FORBIDDEN:        "禁止访问",
	USER_BANNED:      "用户已被封禁",
	INVALID_PARAMS:   "参数无效",
	NOT_FOUND:        "资源不存在",
	ALREADY_EXISTS:   "资源已存在",
	RATE_LIMITED:     "请求过于频繁，请稍后再试",
	BODY_TOO_LARGE:   "请求体过大",
	REQUEST_TIMEOUT:  "请求超时",
	MAINTENANCE:      "站点维护中，请稍后再试",
	INTERNAL_ERROR:   "服务器内部错误",
}

func GetErrMsg(code ErrCode) string {
	if msg, ok := errMsg[code]; ok {
		return msg
	}
	return "未知错误"
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
