package server

import (
	"fmt"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"time"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func seedMockData(store db.Store) {
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

func seedSettings(store db.Store) {
	now := time.Now()
	for key, val := range model.DefaultSettings {
		setting := &model.Setting{
			Key:       key,
			Value:     val,
			UpdatedAt: now,
		}
		if store.SQL() != nil {
			store.SQL().DB().Where("key = ?", key).FirstOrCreate(setting)
		}
	}
	logrus.Info(fmt.Sprintf("default settings seeded: %d keys", len(model.DefaultSettings)))
}

func digestBody(body string, maxLen int) string {
	if utf8.RuneCountInString(body) <= maxLen {
		return body
	}
	runes := []rune(body)
	return string(runes[:maxLen])
}

// reseedPostsAndContents clears posts/contents and re-seeds with correct relationships.
func reseedPostsAndContents(store db.Store) {
	if store.SQL() == nil {
		return
	}
	db := store.SQL().DB()

	// 查出所有用户
	var users []model.User
	db.Find(&users)
	if len(users) == 0 {
		logrus.Warn("reseedPostsAndContents: no users found, skip")
		return
	}

	// 查出分类，没有就创建
	categories := []struct{ name, desc string }{
		{"技术", "编程、架构、工具等技术话题"},
		{"产品", "产品设计、用户体验与需求讨论"},
		{"运维", "部署、CI/CD、云原生与监控"},
		{"闲聊", "日常分享与生活随想"},
	}
	catMap := map[string]string{} // name -> id
	for _, c := range categories {
		var cat model.Category
		if err := db.Where("name = ?", c.name).First(&cat).Error; err != nil {
			cat = model.Category{
				ID:          pkg.NewID(),
				Name:        c.name,
				Description: c.desc,
				CreatedAt:   time.Now(),
			}
			db.Create(&cat)
		}
		catMap[c.name] = cat.ID
	}

	// 清空旧数据（contents 先删，因为 posts 有 content_id 约束）
	db.Unscoped().Where("1 = 1").Delete(&model.Content{})
	db.Unscoped().Where("1 = 1").Delete(&model.Post{})
	logrus.Info("reseedPostsAndContents: cleared posts and contents")

	userID := func(i int) string { return users[i%len(users)].ID }

	type postSeed struct {
		title    string
		body     string
		category string
		authorIdx int
		comments []struct {
			body      string
			authorIdx int
			replies   []struct {
				body      string
				authorIdx int
			}
		}
	}

	seeds := []postSeed{
		{
			title:    "Go 1.23 有哪些值得关注的新特性？",
			body:     "Go 1.23 带来了 range-over-func、改进的工具链以及若干标准库增强。其中 range-over-func 让自定义迭代器成为可能，对于需要懒求值的场景非常实用。大家在实际项目中有用上这些特性吗？",
			category: "技术",
			authorIdx: 0,
			comments: []struct {
				body      string
				authorIdx int
				replies   []struct {
					body      string
					authorIdx int
				}
			}{
				{
					body:      "range-over-func 我已经在内部工具里用上了，配合 iter.Seq 写起来非常优雅，比之前手写 channel 省了很多样板代码。",
					authorIdx: 1,
					replies: []struct {
						body      string
						authorIdx int
					}{
						{"同感，不过要注意 break/continue 语义和普通 for 有点不同，刚开始容易踩坑。", 2},
					},
				},
				{
					body:      "工具链改进感受最深，`go build` 在增量编译上快了不少，大项目差距明显。",
					authorIdx: 3,
					replies:   nil,
				},
			},
		},
		{
			title:    "如何设计一个合理的 REST API 错误码体系？",
			body:     "最近在重构公司的 API，发现错误码混乱是个大问题：有的用 HTTP 状态码，有的自定义业务码，前端处理起来非常痛苦。想听听大家的设计思路，尤其是业务错误码和 HTTP 状态码如何配合。",
			category: "技术",
			authorIdx: 1,
			comments: []struct {
				body      string
				authorIdx int
				replies   []struct {
					body      string
					authorIdx int
				}
			}{
				{
					body:      "我们的做法是：HTTP 状态码只用 200/400/401/403/404/500 这几个，业务细节放在 body 的 code 字段里，文档里维护一张业务码表。",
					authorIdx: 2,
					replies: []struct {
						body      string
						authorIdx int
					}{
						{"这个方案清晰，但业务码表一定要版本化管理，否则迭代几轮就乱了。", 0},
						{"同意，我们还在 code 里加了前缀区分模块，比如 USER_001、ORDER_002 这样。", 4},
					},
				},
				{
					body:      "RFC 7807（Problem Details）值得参考，标准化了错误响应结构，很多框架已经内置支持了。",
					authorIdx: 3,
					replies:   nil,
				},
			},
		},
		{
			title:    "用户体验中的「渐进式披露」原则",
			body:     "渐进式披露（Progressive Disclosure）是指把复杂信息分层展示，让用户按需深入。在表单设计、设置页面、文档导航中都有广泛应用。分享几个我观察到的反例：把所有选项平铺在一屏、用折叠面板藏掉核心操作、以及过度使用 tooltip 代替说明文字。",
			category: "产品",
			authorIdx: 2,
			comments: []struct {
				body      string
				authorIdx int
				replies   []struct {
					body      string
					authorIdx int
				}
			}{
				{
					body:      "折叠面板藏核心操作这个真的是重灾区，很多后台系统的「高级设置」里藏着每天都要用的东西。",
					authorIdx: 0,
					replies: []struct {
						body      string
						authorIdx int
					}{
						{"哈哈，我们内部系统也有这问题，后来直接把高频操作提到了顶层，留访率直接提升了。", 1},
					},
				},
			},
		},
		{
			title:    "K8s 滚动更新策略踩坑记录",
			body:     "上周生产环境做滚动更新时遇到了一个问题：新版本 Pod 启动后 readinessProbe 还没就绪，但旧版本 Pod 已经被终止，导致短暂的 503。排查后发现是 minReadySeconds 没有配置，加上之后问题消失。记录一下，希望对大家有帮助。",
			category: "运维",
			authorIdx: 4,
			comments: []struct {
				body      string
				authorIdx int
				replies   []struct {
					body      string
					authorIdx int
				}
			}{
				{
					body:      "minReadySeconds 这个参数确实容易被忽略，我们还踩过 terminationGracePeriodSeconds 设置太短导致连接被强制断开的坑。",
					authorIdx: 3,
					replies: []struct {
						body      string
						authorIdx int
					}{
						{"这两个参数建议写进 checklist，每次发布前过一遍。", 1},
					},
				},
				{
					body:      "除了这些，PodDisruptionBudget 也很重要，能保证滚动更新期间始终有足够的健康实例。",
					authorIdx: 0,
					replies:   nil,
				},
			},
		},
		{
			title:    "最近发现一个好用的终端工具：Warp",
			body:     "Warp 是一个基于 Rust 的现代终端，支持块级编辑、命令补全、内置 AI 等功能。用了两周感受是：多行命令编辑体验比传统终端好很多，历史搜索也更直观。目前 Mac 免费，Linux 版本也出了。大家有在用吗？",
			category: "闲聊",
			authorIdx: 1,
			comments: []struct {
				body      string
				authorIdx int
				replies   []struct {
					body      string
					authorIdx int
				}
			}{
				{
					body:      "用过，块级编辑确实很爽，但 AI 功能我一般关掉，怕命令历史被上传。",
					authorIdx: 2,
					replies: []struct {
						body      string
						authorIdx int
					}{
						{"隐私这块官方文档有说明，AI 功能是可选的，命令历史默认不上传。", 4},
					},
				},
				{
					body:      "我还是用 iTerm2 + zsh，习惯了就换不动了，哈哈。",
					authorIdx: 0,
					replies:   nil,
				},
			},
		},
	}

	now := time.Now()
	postCount, contentCount := 0, 0

	for i, s := range seeds {
		postContentID := pkg.NewID()
		postID := pkg.NewID()
		catID := catMap[s.category]
		createdAt := now.Add(-time.Duration(len(seeds)-i) * 6 * time.Hour)

		err := db.Transaction(func(tx *gorm.DB) error {
			// 帖子主内容（RootID = 自身 ID）
			postContent := &model.Content{
				ID:        postContentID,
				AuthorID:  userID(s.authorIdx),
				Body:      s.body,
				RootID:    postContentID,
				ParentID:  "",
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			}
			if err := tx.Create(postContent).Error; err != nil {
				return err
			}

			commentCount := int64(0)
			// 一级评论
			for j, c := range s.comments {
				commentID := pkg.NewID()
				commentAt := createdAt.Add(time.Duration(j+1) * 30 * time.Minute)
				comment := &model.Content{
					ID:        commentID,
					AuthorID:  userID(c.authorIdx),
					Body:      c.body,
					RootID:    postContentID,
					ParentID:  postContentID,
					CreatedAt: commentAt,
					UpdatedAt: commentAt,
				}
				if err := tx.Create(comment).Error; err != nil {
					return err
				}
				commentCount++
				contentCount++

				// 二级回复
				for k, r := range c.replies {
					replyAt := commentAt.Add(time.Duration(k+1) * 10 * time.Minute)
					reply := &model.Content{
						ID:        pkg.NewID(),
						AuthorID:  userID(r.authorIdx),
						Body:      r.body,
						RootID:    postContentID,
						ParentID:  commentID,
						CreatedAt: replyAt,
						UpdatedAt: replyAt,
					}
					if err := tx.Create(reply).Error; err != nil {
						return err
					}
					commentCount++
					contentCount++
				}
			}

			post := &model.Post{
				ID:            postID,
				Title:         s.title,
				ContentID:     postContentID,
				ContentDigest: digestBody(s.body, 200),
				AuthorID:      userID(s.authorIdx),
				CategoryID:    catID,
				Status:        "approved",
				CommentCount:  commentCount,
				CreatedAt:     createdAt,
				UpdatedAt:     createdAt,
			}
			return tx.Create(post).Error
		})

		if err != nil {
			logrus.Errorf("reseedPostsAndContents: post %q failed: %v", s.title, err)
			continue
		}
		postCount++
		contentCount++ // post body content
	}

	logrus.Infof("reseedPostsAndContents: seeded %d posts, %d contents", postCount, contentCount)
}

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

func ensureSettings(store db.Store) {
	if store.SQL() == nil {
		return
	}
	var count int64
	store.SQL().DB().Model(&model.Setting{}).Count(&count)
	if count == 0 {
		seedSettings(store)
	}
}
