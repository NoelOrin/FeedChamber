package repository

import (
	"errors"

	"gorm.io/gorm"
)

func init() {
	registerNotFoundChecker(func(err error) bool {
		return errors.Is(err, gorm.ErrRecordNotFound)
	})
}
