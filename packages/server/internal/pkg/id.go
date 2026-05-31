package pkg

import (
	"github.com/oklog/ulid/v2"
)

// NewID 生成一个 ULID 格式的唯一标识符。
func NewID() string {
	return ulid.Make().String()
}
