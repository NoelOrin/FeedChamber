import starlight from '@astrojs/starlight';
import { defineConfig } from 'astro/config';
import rehypeBaseLinks from './scripts/rehype-base-links.mjs';

/**
 * Feed Chamber 对外文档站。
 *
 * 内容住在 `src/content/docs/`（本包内部），与仓库根的 `docs/` 物理隔离——
 * 后者是内部工程材料（plans / reviews / ADR），不进这套导航。
 *
 * 多版本预留：现在单版本（当前分支即唯一版本）。需要并行版本时把内容移入
 * 版本目录并接 `starlight-versions`，本配置的侧栏结构不用改。
 */
export default defineConfig({
	/*
	 * GitHub Pages 项目页挂在 https://<owner>.github.io/<repo>/，所有绝对链接
	 * 都必须带上 /<repo>/ 前缀，否则站内跳转与静态资源全部 404。
	 * 因此 site/base 由环境变量注入：本地开发留空（挂在根），CI 里传真实值。
	 * 换到自定义域名时只改 CI 的环境变量，不用动本文件。
	 */
	site: process.env.DOCS_SITE ?? 'https://noelorin.github.io',
	base: process.env.DOCS_BASE ?? '/',
	/*
	 * 正文里手写的站内绝对链接要补 base 前缀，否则 GitHub Pages 项目页上会 404。
	 * 侧栏与分页由 Starlight 自己处理，这个插件只兜正文。
	 */
	markdown: {
		rehypePlugins: [[rehypeBaseLinks, { base: process.env.DOCS_BASE ?? '/' }]],
	},
	integrations: [
		starlight({
			title: 'Feed Chamber',
			description: 'Go + React 社区论坛的架构、API、MCP、部署与二开指南',
			defaultLocale: 'root',
			locales: {
				root: { label: '简体中文', lang: 'zh-CN' },
			},
			credits: false,
			favicon: '/favicon.svg',
			customCss: ['./src/styles/custom.css'],
			lastUpdated: false,
			sidebar: [
				{
					label: '概览',
					items: [
						{ label: '文档首页', link: '/' },
						{ label: '快速开始', link: '/getting-started/' },
						{ label: '功能总览', link: '/capabilities/' },
						{ label: '术语表', link: '/glossary/' },
					],
				},
				{
					label: '架构',
					items: [
						{ label: '架构总览', link: '/architecture/overview/' },
						{ label: '分层职责', link: '/architecture/layers/' },
						{ label: '可见性与门控', link: '/architecture/visibility/' },
						{ label: '事件与缓存', link: '/architecture/events/' },
					],
				},
				{
					label: 'API',
					items: [
						{ label: 'API 约定', link: '/api/conventions/' },
						{ label: '错误码参考', link: '/api/errors/' },
						{ label: '按域查端点', link: '/api/domains/' },
						{ label: '下载 openapi.json', link: '/api/download/' },
					],
				},
				{
					label: 'MCP',
					items: [
						{ label: 'MCP 概览', link: '/mcp/overview/' },
						{ label: '作为 MCP Server', link: '/mcp/server/' },
						{ label: '作为 MCP Client', link: '/mcp/client/' },
					],
				},
				{
					label: '部署',
					items: [
						{ label: 'Docker Compose', link: '/deploy/compose/' },
						{ label: '本地开发', link: '/deploy/local/' },
						{ label: '环境变量', link: '/deploy/env/' },
					],
				},
				{
					label: '二开',
					items: [
						{ label: '新增一个模块', link: '/extend/add-module/' },
						{ label: '扩展点', link: '/extend/extension-points/' },
					],
				},
				{
					label: '常见问题',
					items: [{ label: 'FAQ', link: '/faq/' }],
				},
				{
					label: '路线图',
					items: [{ label: '状态与计划', link: '/roadmap/' }],
				},
			],
		}),
	],
});
