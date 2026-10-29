-- 版面级禁言系统迁移脚本
-- 执行时间：2024-XX-XX

-- 1. 添加新字段到 banned 表
ALTER TABLE banned ADD COLUMN scope VARCHAR(20) NOT NULL DEFAULT 'global';
ALTER TABLE banned ADD COLUMN space_id VARCHAR(32) DEFAULT NULL;

-- 2. 更新现有数据（将现有记录标记为全局禁言）
UPDATE banned SET scope = 'global', space_id = NULL WHERE scope IS NULL OR scope = '';

-- 3. 创建索引
CREATE INDEX IF NOT EXISTS idx_banned_user_scope ON banned(user_id, scope);
CREATE INDEX IF NOT EXISTS idx_banned_scope ON banned(scope);
CREATE INDEX IF NOT EXISTS idx_banned_space_id ON banned(space_id);

-- 4. 创建联合唯一索引（可选，根据业务需求决定）
-- 注意：MySQL 和 PostgreSQL 语法略有不同，以下为通用写法
-- 如果使用 MySQL：
-- CREATE UNIQUE INDEX idx_banned_unique_global ON banned(user_id, scope) WHERE scope = 'global';
-- 如果使用 PostgreSQL：
-- CREATE UNIQUE INDEX idx_banned_unique_global ON banned(user_id, scope) WHERE scope = 'global';
-- CREATE UNIQUE INDEX idx_banned_unique_space ON banned(user_id, scope, space_id) WHERE scope = 'space';

-- 5. 验证迁移结果
SELECT
    scope,
    COUNT(*) as count,
    SUM(CASE WHEN space_id IS NULL OR space_id = '' THEN 1 ELSE 0 END) as no_space
FROM banned
GROUP BY scope;

-- 6. 检查数据完整性（应该没有重复记录）
SELECT user_id, scope, space_id, COUNT(*)
FROM banned
GROUP BY user_id, scope, space_id
HAVING COUNT(*) > 1;
