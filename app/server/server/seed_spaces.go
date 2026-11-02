package server

import (
	"context"
	"errors"

	"feedchamber/internal/db"
	"feedchamber/internal/model"
	"feedchamber/internal/pkg"
	"feedchamber/internal/repository"

	"github.com/sirupsen/logrus"
)

// darkZoneSlug 与 service.DarkZoneSlug 保持一致（跨包常量不宜互相 import，
// 这里独立声明并由测试/使用处共同约束）。
const darkZoneSlug = "dark-zone"

// seedSpace 描述一个基础版面。图标存的是「图标名」而不是 emoji：emoji 是字体
// 渲染的产物，各平台字形不一致、无法跟随主题着色，前端只能靠一张映射表去猜。
// 存名字后前端直接查图标库（app/web/components/space/space-icon.tsx），
// 新增版面不需要改前端代码。
type seedSpace struct {
	name string
	desc string
	slug string
	icon string
	sort int
	// system / recommendable 只对特殊版面（如暗区）设置，普通版面保持零值。
	system        bool
	recommendable bool
}

// seedSpaces 是新装站点的版面骨架，按 sort 升序展示（间隔 10，便于管理员插入）。
var seedSpaces = []seedSpace{
	{"技术", "编程、架构、工具等技术话题", "tech", "laptop", 10, false, true},
	{"问答", "提问与解答，求助前建议先搜索", "qa", "message-circle", 20, false, true},
	{"分享", "经验、教程与资源分享", "share", "sparkles", 30, false, true},
	{"产品", "产品设计、用户体验与需求讨论", "product", "smartphone", 40, false, true},
	{"运维", "部署、CI/CD、云原生与监控", "ops", "server", 50, false, true},
	{"闲聊", "日常分享与生活随想", "chat", "coffee", 60, false, true},
	{"公告", "站务公告与规则更新", "announcement", "megaphone", 70, false, true},
	{"反馈", "问题反馈与功能建议", "feedback", "bug", 80, false, true},
	{"暗区", "/版/暗区：被社区动议放逐的帖子与用户在此软隔离，可访问、可翻案，不进主推荐流", "dark-zone", "moon", 999, true, false},
}

// seedBaselineSpaces ensures the baseline spaces exist.
//
// 只在首次启动（!alreadySeeded）或版面表为空时执行：补齐缺失行、从不覆盖
// 已存在行（管理员改名/换图标/调排序不会被重启重置），也避免把管理员主动
// 删除的版面在下次重启时复活。
func seedBaselineSpaces(store db.Store, alreadySeeded bool) {
	ctx := context.Background()
	repo := repository.NewSpaceRepository(store)

	if alreadySeeded {
		spaces, err := repo.List(ctx)
		if err != nil {
			logrus.Warnf("seedBaselineSpaces: list spaces: %v", err)
			return
		}
		if len(spaces) > 0 {
			logrus.Info("seedBaselineSpaces: already seeded, skip")
			return
		}
	}

	ensureSeedSpaces(ctx, repo)
}

// ensureSeedSpaces creates any missing baseline space and returns slug → space ID.
// It is idempotent and never mutates an existing row, so it is safe to call both
// from startup seeding and from the dev post reseed (which needs the IDs).
func ensureSeedSpaces(ctx context.Context, repo repository.SpaceRepository) map[string]string {
	ids := make(map[string]string, len(seedSpaces))
	created := 0

	for _, s := range seedSpaces {
		existing, err := repo.FindBySlug(ctx, s.slug)
		if err == nil {
			ids[s.slug] = existing.ID
			continue
		}
		if !errors.Is(err, repository.ErrNotFound) {
			logrus.Warnf("seed spaces: lookup %s: %v", s.slug, err)
			continue
		}

		id := pkg.NewID()
		space := &model.Space{
			ID:            id,
			Name:          s.name,
			Description:   s.desc,
			Slug:          s.slug,
			Icon:          s.icon,
			Depth:         0,
			Path:          "/" + id,
			SortOrder:     s.sort,
			IsActive:      true,
			System:        s.system,
			Recommendable: s.recommendable,
		}
		if err := repo.Create(ctx, space); err != nil {
			logrus.Errorf("seed spaces: create %s: %v", s.slug, err)
			continue
		}
		ids[s.slug] = id
		created++
	}

	logrus.Infof("seed spaces: %d baseline spaces ensured, %d created", len(ids), created)
	return ids
}

// ensureSeedDarkZone 幂等确保系统「暗区」版面存在。
//
// 与 seedBaselineSpaces 不同，它**无条件执行**：老站点（alreadySeeded=true）也要能
// 在升级后拿到暗区，否则社区动议的 exile 结果无处落地。已存在行只校正
// system/recommendable 两个系统标记（不改管理员的其它自定义），符合"纯增量"迁移。
func ensureSeedDarkZone(ctx context.Context, repo repository.SpaceRepository) *model.Space {
	existing, err := repo.FindBySlug(ctx, darkZoneSlug)
	if err == nil && existing != nil {
		if !existing.System || existing.Recommendable {
			if err := repo.SetSystemFlags(ctx, existing.ID, true, false); err != nil {
				logrus.Warnf("seed dark-zone: fix flags: %v", err)
			}
			existing.System = true
			existing.Recommendable = false
		}
		return existing
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		logrus.Warnf("seed dark-zone: lookup: %v", err)
		return nil
	}

	id := pkg.NewID()
	space := &model.Space{
		ID:            id,
		Name:          "暗区",
		Description:   "/版/暗区：被社区动议放逐的帖子与用户在此软隔离，可访问、可翻案，不进主推荐流",
		Slug:          darkZoneSlug,
		Icon:          "moon",
		Depth:         0,
		Path:          "/" + id,
		SortOrder:     999,
		IsActive:      true,
		System:        true,
		Recommendable: false,
	}
	if err := repo.Create(ctx, space); err != nil {
		logrus.Errorf("seed dark-zone: create: %v", err)
		return nil
	}
	logrus.Info("seed dark-zone: created system space")
	return space
}
