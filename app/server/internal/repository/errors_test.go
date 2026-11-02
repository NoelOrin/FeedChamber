package repository

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 唯一冲突必须被统一翻译成 ErrDuplicate：三家后端文案不同，
// service 只该判断 errors.Is(err, ErrDuplicate)，不该再字符串匹配。
func TestWrapDBErrMapsUniqueViolations(t *testing.T) {
	cases := map[string]bool{
		"UNIQUE constraint failed: custom_pages.slug":    true, // SQLite
		"Error 1062: Duplicate entry 'x' for key 'slug'": true, // MySQL
		"duplicate key value violates unique constraint": true, // PostgreSQL
		"connection refused":                             false,
		"record not found":                               false,
	}

	for msg, wantDuplicate := range cases {
		err := WrapDBErr(fmt.Errorf("%s", msg))
		if wantDuplicate {
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrDuplicate), "msg=%q 应翻成 ErrDuplicate，得到 %v", msg, err)
			continue
		}
		assert.False(t, errors.Is(err, ErrDuplicate), "msg=%q 不应翻成 ErrDuplicate", msg)
	}
}

func TestWrapDBErrMapsGormDuplicatedKey(t *testing.T) {
	err := WrapDBErr(gorm.ErrDuplicatedKey)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDuplicate))
}
