package server

import (
	"time"

	"orinforum/internal/db"
	"orinforum/internal/model"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// migrateTrustData 把存量库升级到「数值等级 + total_xp」模型。
//
// 迁移是幂等的：只做两件事——把 trust_level 的旧字符串值换成数字文本，把
// total_points 的历史值复制到 total_xp。XP 的权威来源是行为聚合，复制只是
// 让升级瞬间的数字不至于归零；随后的全量重算会覆盖它。
//
// 之所以在启动时做而不是只留 SQL 脚本：仓库里的 db/migrations 没有自动执行器
// （见 app/server/AGENTS.md），把字符串等级留着会让下一次读取 users 直接失败。
func migrateTrustData(store db.Store) {
	gdb := store.SQL().DB()
	migrator := gdb.Migrator()

	// 1. trust_level: 旧库存的是 basic/member/regular/leader。
	//
	// 不能用「列类型是不是 varchar」来判断：GORM 的 SQLite AutoMigrate 会为
	// 类型变更重建表（列声明变成 integer），但已存在的行仍以 text 存储。因此
	// 判据换成「值是不是文本」（SQLite 的 typeof）。
	switch store.Type() {
	case db.BackendSQLite:
		if err := gdb.Exec(`UPDATE users SET trust_level =
			CASE trust_level
				WHEN 'member' THEN '1'
				WHEN 'regular' THEN '2'
				WHEN 'leader' THEN '3'
				WHEN 'basic' THEN '0'
				ELSE trust_level END
			WHERE typeof(trust_level) = 'text'`).Error; err != nil {
			logrus.Warnf("migrate trust_level values: %v", err)
		} else {
			logrus.Info("migrateTrustData: normalized users.trust_level values")
		}
	default:
		// PostgreSQL / MySQL 的类型变更由 db/migrations/2026-10-29_trust_level_xp.sql
		// 显式处理；这里只做一次幂等的文本映射尝试。
		if err := gdb.Exec(`UPDATE users SET trust_level =
			CASE CAST(trust_level AS CHAR)
				WHEN 'member' THEN '1'
				WHEN 'regular' THEN '2'
				WHEN 'leader' THEN '3'
				WHEN 'basic' THEN '0'
				ELSE trust_level END`).Error; err != nil {
			logrus.Warnf("migrate trust_level values: %v (run db/migrations/2026-10-29_trust_level_xp.sql)", err)
		}
	}

	// 2. 旧列清理：reputation 已由「按等级计算票权」取代（P4）。
	if migrator.HasColumn("users", "reputation") {
		if err := gdb.Exec(`ALTER TABLE users DROP COLUMN reputation`).Error; err != nil {
			logrus.Warnf("drop users.reputation (run db/migrations manually on old SQLite < 3.35): %v", err)
		}
	}

	// 3. spaces.post_count 是无人维护的旧缓存列（P4），删列避免误导运营。
	if migrator.HasColumn("spaces", "post_count") {
		if err := gdb.Exec(`ALTER TABLE spaces DROP COLUMN post_count`).Error; err != nil {
			logrus.Warnf("drop spaces.post_count: %v", err)
		}
	}

	// 4. total_points → total_xp（只补零值，避免覆盖已重算的数据）
	if migrator.HasColumn("users", "total_points") {
		if err := gdb.Exec(`UPDATE users SET total_xp = total_points WHERE total_xp = 0 AND total_points <> 0`).Error; err != nil {
			logrus.Warnf("migrate total_points → total_xp: %v", err)
		}
	}
}

// migrateSpaceVisibility 把旧的 auth_required 布尔开关迁移成 visibility 枚举。
//
// 幂等：迁移后立刻把 auth_required 清零，下一次启动没有可迁移的行；
// 管理员之后把某个 login 版面改回 public 也不会被再次改回 login。
func migrateSpaceVisibility(store db.Store) {
	res := store.SQL().DB().Exec(
		`UPDATE spaces SET visibility = ?, auth_required = ? WHERE auth_required = ?`,
		model.SpaceVisibilityLogin, false, true,
	)
	if res.Error != nil {
		logrus.Warnf("migrate spaces.auth_required → visibility: %v", res.Error)
	} else if res.RowsAffected > 0 {
		logrus.Infof("migrateSpaceVisibility: %d spaces switched to login", res.RowsAffected)
	}
}

// migrateModerationMode 把旧的布尔审核开关迁移成 moderation_mode 枚举。
//
// 只迁移一次：moderation_mode 没有行时才写。post_approval=true 的存量站点继续
// 「全部人工」（行为不变），其余站点落到默认的「发布后异步审核」。必须早于
// syncDefaultSettings，否则它会先把 moderation_mode 补成 async，旧开关的意图就丢了。
func migrateModerationMode(store db.Store) {
	gdb := store.SQL().DB()

	var existing model.Setting
	if err := gdb.Where("key = ?", model.SettingModerationMode).First(&existing).Error; err == nil {
		return // 已经迁移过 / 管理员配置过，尊重现值
	}

	mode := model.DefaultModerationMode
	var legacy model.Setting
	if err := gdb.Where("key = ?", "post_approval").First(&legacy).Error; err == nil && legacy.Value == "true" {
		mode = model.ModerationModeManual
	}
	if err := gdb.Create(&model.Setting{Key: model.SettingModerationMode, Value: mode, UpdatedAt: time.Now()}).Error; err != nil {
		logrus.Warnf("migrate post_approval → moderation_mode: %v", err)
		return
	}
	logrus.Infof("migrateModerationMode: moderation_mode=%s", mode)
}

// seedTrustDefaults 在配置表为空时写入定稿默认值。只按「整表为空」判断，
// 不逐行补齐——管理员删掉的规则不应在下次重启时自己回来。
func seedTrustDefaults(store db.Store) {
	gdb := store.SQL().DB()

	if isEmptyTable(gdb, &model.XPRule{}) {
		for i := range model.DefaultXPRules {
			if err := gdb.Create(&model.DefaultXPRules[i]).Error; err != nil {
				logrus.Warnf("seed xp rule %s: %v", model.DefaultXPRules[i].Metric, err)
			}
		}
		logrus.Infof("seeded %d XP rules", len(model.DefaultXPRules))
	}
	if isEmptyTable(gdb, &model.XPStreakBonus{}) {
		for i := range model.DefaultXPStreakBonuses {
			if err := gdb.Create(&model.DefaultXPStreakBonuses[i]).Error; err != nil {
				logrus.Warnf("seed streak bonus %d: %v", model.DefaultXPStreakBonuses[i].Days, err)
			}
		}
		logrus.Infof("seeded %d streak bonuses", len(model.DefaultXPStreakBonuses))
	}
	if isEmptyTable(gdb, &model.TrustLevel{}) {
		for i := range model.DefaultTrustLevels {
			if err := gdb.Create(&model.DefaultTrustLevels[i]).Error; err != nil {
				logrus.Warnf("seed trust level %d: %v", model.DefaultTrustLevels[i].Level, err)
			}
		}
		logrus.Infof("seeded %d trust levels", len(model.DefaultTrustLevels))
	}
}

func isEmptyTable(gdb *gorm.DB, tbl any) bool {
	var n int64
	if err := gdb.Model(tbl).Count(&n).Error; err != nil {
		return false
	}
	return n == 0
}

// seedGateDefaults 在门控表为空时写入域默认动作与外链规则；不逐行补齐，
// 管理员删除的规则不会在重启后自己回来。
func seedGateDefaults(store db.Store) {
	gdb := store.SQL().DB()
	if isEmptyTable(gdb, &model.GateDomainDefault{}) {
		for i := range model.DefaultGateDomainDefaults {
			if err := gdb.Create(&model.DefaultGateDomainDefaults[i]).Error; err != nil {
				logrus.Warnf("seed gate domain %s: %v", model.DefaultGateDomainDefaults[i].Domain, err)
			}
		}
		logrus.Infof("seeded %d gate domain defaults", len(model.DefaultGateDomainDefaults))
	}
	if isEmptyTable(gdb, &model.GateRule{}) {
		for i := range model.DefaultGateRules {
			if err := gdb.Create(&model.DefaultGateRules[i]).Error; err != nil {
				logrus.Warnf("seed gate rule %s: %v", model.DefaultGateRules[i].ID, err)
			}
		}
		logrus.Infof("seeded %d gate rules", len(model.DefaultGateRules))
	}
}
