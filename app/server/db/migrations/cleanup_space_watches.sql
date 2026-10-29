-- 版面关注（target_type='space'）移除后的存量清理（2026-10-28）
--
-- 背景：产品决定不再提供「关注版面」——它与帖子详情的「关注话题」语义重叠，只保留后者。
-- 前台入口（单版首页头部卡那颗按钮）已删除，后端 `validTargetType` 也不再接受 "space"
-- （见 internal/service/watch.go、internal/model/watch.go）。
--
-- 本脚本是**可选的**维护动作，不影响正确性：
--   * 扇出逻辑已删掉 space 分支，存量行不会再产生任何通知；
--   * 新的 /watches/set 传 "space" 会直接被拒（2001 无效的关注目标），不会新增。
-- 因此可以安全跳过；执行只是让表更干净、避免误读监控里的行数。
--
-- 幂等：按 target_type 精确匹配，重复执行结果相同（第二次影响 0 行）。
--
-- 兼容性：SQLite / MySQL / PostgreSQL 均支持以下语法。

-- 1) 先看有多少遗留行
SELECT COUNT(*) AS legacy_space_watches
FROM watches
WHERE target_type = 'space';

-- 2) 删除遗留的版面关注行。
--    先备份再删：如需留档，把 SELECT 换成 CREATE TABLE watches_space_backup AS ...
DELETE FROM watches
WHERE target_type = 'space';

-- 3) 验证：应返回 0
SELECT COUNT(*) AS remaining_space_watches
FROM watches
WHERE target_type = 'space';
