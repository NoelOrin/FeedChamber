package server

import (
	"context"
	"errors"
	"fmt"
	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"
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
	credRepo := repository.NewCredentialRepository(store)

	for i, u := range users {
		user := &model.User{
			ID:          pkg.NewID(),
			Username:    u.username,
			DisplayName: u.username,
			Email:       u.email,
			Avatar:      u.avatar,
			Bio:         u.bio,
			Role:        "user",
			CreatedAt:   time.Now().Add(-time.Duration(len(users)-i) * 24 * time.Hour),
			UpdatedAt:   time.Now(),
		}
		store.SQL().DB().Where("username = ?", u.username).FirstOrCreate(user)
		// Save credential for this user
		cred := &model.Credential{
			UserID:       user.ID,
			PasswordHash: string(hash),
		}
		if err := credRepo.Upsert(context.Background(), cred); err != nil {
			logrus.Errorf("seed mock credential for %s: %v", u.username, err)
		}
	}

	logrus.Info(fmt.Sprintf("mock data seeded: %d users", len(users)))
}

// syncDefaultSettings ensures every key in DefaultSettings exists in the database.
// It is idempotent — safe to call every boot. Missing keys are inserted; existing keys
// are left untouched (respecting admin customizations).
func syncDefaultSettings(store db.Store) {
	now := time.Now()

	for key, val := range model.DefaultSettings {
		store.SQL().DB().Where("key = ?", key).FirstOrCreate(&model.Setting{Key: key, Value: val, UpdatedAt: now})
	}
	logrus.Infof("syncDefaultSettings: %d keys ensured", len(model.DefaultSettings))
}

// syncModerationConfigs ensures every key in DefaultModerationConfigs exists.
// Idempotent — safe to call every boot; existing values are left untouched.
func syncModerationConfigs(store db.Store) {
	for key, val := range model.DefaultModerationConfigs {
		store.SQL().DB().Where("key = ?", key).FirstOrCreate(&model.ModerationConfig{Key: key, Value: val})
	}
	logrus.Infof("syncModerationConfigs: %d keys ensured", len(model.DefaultModerationConfigs))
}

func digestBody(body string, maxLen int) string {
	if utf8.RuneCountInString(body) <= maxLen {
		return body
	}
	runes := []rune(body)
	return string(runes[:maxLen])
}

// ── Shared seed definitions ───────────────────────────────────────────────────

type replySeed struct {
	body      string
	authorIdx int
}

type commentSeed struct {
	body      string
	authorIdx int
	replies   []replySeed
}

type postSeed struct {
	title     string
	body      string
	spaceSlug string
	authorIdx int
	tags      []string
	comments  []commentSeed
}

// 版面与标签数据的定义见 seed_spaces.go / seed_tags.go。

