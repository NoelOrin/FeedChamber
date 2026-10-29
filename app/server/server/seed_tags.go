package server

import (
	"context"
	"errors"
	"time"

	"orinforum/internal/db"
	"orinforum/internal/model"
	"orinforum/internal/pkg"
	"orinforum/internal/repository"

	"github.com/sirupsen/logrus"
)

// seedTagNames 是标签库的基础集合：覆盖常见技术栈与社区话题，让新站在
// 筛选器、SEO sitemap-tags.xml 和后台标签页里有可用内容。标签本身没有
// 排序字段，列表按名称升序展示。
var seedTagNames = []string{
	"Go", "Rust", "TypeScript", "React", "Vue", "Node.js", "Python", "Java",
	"Kubernetes", "Docker", "Linux", "云原生",
	"PostgreSQL", "MySQL", "Redis", "NATS", "数据库",
	"架构设计", "微服务", "性能优化", "前端工程化",
	"开源", "效率工具", "AI",
	"产品设计", "用户体验", "职场", "新手提问",
}

// seedBaselineTags ensures the baseline tag library exists.
//
// 只在首次启动（!alreadySeeded）或标签表为空时执行：按名称补齐缺失行，
// 已存在的标签不动（管理员改过的名字保留原样），也不会复活被删除的标签。
func seedBaselineTags(store db.Store, alreadySeeded bool) {
	ctx := context.Background()
	repo := repository.NewTagRepository(store)

	if alreadySeeded {
		count, err := repo.Count(ctx)
		if err != nil {
			logrus.Warnf("seedBaselineTags: count tags: %v", err)
			return
		}
		if count > 0 {
			logrus.Info("seedBaselineTags: already seeded, skip")
			return
		}
	}

	ensureSeedTags(ctx, repo)
}

// ensureSeedTags inserts any missing baseline tag by name. It is idempotent:
// existing tags are left untouched, and a soft-deleted row with the same name
// is purged first (repo.Create) so the unique index cannot block the insert.
func ensureSeedTags(ctx context.Context, repo repository.TagRepository) {
	created := 0
	for _, name := range seedTagNames {
		if _, err := repo.GetByName(ctx, name); err == nil {
			continue
		} else if !errors.Is(err, repository.ErrNotFound) {
			logrus.Warnf("seed tags: lookup %s: %v", name, err)
			continue
		}

		tag := &model.Tag{ID: pkg.NewID(), Name: name, CreatedAt: time.Now()}
		if err := repo.Create(ctx, tag); err != nil {
			logrus.Errorf("seed tags: create %s: %v", name, err)
			continue
		}
		created++
	}

	logrus.Infof("seed tags: %d baseline tags ensured, %d created", len(seedTagNames), created)
}
