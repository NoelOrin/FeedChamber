-- OAuth 令牌改为 opaque 后的遗留清理（change: 2026-10-18-oauth-opaque-tokens）
--
-- 背景：access token 由 JWT 改为 opaque 字符串（`of_at_` + 32B base64url），
-- 服务端只存 SHA-256 hex 于 `oauth_tokens.access_token`。旧 JWT 时代该列存的是
-- JTI，新逻辑永远不会命中这些行——它们既不会被校验通过，也不会被撤销流程更新。
--
-- 本脚本是**可选的**维护动作，不影响正确性：
--   * 旧行本来就会在 `expires_at` 到点后失效；
--   * 新逻辑按 `access_token = sha256(token)` 精确匹配，JTI 行不参与任何查询。
-- 因此可以安全跳过；执行也只是让表更干净、避免误读监控里的行数。
--
-- 幂等：用长度判定。opaque 令牌的 hash 恒为 64 个 hex 字符；JTI 是 UUID 形态
-- （36 字符，含连字符）。两者不会重叠。
--
-- 兼容性：SQLite / MySQL / PostgreSQL 均支持以下语法。

-- 1) 先看有多少遗留行（JTI 形态 = 长度不为 64）
SELECT COUNT(*) AS legacy_rows
FROM oauth_tokens
WHERE LENGTH(access_token) <> 64;

-- 2) 删除遗留 JTI 行。
--    先备份再删：如需留档，把 SELECT 换成 CREATE TABLE oauth_tokens_jti_backup AS ...
DELETE FROM oauth_tokens
WHERE LENGTH(access_token) <> 64;

-- 3) 清理 settings 表中已不再被代码读取的两个旧 key。
--    （代码侧已于同 change 移除读取，见 internal/model/setting.go 的默认值表）
DELETE FROM settings
WHERE key IN ('jwt_access_token_ttl', 'jwt_refresh_token_ttl');

-- 4) 验证：两步都应返回 0
SELECT COUNT(*) AS remaining_legacy_rows FROM oauth_tokens WHERE LENGTH(access_token) <> 64;
SELECT COUNT(*) AS remaining_legacy_settings
FROM settings WHERE key IN ('jwt_access_token_ttl', 'jwt_refresh_token_ttl');
