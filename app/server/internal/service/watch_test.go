package service

import (
	"testing"

	"feedchamber/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestValidWatchTargetAndLevel(t *testing.T) {
	assert.True(t, validTargetType(model.WatchTargetPost))
	assert.True(t, validTargetType(model.WatchTargetTag))
	// 版面关注已于 2026-10-28 移除（前台无入口、后端不再接受）。这条断言是收口的钉子：
	// 谁想把 "space" 加回 validTargetType，必须先回答「前台入口在哪」。
	assert.False(t, validTargetType("space"))
	assert.False(t, validTargetType("user"))

	assert.True(t, validLevel(model.WatchLevelWatching))
	assert.True(t, validLevel(model.WatchLevelTracking))
	assert.True(t, validLevel(model.WatchLevelMuted))
	assert.False(t, validLevel("loud"))
}
