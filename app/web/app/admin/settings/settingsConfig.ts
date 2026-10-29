export interface SettingItemDef {
	key: string;
	value: string;
	description?: string;
	type?: 'boolean' | 'secret' | 'select';
	options?: { label: string; value: string }[];
}

export type ServiceTabKey = 'site' | 'user' | 'content' | 'oauth' | 'storage';

export interface SettingsGroup {
	name: string;
	tab: ServiceTabKey;
	items: SettingItemDef[];
}

/** 服务设置下的细分 Tab，顺序即展示顺序 */
export const serviceTabs: { key: ServiceTabKey; label: string }[] = [
	{ key: 'site', label: '站点 / SEO' },
	{ key: 'user', label: '用户与认证' },
	{ key: 'content', label: '内容与通知' },
	{ key: 'oauth', label: '第三方登录' },
	{ key: 'storage', label: '存储设置' },
];

export const settingsGroups: SettingsGroup[] = [
	{
		name: '站点信息',
		tab: 'site',
		items: [
			{ key: 'site_name', value: '', description: '网站名称' },
			{ key: 'site_description', value: '', description: '网站描述' },
			{ key: 'site_logo', value: '', description: 'Logo URL' },
			{ key: 'site_url', value: '', description: '网站 URL' },
		],
	},
	{
		name: 'SEO 设置',
		tab: 'site',
		items: [
			{ key: 'seo_title', value: '', description: 'SEO 标题（站点默认标题；帖子页以帖子标题为准）' },
			{ key: 'seo_description', value: '', description: 'SEO 描述（缺省回退「网站描述」）' },
			{ key: 'seo_keywords', value: '', description: 'SEO 关键词（逗号分隔）' },
			{ key: 'og_image', value: '', description: '分享图 URL（缺省回退 Logo；用于 og:image 与 Twitter 卡片）' },
			{ key: 'google_site_verification', value: '', description: 'Google Search Console 验证码' },
			{ key: 'baidu_site_verification', value: '', description: '百度搜索资源平台验证码' },
			{
				key: 'user_profile_indexable',
				value: 'false',
				description: '允许搜索引擎索引他人主页（UGC 薄内容，默认关闭）',
				type: 'boolean',
			},
		],
	},
	{
		name: '用户与注册',
		tab: 'user',
		items: [
			{ key: 'register_enabled', value: 'true', description: '允许注册', type: 'boolean' },
			{
				key: 'register_mode',
				value: 'all',
				description: '注册方式',
				type: 'select',
				options: [
					{ label: '开放注册', value: 'all' },
					{ label: '仅邮箱注册', value: 'email' },
					{ label: '仅用户名注册', value: 'username' },
					{ label: '邀请码注册', value: 'invite' },
					{ label: '审批注册', value: 'approve' },
				],
			},
		],
	},
	{
		name: '发帖与评论',
		tab: 'content',
		items: [
			{
				key: 'moderation_mode',
				value: 'async',
				description: '文章审核模式',
				type: 'select',
				options: [
					{ label: '不审核（发布即公开）', value: 'off' },
					{ label: '发布后异步审核（默认，命中转人工）', value: 'async' },
					{ label: '外部自动化审核（发布前调用外部服务）', value: 'external' },
					{ label: '全部人工审核（发布即待审）', value: 'manual' },
				],
			},
			{ key: 'email_verification_required', value: 'false', description: '发帖/评论前需验证邮箱', type: 'boolean' },
			{ key: 'site_maintenance', value: 'false', description: '站点维护模式（非管理员只读）', type: 'boolean' },
			{ key: 'site_maintenance_message', value: '站点维护中，请稍后再试', description: '维护模式提示文案' },
			{ key: 'post_auth_required', value: 'false', description: '发帖需要登录', type: 'boolean' },
			{ key: 'comment_auth_required', value: 'false', description: '评论需要登录', type: 'boolean' },
			{ key: 'article_page_size', value: '20', description: '每页文章数' },
			{ key: 'page_max_size', value: '100', description: '分页每页上限' },
			{ key: 'page_default_size', value: '20', description: '分页默认每页数' },
		],
	},
	{
		name: 'OAuth2 / OIDC',
		tab: 'user',
		items: [
			{ key: 'oauth_access_token_ttl', value: '86400', description: 'OAuth2 access token 有效期（秒，令牌为 opaque）' },
			{ key: 'sso_id_token_ttl', value: '3600', description: 'OIDC id_token 有效期（秒）' },
		],
	},
	{
		name: '上传设置',
		tab: 'content',
		items: [
			{ key: 'upload_max_size', value: '10485760', description: '上传文件大小上限（字节，10MB=10485760）' },
			{
				key: 'upload_allowed_types',
				value: 'image/jpeg,image/png,image/gif,image/webp',
				description: '允许的 MIME 类型（逗号分隔）',
			},
			{ key: 'upload_presign_expiry', value: '900', description: '预签名 URL 有效期（秒）' },
		],
	},
	{
		name: 'SSE / 通知',
		tab: 'content',
		items: [
			{ key: 'sse_polling_interval', value: '5', description: 'SSE 轮询间隔（秒）' },
			{ key: 'view_count_flush_interval', value: '60', description: '浏览量批量刷入间隔（秒）' },
		],
	},
	{
		name: 'OAuth 第三方登录',
		tab: 'oauth',
		items: [
			{ key: 'oauth_github_enabled', value: 'false', description: '启用 GitHub 登录', type: 'boolean' },
			{ key: 'oauth_github_client_id', value: '', description: 'GitHub Client ID' },
			{ key: 'oauth_github_client_secret', value: '', description: 'GitHub Client Secret', type: 'secret' },
			{ key: 'oauth_github_callback', value: '', description: 'GitHub 回调 URL' },
			{ key: 'oauth_google_enabled', value: 'false', description: '启用 Google 登录', type: 'boolean' },
			{ key: 'oauth_google_client_id', value: '', description: 'Google Client ID' },
			{ key: 'oauth_google_client_secret', value: '', description: 'Google Client Secret', type: 'secret' },
			{ key: 'oauth_google_callback', value: '', description: 'Google 回调 URL' },
			{ key: 'oauth_qq_enabled', value: 'false', description: '启用 QQ 登录', type: 'boolean' },
			{ key: 'oauth_qq_client_id', value: '', description: 'QQ Client ID' },
			{ key: 'oauth_qq_client_secret', value: '', description: 'QQ Client Secret', type: 'secret' },
			{ key: 'oauth_qq_callback', value: '', description: 'QQ 回调 URL' },
			{ key: 'oauth_wechat_enabled', value: 'false', description: '启用微信登录', type: 'boolean' },
			{ key: 'oauth_wechat_client_id', value: '', description: '微信 AppID' },
			{ key: 'oauth_wechat_client_secret', value: '', description: '微信 AppSecret', type: 'secret' },
			{ key: 'oauth_wechat_callback', value: '', description: '微信回调 URL' },
			{ key: 'oauth_weibo_enabled', value: 'false', description: '启用微博登录', type: 'boolean' },
			{ key: 'oauth_weibo_client_id', value: '', description: '微博 App Key' },
			{ key: 'oauth_weibo_client_secret', value: '', description: '微博 App Secret', type: 'secret' },
			{ key: 'oauth_weibo_callback', value: '', description: '微博回调 URL' },
		],
	},
	{
		name: '邮件服务',
		tab: 'user',
		items: [
			{ key: 'email_enabled', value: 'false', description: '启用邮件发送', type: 'boolean' },
			{
				key: 'email_provider',
				value: 'log',
				description: '邮件提供商',
				type: 'select',
				options: [
					{ label: '日志（开发）', value: 'log' },
					{ label: 'SMTP', value: 'smtp' },
					{ label: 'SendGrid', value: 'sendgrid' },
					{ label: 'Resend', value: 'resend' },
				],
			},
			{ key: 'email_smtp_host', value: '', description: 'SMTP Host' },
			{ key: 'email_smtp_port', value: '587', description: 'SMTP Port' },
			{ key: 'email_username', value: '', description: 'SMTP / 邮箱用户名' },
			{ key: 'email_password', value: '', description: 'SMTP / 邮箱密码', type: 'secret' },
			{ key: 'email_from', value: '', description: '发件人邮箱' },
			{ key: 'email_from_name', value: 'Feed Chamber', description: '发件人名称' },
			{ key: 'email_use_tls', value: 'false', description: 'SMTP 使用 TLS', type: 'boolean' },
			{ key: 'email_api_key', value: '', description: 'SendGrid / Resend API Key', type: 'secret' },
			{ key: 'email_code_ttl', value: '600', description: '邮箱验证码有效期（秒）' },
			{ key: 'email_resend_cooldown', value: '60', description: '验证码重发冷却（秒）' },
			{ key: 'email_daily_limit', value: '10', description: '每日验证码发送上限' },
		],
	},
	{
		name: '安全与高级认证',
		tab: 'user',
		items: [
			{ key: 'api_key_enabled', value: 'false', description: '启用 API Key 网关', type: 'boolean' },
			{ key: 'api_key_default_rate_limit', value: '100', description: 'API Key 默认限流（次/窗口）' },
			{
				key: 'mfa_totp_enabled',
				value: 'false',
				description: '启用 TOTP MFA（阶段 3，当前仅配置预留）',
				type: 'boolean',
			},
		],
	},
	{
		name: '存储设置 (S3)',
		tab: 'storage',
		items: [
			{
				key: 'storage_provider',
				value: 'local',
				description: '存储后端',
				type: 'select',
				options: [
					{ label: '本地存储', value: 'local' },
					{ label: 'S3 兼容', value: 's3' },
				],
			},
			{ key: 'storage_local_dir', value: 'uploads', description: '本地存储目录' },
			{ key: 's3_endpoint', value: '', description: 'S3 Endpoint' },
			{ key: 's3_region', value: 'us-east-1', description: 'S3 Region' },
			{ key: 's3_bucket', value: '', description: 'S3 Bucket' },
			{ key: 's3_access_key_id', value: '', description: 'S3 Access Key ID' },
			{ key: 's3_access_key_secret', value: '', description: 'S3 Access Key Secret', type: 'secret' },
			{ key: 's3_use_ssl', value: 'true', description: '使用 SSL', type: 'boolean' },
			{ key: 's3_custom_domain', value: '', description: 'S3 自定义域名/CDN' },
		],
	},
];