var postSeeds = []postSeed{
	{
		title:     "Go 1.23 有哪些值得关注的新特性？",
		body:      "Go 1.23 带来了 range-over-func、改进的工具链以及若干标准库增强。其中 range-over-func 让自定义迭代器成为可能，对于需要懒求值的场景非常实用。大家在实际项目中有用上这些特性吗？",
		spaceSlug: "tech",
		authorIdx: 0,
		tags:      []string{"Go", "开源"},
		comments: []commentSeed{
			{
				body:      "range-over-func 我已经在内部工具里用上了，配合 iter.Seq 写起来非常优雅，比之前手写 channel 省了很多样板代码。",
				authorIdx: 1,
				replies: []replySeed{
					{"同感，不过要注意 break/continue 语义和普通 for 有点不同，刚开始容易踩坑。", 2},
				},
			},
			{
				body:      "工具链改进感受最深，`go build` 在增量编译上快了不少，大项目差距明显。",
				authorIdx: 3,
			},
		},
	},
	{
		title:     "如何设计一个合理的 REST API 错误码体系？",
		body:      "最近在重构公司的 API，发现错误码混乱是个大问题：有的用 HTTP 状态码，有的自定义业务码，前端处理起来非常痛苦。想听听大家的设计思路，尤其是业务错误码和 HTTP 状态码如何配合。",
		spaceSlug: "tech",
		authorIdx: 1,
		tags:      []string{"架构设计", "性能优化"},
		comments: []commentSeed{
			{
				body:      "我们的做法是：HTTP 状态码只用 200/400/401/403/404/500 这几个，业务细节放在 body 的 code 字段里，文档里维护一张业务码表。",
				authorIdx: 2,
				replies: []replySeed{
					{"这个方案清晰，但业务码表一定要版本化管理，否则迭代几轮就乱了。", 0},
					{"同意，我们还在 code 里加了前缀区分模块，比如 USER_001、ORDER_002 这样。", 4},
				},
			},
			{
				body:      "RFC 7807（Problem Details）值得参考，标准化了错误响应结构，很多框架已经内置支持了。",
				authorIdx: 3,
			},
		},
	},
	{
		title:     "用户体验中的「渐进式披露」原则",
		body:      "渐进式披露（Progressive Disclosure）是指把复杂信息分层展示，让用户按需深入。在表单设计、设置页面、文档导航中都有广泛应用。分享几个我观察到的反例：把所有选项平铺在一屏、用折叠面板藏掉核心操作、以及过度使用 tooltip 代替说明文字。",
		spaceSlug: "product",
		authorIdx: 2,
		tags:      []string{"产品设计", "用户体验"},
		comments: []commentSeed{
			{
				body:      "折叠面板藏核心操作这个真的是重灾区，很多后台系统的「高级设置」里藏着每天都要用的东西。",
				authorIdx: 0,
				replies: []replySeed{
					{"哈哈，我们内部系统也有这问题，后来直接把高频操作提到了顶层，留访率直接提升了。", 1},
				},
			},
		},
	},
	{
		title:     "K8s 滚动更新策略踩坑记录",
		body:      "上周生产环境做滚动更新时遇到了一个问题：新版本 Pod 启动后 readinessProbe 还没就绪，但旧版本 Pod 已经被终止，导致短暂的 503。排查后发现是 minReadySeconds 没有配置，加上之后问题消失。记录一下，希望对大家有帮助。",
		spaceSlug: "ops",
		authorIdx: 4,
		tags:      []string{"Kubernetes", "云原生"},
		comments: []commentSeed{
			{
				body:      "minReadySeconds 这个参数确实容易被忽略，我们还踩过 terminationGracePeriodSeconds 设置太短导致连接被强制断开的坑。",
				authorIdx: 3,
				replies: []replySeed{
					{"这两个参数建议写进 checklist，每次发布前过一遍。", 1},
				},
			},
			{
				body:      "除了这些，PodDisruptionBudget 也很重要，能保证滚动更新期间始终有足够的健康实例。",
				authorIdx: 0,
			},
		},
	},
	{
		title:     "最近发现一个好用的终端工具：Warp",
		body:      "Warp 是一个基于 Rust 的现代终端，支持块级编辑、命令补全、内置 AI 等功能。用了两周感受是：多行命令编辑体验比传统终端好很多，历史搜索也更直观。目前 Mac 免费，Linux 版本也出了。大家有在用吗？",
		spaceSlug: "chat",
		authorIdx: 1,
		tags:      []string{"效率工具", "Linux"},
		comments: []commentSeed{
			{
				body:      "用过，块级编辑确实很爽，但 AI 功能我一般关掉，怕命令历史被上传。",
				authorIdx: 2,
				replies: []replySeed{
					{"隐私这块官方文档有说明，AI 功能是可选的，命令历史默认不上传。", 4},
				},
			},
			{
				body:      "我还是用 iTerm2 + zsh，习惯了就换不动了，哈哈。",
				authorIdx: 0,
			},
		},
	},
}

