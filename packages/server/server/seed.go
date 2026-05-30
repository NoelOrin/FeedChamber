package server

import (
	"fmt"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/repository"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

func seedMockData(store db.Store) {
	// ── Users ──────────────────────────────────────────────────
	users := []struct {
		username string
		email    string
		bio      string
	}{
		{"alice", "alice@example.com", "热爱编程与开源社区"},
		{"bob", "bob@example.com", "全栈开发者，喜欢分享技术"},
		{"charlie", "charlie@example.com", "产品设计师，关注用户体验"},
		{"diana", "diana@example.com", "后端工程师，Go 语言爱好者"},
		{"eve", "eve@example.com", "DevOps 工程师，云原生布道者"},
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	userIDs := make([]string, len(users))

	for i, u := range users {
		id := uuid.NewString()
		userIDs[i] = id
		user := &model.User{
			ID:        id,
			UUID:      uuid.NewString(),
			Username:  u.username,
			Email:     u.email,
			Password:  string(hash),
			Bio:       u.bio,
			Role:      "user",
			CreatedAt: time.Now().Add(-time.Duration(len(users)-i) * 24 * time.Hour),
			UpdatedAt: time.Now(),
		}
		if store.SQL() != nil {
			store.SQL().DB().Where("username = ?", u.username).FirstOrCreate(user)
		}
	}

	// ── Categories ─────────────────────────────────────────────
	categories := []struct {
		name string
		desc string
	}{
		{"技术讨论", "编程语言、框架、工具等技术话题"},
		{"项目分享", "分享你的开源项目和个人作品"},
		{"问答求助", "遇到问题来这里寻求帮助"},
		{"公告通知", "社区官方公告和通知"},
		{"闲聊灌水", "轻松聊天，不限话题"},
	}
	catIDs := make([]string, len(categories))
	for i, c := range categories {
		id := uuid.NewString()
		catIDs[i] = id
		cat := &model.Category{ID: id, Name: c.name, Description: c.desc, CreatedAt: time.Now()}
		if store.SQL() != nil {
			store.SQL().DB().Where("name = ?", c.name).FirstOrCreate(cat)
		}
	}

	// ── Posts + Contents ────────────────────────────────────────
	type mockPost struct {
		title    string
		body     string
		userIdx  int
		catIdx   int
		pinned   bool
		views    int64
		comments int
	}
	posts := []mockPost{
		{"Go 语言泛型实践指南", "本文介绍 Go 1.18 引入的泛型特性，包括类型参数、类型约束、泛型函数和泛型类型的使用方法。通过实际案例展示如何在日常开发中合理使用泛型来提高代码复用性。\n\n泛型的核心优势在于可以在编译期保证类型安全的同时实现代码复用。相比 interface{} 的运行时类型断言，泛型提供了更好的类型检查和性能。", 0, 0, true, 128, 3},
		{"TanStack Router 使用心得", "最近把项目迁移到了 TanStack Router，记录一下使用感受和踩坑经历。\n\n主要优势：类型安全的路由、嵌套布局、搜索参数验证。相比 react-router-dom，TanStack Router 的类型推导确实更强大。", 1, 0, false, 96, 2},
		{"开源项目：高性能 JSON 解析器", "我用 Rust 写了一个高性能 JSON 解析器，benchmark 显示比 serde_json 快 30%。项目地址在 GitHub 上，欢迎 star 和 PR。\n\n核心技术：SIMD 指令加速、零拷贝解析、lazy number parsing。", 2, 1, false, 256, 2},
		{"如何设计一个好的 API？", "分享一些 RESTful API 设计的最佳实践，包括命名规范、版本控制、错误处理、分页策略等。\n\n好的 API 应该是自描述的、一致的、向后兼容的。", 3, 0, false, 180, 1},
		{"社区规范 v1.0 发布", "经过社区讨论，我们制定了第一版社区行为规范。请大家在发帖和评论时遵守以下规则：\n\n1. 尊重他人，禁止人身攻击\n2. 禁止发布广告和垃圾内容\n3. 技术讨论请提供可复现的代码\n4. 转载请注明出处", 0, 3, true, 512, 0},
		{"Docker 容器网络问题排查", "最近在生产环境遇到容器间通信失败的问题，排查过程记录如下。\n\n症状：同一个 pod 内的容器无法通过 localhost 互相访问。最终发现是 iptables 规则被意外覆盖。", 4, 2, false, 72, 1},
	}

	postIDs := make([]string, len(posts))
	contentIDs := make([]string, len(posts))

	for i, p := range posts {
		now := time.Now().Add(-time.Duration(len(posts)-i) * 6 * time.Hour)
		contentID := uuid.NewString()
		postID := uuid.NewString()
		contentIDs[i] = contentID
		postIDs[i] = postID

		content := &model.Content{
			ID:        contentID,
			AuthorID:  userIDs[p.userIdx],
			Body:      p.body,
			RootID:    contentID,
			ParentID:  "",
			CreatedAt: now,
			UpdatedAt: now,
		}
		post := &model.Post{
			ID:            postID,
			Title:         p.title,
			ContentID:     contentID,
			ContentDigest: repository.DigestBody(p.body, 200),
			AuthorID:      userIDs[p.userIdx],
			CategoryID:    catIDs[p.catIdx],
			Status:        "approved",
			Pinned:        p.pinned,
			ViewCount:     p.views,
			CommentCount:  int64(p.comments),
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		if store.SQL() != nil {
			tx := store.SQL().DB().Begin()
			tx.Create(content)
			tx.Create(post)
			tx.Commit()
		}
	}

	// ── Comments / Replies ──────────────────────────────────────
	type mockComment struct {
		postIdx  int
		userIdx  int
		body     string
		parentID string // "" = top-level, non-empty = reply to that contentID
	}

	comments := []mockComment{
		// Post 0: Go 泛型
		{0, 1, "泛型确实很好用，但在实际项目中还是要注意不要过度抽象，保持代码可读性更重要。", ""},
		{0, 2, "同意楼上，泛型适合用在工具函数和数据结构上，业务逻辑还是用具体类型更清晰。", ""},
		{0, 3, "请问泛型对编译速度的影响大吗？我们的项目比较大，担心会拖慢构建。", ""},
		// Post 1: TanStack Router
		{1, 3, "TanStack Router 的搜索参数验证是杀手级功能，再也不用担心 URL 参数类型不匹配了。", ""},
		{1, 4, "迁移成本高吗？现有的路由配置需要全部重写吗？", ""},
		// Post 2: JSON 解析器
		{2, 0, "看起来很不错！SIMD 指令是用的 AVX2 还是 SSE？", ""},
		{2, 3, "能支持流式解析吗？对于大文件场景流式解析很重要。", ""},
		// Post 3: API 设计
		{3, 4, "补充一点：API 的错误响应格式也要统一，推荐使用 RFC 7807 问题详情标准。", ""},
		// Post 5: Docker 网络
		{5, 0, "遇到过类似问题，最终用 Calico 替换了 flannel 解决的。你们用的什么 CNI？", ""},
	}

	commentIDs := make([]string, len(comments))
	for i, c := range comments {
		id := uuid.NewString()
		commentIDs[i] = id
		now := time.Now().Add(-time.Duration(len(comments)-i) * 30 * time.Minute)
		content := &model.Content{
			ID:        id,
			AuthorID:  userIDs[c.userIdx],
			Body:      c.body,
			RootID:    contentIDs[c.postIdx],
			ParentID:  c.parentID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if store.SQL() != nil {
			store.SQL().DB().Create(content)
		}
	}

	// ── Replies to comments ─────────────────────────────────────
	replies := []mockComment{
		// Reply to comment 0 (under post 0)
		{0, 0, "确实，泛型的最佳实践是「适度使用」，而不是处处泛型化。", commentIDs[0]},
		// Reply to comment 2 (under post 0)
		{0, 1, "编译速度影响很小，Go 团队在设计时就考虑了这一点，泛型是通过 GCShape stenciling 实现的。", commentIDs[2]},
		// Reply to comment 4 (under post 1)
		{1, 1, "迁移成本取决于你现有的路由复杂度。如果是简单的平面路由，基本可以自动迁移。嵌套路由需要手动调整。", commentIDs[4]},
		// Reply to comment 6 (under post 2)
		{2, 2, "目前支持 SSE2，AVX2 支持正在开发中，预计下个版本发布。", commentIDs[6]},
	}

	for i, r := range replies {
		now := time.Now().Add(-time.Duration(len(replies)-i) * 15 * time.Minute)
		content := &model.Content{
			ID:        uuid.NewString(),
			AuthorID:  userIDs[r.userIdx],
			Body:      r.body,
			RootID:    contentIDs[r.postIdx],
			ParentID:  r.parentID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if store.SQL() != nil {
			store.SQL().DB().Create(content)
		}
	}

	// ── Content Likes ───────────────────────────────────────────
	likePairs := []struct{ contentIdx, userIdx int }{
		{0, 1}, {0, 2}, {0, 3}, // post 0 content liked by 3 users
		{1, 0}, {1, 3},         // post 1 content
		{2, 0}, {2, 1}, {2, 3}, {2, 4}, // post 2 content
	}
	for _, lp := range likePairs {
		like := &model.ContentLike{
			ID:        uuid.NewString(),
			ContentID: contentIDs[lp.contentIdx],
			UserID:    userIDs[lp.userIdx],
			CreatedAt: time.Now(),
		}
		if store.SQL() != nil {
			store.SQL().DB().Create(like)
		}
	}

	// Update like counts on posts
	for i := range posts {
		count := int64(0)
		for _, l := range likePairs {
			if l.contentIdx == i {
				count++
			}
		}
		if store.SQL() != nil && count > 0 {
			store.SQL().DB().Model(&model.Post{}).Where("id = ?", postIDs[i]).UpdateColumn("like_count", count)
		}
	}

	logrus.Info(fmt.Sprintf("mock data seeded: %d users, %d categories, %d posts, %d comments, %d replies",
		len(users), len(categories), len(posts), len(comments), len(replies)))
}

// ensurePostExists checks if any posts exist; if not, seeds mock data.
func ensureMockData(store db.Store) {
	if store.SQL() == nil {
		return
	}
	var count int64
	store.SQL().DB().Model(&model.Post{}).Count(&count)
	if count == 0 {
		seedMockData(store)
	}
}
