package repository

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

func init() {
	registerNotFoundChecker(func(err error) bool {
		return errors.Is(err, gorm.ErrRecordNotFound)
	})
	// 唯一约束冲突：三家后端的错误形态各不相同，统一在这里识别，
	// 让 service 只判断 repository.ErrDuplicate，不必再字符串匹配。
	registerDuplicateChecker(func(err error) bool {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return true
		}
		msg := strings.ToLower(err.Error())
		return strings.Contains(msg, "unique constraint") || // SQLite / PostgreSQL
			strings.Contains(msg, "duplicate entry") || // MySQL
			strings.Contains(msg, "duplicate key") // PostgreSQL
	})
}