// reseedPostsAndContents clears posts/contents and re-seeds with correct relationships.
// It is destructive (clears existing posts/contents), so once the one-time seed
// has completed it runs only when the posts table is empty (recovery).
func reseedPostsAndContents(store db.Store, alreadySeeded bool) {
	db := store.SQL().DB()

	if alreadySeeded {
		var postCount int64
		db.Model(&model.Post{}).Count(&postCount)
		if postCount > 0 {
			logrus.Info("reseedPostsAndContents: already seeded, skip")
			return
		}
	}

	// 查出所有用户
	var users []model.User
	db.Find(&users)
	if len(users) == 0 {
		logrus.Warn("reseedPostsAndContents: no users found, skip")
		return
	}

	// 查出或创建 Space（幂等，已存在的版面不会被动）
	spaceBySlug := ensureSeedSpaces(context.Background(), repository.NewSpaceRepository(store))

	// 清空旧数据（contents 先删，因为 posts 有 content_id 约束）
	db.Unscoped().Where("1 = 1").Delete(&model.Content{})
	db.Unscoped().Where("1 = 1").Delete(&model.Post{})
	// post_tags 没有级联删除，帖子清空后残留的关联会变成孤儿行
	db.Unscoped().Where("1 = 1").Delete(&model.PostTag{})
	logrus.Info("reseedPostsAndContents: cleared posts, contents and post_tags")

	userID := func(i int) string { return users[i%len(users)].ID }

	seeds := postSeeds

	now := time.Now()
	postCount, contentCount, tagLinkCount := 0, 0, 0
	tagRepo := repository.NewTagRepository(store)

	for i, s := range seeds {
		postContentID := pkg.NewID()
		postID := pkg.NewID()
		createdAt := now.Add(-time.Duration(len(seeds)-i) * 6 * time.Hour)

		// 标签库由 seedBaselineTags 提供；这里按名称取 ID，缺失时补建，
		// 避免标签表被单独清空后帖子挂不上标签。
		tagIDs := make([]string, 0, len(s.tags))
		for _, name := range s.tags {
			tag, err := tagRepo.GetByName(context.Background(), name)
			if err != nil {
				if !errors.Is(err, repository.ErrNotFound) {
					logrus.Warnf("reseedPostsAndContents: lookup tag %q: %v", name, err)
					continue
				}
				tag = &model.Tag{ID: pkg.NewID(), Name: name, CreatedAt: createdAt}
				if err := tagRepo.Create(context.Background(), tag); err != nil {
					logrus.Warnf("reseedPostsAndContents: create tag %q: %v", name, err)
					continue
				}
			}
			tagIDs = append(tagIDs, tag.ID)
		}

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
				SpaceID:       spaceBySlug[s.spaceSlug],
				Status:        "approved",
				CommentCount:  commentCount,
				CreatedAt:     createdAt,
				UpdatedAt:     createdAt,
			}
			if err := tx.Create(post).Error; err != nil {
				return err
			}
			for _, tagID := range tagIDs {
				if err := tx.Create(&model.PostTag{PostID: postID, TagID: tagID}).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			logrus.Errorf("reseedPostsAndContents: post %q failed: %v", s.title, err)
			continue
		}
		postCount++
		contentCount++ // post body content
		tagLinkCount += len(tagIDs)
	}

	// 种子帖绕过 service 直接落库，版面 post_count 需要重算：否则版面页会显示
	// 「有帖子但计数为 0」。顺带修正恢复场景下清空帖子后遗留的旧计数。
	if err := db.Exec(`UPDATE spaces SET post_count = (SELECT COUNT(*) FROM posts WHERE posts.space_id = spaces.id AND posts.deleted_at IS NULL)`).Error; err != nil {
		logrus.Warnf("reseedPostsAndContents: refresh space post_count: %v", err)
	}

	logrus.Infof("reseedPostsAndContents: seeded %d posts, %d contents, %d post-tag links", postCount, contentCount, tagLinkCount)
}

func ensureMockData(store db.Store) {
	var count int64
	store.SQL().DB().Model(&model.User{}).Count(&count)
	if count == 0 {
		seedMockData(store)
	}
}

// backfillDisplayName sets display_name = username for any user whose
// display_name is still empty (pre-existing rows created before the
// username/display_name split). Idempotent — safe to run every boot.
func backfillDisplayName(store db.Store) {
	if err := store.SQL().DB().
		Model(&model.User{}).
		Where("display_name IS NULL OR display_name = ''").
		Update("display_name", gorm.Expr("username")).Error; err != nil {
		logrus.Warnf("backfill display_name: %v", err)
	}
}
