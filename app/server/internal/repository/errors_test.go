package repository

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestWrapDBErr_Nil(t *testing.T) {
	assert.Nil(t, WrapDBErr(nil))
}

func TestWrapDBErr_GormRecordNotFound(t *testing.T) {
	err := WrapDBErr(gorm.ErrRecordNotFound)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestWrapDBErr_OtherError(t *testing.T) {
	original := errors.New("some db error")
	err := WrapDBErr(original)
	assert.Equal(t, original, err)
	assert.False(t, errors.Is(err, ErrNotFound))
}
