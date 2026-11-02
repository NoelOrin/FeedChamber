import starlight from '@astrojs/starlight';
import { defineConfig } from 'astro/config';
import rehypeBaseLinks from './scripts/rehype-base-links.mjs';

/**
 * Feed Chamber 对外文档站（Astro + Starlight）。
 *
 * 结构参考成熟开源项目的文档站：分组侧栏 + 任务型正文 + 参考页全表。
 * 首页是普通文档页（保留左侧导航），而不是把入口卡片塞进 splash 模板。
 *
 * 内容住 `src/content/docs/`（zh-CN 在根，英文在 `en/`），与仓库根的 `docs/`
 * 物理隔离——后者是内部工程材料，不进这套导航。
 *
 * GitHub Pages 项目页挂在 `https://<owner>.github.io/<repo>/`，所有绝对资源
 * 都要带 base 前缀：site/base 由环境变量注入，本地留空。
 */
const site = process.env.DOCS_SITE ?? 'https://noelorin.github.io';
const base = process.env.DOCS_BASE ?? '/';
const repo = 'https://github.com/NoelOrin/FeedChamber';
const asset = (path) => `${base.replace(/\/$/, '')}/${path.replace(/^\//, '')}`;

export default defineConfig({
	site,
	base,
	// 开发工具栏会遮住内容、也会出现在截图里；文档站不需要它。
	devToolbar: { enabled: false },
	markdown: {
		// 正文里手写的站内绝对链接要补 base 前缀，否则项目页上会静默 404。
		// 侧栏与分页由 Starlight 自己处理，这个插件只兜正文。
		rehypePlugins: [[rehypeBaseLinks, { base }]],
	},
	integrations: [
		starlight({
			title: {
				'zh-CN': 'Feed Chamber 文档',
				en: 'Feed Chamber Docs',
			},
			description: 'Go + React 社区论坛：安装部署、配置、管理、运维与 API 集成文档。',
			titleDelimiter: '·',
			logo: {
				src: './public/logo.svg',
				alt: 'Feed Chamber',
			},
			favicon: '/favicon.svg',
			components: {
				// 首页标题不带站点前缀，其余页面用 Starlight 默认拼接。
				Head: './src/components/Head.astro',
			},
			defaultLocale: 'root',
			locales: {
				root: { label: '简体中文', lang: 'zh-CN' },
				en: { label: 'English', lang: 'en' },
			},
			social: [{ icon: 'github', label: 'GitHub', href: repo }],
			editLink: {
				baseUrl: `${repo}/edit/main/app/docs/src/content/docs/`,
			},
			lastUpdated: true,
			credits: false,
			tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
			customCss: ['./src/styles/custom.css'],
			head: [
				{ tag: 'meta', attrs: { property: 'og:type', content: 'website' } },
				{ tag: 'meta', attrs: { property: 'og:site_name', content: 'Feed Chamber' } },
				{ tag: 'meta', attrs: { property: 'og:image', content: asset('og.png') } },
				{ tag: 'meta', attrs: { name: 'twitter:card', content: 'summary_large_image' } },
				{ tag: 'meta', attrs: { name: 'twitter:image', content: asset('og.png') } },
				{
					tag: 'meta',
					attrs: {
						name: 'theme-color',
						content: '#ffffff',
						media: '(prefers-color-scheme: light)',
					},
				},
				{
					tag: 'meta',
					attrs: {
						name: 'theme-color',
						content: '#0b0b0f',
						media: '(prefers-color-scheme: dark)',
					},
				},
			],
			expressiveCode: {
				styleOverrides: { borderRadius: '8px', codeFontSize: '0.84rem' },
			},
			pagefind: {
				ranking: {
					pageLength: 0.05,
					termFrequency: 0.2,
					termSaturation: 1.4,
					termSimilarity: 6,
				},
			},
			sidebar: [
				{
					label: '开始使用',
					translations: { en: 'Get Started' },
					items: [
						{ label: '快速开始', translations: { en: 'Quick Start' }, slug: 'getting-started' },
						{
							label: '选择部署方式',
							translations: { en: 'Choose a Deployment' },
							slug: 'install/choose-path',
						},
						{
							label: 'Docker Compose 安装',
							translations: { en: 'Install with Docker Compose' },
							slug: 'install/docker-compose',
						},
						{
							label: '本地开发',
							translations: { en: 'Local Development' },
							slug: 'install/local',
						},
						{
							label: '生产部署清单',
							translations: { en: 'Production Checklist' },
							slug: 'install/production',
						},
					],
					collapsed: false,
				},
				{
					label: '配置',
					translations: { en: 'Configuration' },
					items: [
						{
							label: '站点设置',
							translations: { en: 'Site Settings' },
							slug: 'configuration/site',
						},
						{
							label: '认证与安全',
							translations: { en: 'Authentication and Security' },
							slug: 'configuration/authentication',
						},
						{
							label: '数据库、NATS 与存储',
							translations: { en: 'Database, NATS, and Storage' },
							slug: 'configuration/infrastructure',
						},
						{
							label: '内容管线',
							translations: { en: 'Content Pipeline' },
							slug: 'configuration/content',
						},
						{
							label: '成长体系',
							translations: { en: 'Growth and XP' },
							slug: 'configuration/growth',
						},
						{
							label: '站点自定义',
							translations: { en: 'Site Customization' },
							slug: 'configuration/customization',
						},
					],
					collapsed: true,
				},
				{
					label: '管理',
					translations: { en: 'Administration' },
					items: [
						{
							label: '后台总览',
							translations: { en: 'Admin Overview' },
							slug: 'administration/overview',
						},
						{
							label: '用户与权限',
							translations: { en: 'Users and Permissions' },
							slug: 'administration/users',
						},
						{
							label: '内容与审核',
							translations: { en: 'Content and Moderation' },
							slug: 'administration/content',
						},
						{
							label: '系统与集成',
							translations: { en: 'System and Integrations' },
							slug: 'administration/system',
						},
					],
					collapsed: true,
				},
				{
					label: '运维',
					translations: { en: 'Operations' },
					items: [
						{
							label: '架构总览',
							translations: { en: 'Architecture' },
							slug: 'operations/architecture',
						},
						{
							label: '数据与事件',
							translations: { en: 'Data and Events' },
							slug: 'operations/data-and-events',
						},
						{
							label: '观测与指标',
							translations: { en: 'Observability and Metrics' },
							slug: 'operations/observability',
						},
						{
							label: '排障',
							translations: { en: 'Troubleshooting' },
							slug: 'operations/troubleshooting',
						},
						{
							label: '升级与迁移',
							translations: { en: 'Upgrades and Migrations' },
							slug: 'operations/upgrades',
						},
					],
					collapsed: true,
				},
				{
					label: 'API 与集成',
					translations: { en: 'API and Integrations' },
					items: [
						{
							label: 'API 约定',
							translations: { en: 'API Conventions' },
							slug: 'api/conventions',
						},
						{
							label: '认证与权限',
							translations: { en: 'Authentication and Permissions' },
							slug: 'api/authentication',
						},
						{
							label: '调用示例',
							translations: { en: 'API Examples' },
							slug: 'api/examples',
						},
						{
							label: '实时事件（SSE）',
							translations: { en: 'Realtime (SSE)' },
							slug: 'api/realtime',
						},
						{
							label: 'MCP 集成',
							translations: { en: 'MCP Integration' },
							slug: 'api/mcp',
						},
						{ label: 'OpenAPI', slug: 'api/openapi' },
					],
					collapsed: true,
				},
				{
					label: '参考',
					translations: { en: 'Reference' },
					items: [
						{
							label: '端点索引',
							translations: { en: 'Endpoint Index' },
							slug: 'reference/endpoints',
						},
						{
							label: '配置字段',
							translations: { en: 'Configuration Fields' },
							slug: 'reference/configuration',
						},
						{
							label: '环境变量',
							translations: { en: 'Environment Variables' },
							slug: 'reference/environment',
						},
						{
							label: '错误码',
							translations: { en: 'Error Codes' },
							slug: 'reference/errors',
						},
						{ label: 'CLI', slug: 'reference/cli' },
					],
					collapsed: true,
				},
				{
					label: '项目',
					translations: { en: 'Project' },
					items: [
						{ label: '常见问题', translations: { en: 'FAQ' }, slug: 'faq' },
						{ label: '路线图', translations: { en: 'Roadmap' }, slug: 'roadmap' },
						{
							label: '贡献指南',
							translations: { en: 'Contributing' },
							slug: 'contributing',
						},
						{ label: '许可证', translations: { en: 'License' }, slug: 'license' },
						{ label: '术语表', translations: { en: 'Glossary' }, slug: 'glossary' },
					],
					collapsed: true,
				},
			],
		}),
	],
});
