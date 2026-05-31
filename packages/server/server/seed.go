package server

import (
	"fmt"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

func seedMockData(store db.Store) {
	// ── Users ──────────────────────────────────────────────────
	users := []struct {
		username string
		email    string
		bio      string
		avatar   string
	}{
		{"alice", "alice@example.com", "热爱编程与开源社区", ""},
		{"bob", "bob@example.com", "全栈开发者，喜欢分享技术", ""},
		{"charlie", "charlie@example.com", "产品设计师，关注用户体验", ""},
		{"diana", "diana@example.com", "后端工程师，Go 语言爱好者", ""},
		{"eve", "eve@example.com", "DevOps 工程师，云原生布道者", ""},
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)

	for i, u := range users {
		user := &model.User{
			ID:        pkg.NewID(),
			UUID:      pkg.NewID(),
			Username:  u.username,
			Email:     u.email,
			Password:  string(hash),
			Avatar:    u.avatar,
			Bio:       u.bio,
			Role:      "user",
			CreatedAt: time.Now().Add(-time.Duration(len(users)-i) * 24 * time.Hour),
			UpdatedAt: time.Now(),
		}
		if store.SQL() != nil {
			store.SQL().DB().Where("username = ?", u.username).FirstOrCreate(user)
		}
	}

	logrus.Info(fmt.Sprintf("mock data seeded: %d users", len(users)))
}

// ensurePostExists checks if any users exist; if not, seeds mock data.
func ensureMockData(store db.Store) {
	if store.SQL() == nil {
		return
	}
	var count int64
	store.SQL().DB().Model(&model.User{}).Count(&count)
	if count == 0 {
		seedMockData(store)
	}
}
