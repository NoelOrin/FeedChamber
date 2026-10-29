package repository

import "errors"

// 统一数据库错误 —— 所有 Repository 实现在 Find 失败时应返回这些错误，
// 而非直接返回 gorm.ErrRecordNotFound。
// Service 层统一使用 errors.Is(err, ErrNotFound) 判断。
var (
	ErrNotFound  = errors.New("record not found")
	ErrDuplicate = errors.New("record already exists")

	// 邀请码原子消耗失败语义。
	ErrInviteExpired   = errors.New("invite expired")
	ErrInviteExhausted = errors.New("invite exhausted")
)

// notFoundChecker 检查 err 是否为对应后端的 "记录不存在" 错误。
type notFoundChecker func(error) bool

var notFoundCheckers []notFoundChecker

// registerNotFoundChecker 注册一个后端错误检查器。
// checker_sql.go 在 init() 中注册 SQL 后端的检查器。
func registerNotFoundChecker(fn notFoundChecker) {
	notFoundCheckers = append(notFoundCheckers, fn)
}

// WrapDBErr 将底层数据库错误转换为统一错误。
// 通过已注册的后端检查器识别 ErrNotFound，其他错误原样返回。
func WrapDBErr(err error) error {
	if err == nil {
		return nil
	}
	for _, check := range notFoundCheckers {
		if check(err) {
			return ErrNotFound
		}
	}
	return err
}
