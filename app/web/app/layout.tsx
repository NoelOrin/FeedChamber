import type { Metadata, Viewport } from 'next';

import './globals.css';

import { dehydrate } from '@tanstack/react-query';
import { headers } from 'next/headers';
import { SnippetSlot } from '~/components/customization/snippet-slot';
import { Providers } from '~/components/providers';
import { JsonLd } from '~/components/seo/json-ld';
import { emitSnippets } from '~/lib/customization/emit';
import { buildThemeOverrideCss, loadPublicCustomization } from '~/lib/customization/server';
import { loadPublicSettings, loadShellData } from '~/lib/queries/server';
import { getQueryClient } from '~/lib/query/client';
import { qk } from '~/lib/query/keys';

export const viewport: Viewport = {
	themeColor: '#8c2de4',
	width: 'device-width',
	initialScale: 1,
};

/** Site base URL: settings `site_url` wins, env is the fallback for unconfigured deploys. */
function siteBase(settings: Record<string, string>): string {
	return (settings.site_url || process.env.SITE_URL || '').replace(/\/$/, '');
}

export async function generateMetadata(): Promise<Metadata> {
	const h = await headers();
	const pathname = h.get('x-pathname') ?? '/';
	if (pathname === '/admin' || pathname.startsWith('/admin/')) {
		return { title: '管理后台', robots: { index: false, follow: false } };
	}
	const settings = await loadPublicSettings();
	const siteName = settings.site_name || 'Feed Chamber';
	const base = siteBase(settings);
	/*
	 * seo_title 是「站点默认标题」。它此前只落到 title.default，而 template 把
	 * 「Feed Chamber」硬编码成后缀——由于每个页面都带自己的 title，default 永远轮不到，
	 * 于是管理员改 seo_title 后首页/列表页标题毫无变化（spec 要求首页 title MUST 包含
	 * seo_title）。改成 template 也用配置标题，default 与后缀就都跟着设置走。
	 */
	const defaultTitle = settings.seo_title || siteName;
	const description = settings.seo_description || settings.site_description || 'Feed Chamber 社区论坛';
	const ogImage = settings.og_image || settings.site_logo;

	return {
		metadataBase: base ? new URL(base) : undefined,
		title: {
			default: defaultTitle,
			template: `%s — ${defaultTitle}`,
		},
		description,
		/* 以下三项是基线 `__root.tsx` head 的契约，迁移时漏了，此处补齐：
		   apple-touch-icon（基线指 /pwa-192x192.png）、apple-mobile-web-app-capable
		   （Next 的 appleWebApp.capable 只产出 mobile-web-app-capable，不带 apple- 前缀那份）。
		   manifest 的 <link> 在下面的 <head> 里补——Metadata API 没有 manifest 字段。 */
		icons: { icon: '/favicon.ico', apple: '/pwa-192x192.png' },
		appleWebApp: { title: siteName, capable: true, statusBarStyle: 'default' },
		other: { 'apple-mobile-web-app-capable': 'yes' },
		keywords: settings.seo_keywords ? settings.seo_keywords.split(',') : undefined,
		openGraph: {
			type: 'website',
			siteName,
			title: defaultTitle,
			description,
			...(base ? { url: base } : {}),
			...(ogImage ? { images: [{ url: ogImage }] } : {}),
		},
		twitter: {
			card: ogImage ? 'summary_large_image' : 'summary',
			title: defaultTitle,
			description,
			...(ogImage ? { images: [ogImage] } : {}),
		},
		verification: {
			...(settings.google_site_verification ? { google: settings.google_site_verification } : {}),
			// Baidu has no dedicated Metadata field; it reads <meta name="baiduspider">.
			...(settings.baidu_site_verification ? { other: { baiduspider: settings.baidu_site_verification } } : {}),
		},
	};
}

/** Site-wide WebSite + SearchAction structured data. */
function websiteJsonLd(settings: Record<string, string>) {
	const base = siteBase(settings);
	const siteName = settings.site_name || 'Feed Chamber';
	/*
	 * SearchAction 的 urlTemplate 按 Google 规范应为绝对 URL，故优先用 base。
	 * 但「首页 MUST 声明站内搜索入口」是无条件的：base 为空（部署未配 site_url）时
	 * 退回相对路径，不让整块搜索入口消失——缺结构化数据的代价比 URL 形式不完美更大。
	 */
	return {
		'@context': 'https://schema.org',
		'@type': 'WebSite',
		name: siteName,
		...(base ? { url: base } : {}),
		description: settings.seo_description || settings.site_description || '',
		potentialAction: {
			'@type': 'SearchAction',
			target: { '@type': 'EntryPoint', urlTemplate: `${base}/search?q={search_term_string}` },
			'query-input': 'required name=search_term_string',
		},
	};
}

/** Runs before first paint so the dark class is applied without a flash. */
const THEME_SCRIPT = `(function(){try{var t=localStorage.getItem('theme');var m=t||'system';var isDark=m==='dark'||(m!=='light'&&window.matchMedia('(prefers-color-scheme:dark)').matches);document.documentElement.setAttribute('data-theme-mode',m);document.documentElement.classList.toggle('dark',isDark);document.documentElement.setAttribute('data-theme',isDark?'orindark':'orinlight')}catch(e){}})();`;

export default async function RootLayout({ children }: { children: React.ReactNode }) {
	// `/admin/**` 的内容全部在浏览器里取（纯 CSR），外壳的 spaces/tags 对它是纯开销，
	// 所以按 proxy 透传的路径跳过这三次请求。
	const h = await headers();
	const pathname = h.get('x-pathname') ?? '/';
	const isAdmin = pathname === '/admin' || pathname.startsWith('/admin/');

	const queryClient = getQueryClient();
	const { settings, user, spaces, tags } = isAdmin
		? { settings: {} as Record<string, string>, user: null, spaces: [], tags: { data: [], total: 0 } }
		: await loadShellData();

	/*
	 * 站点自定义层（P1）。
	 *
	 * head 段在服务端内联，body_end 段交给客户端组件延迟注入：放在服务端
	 * 意味着首屏就有，但会拖慢 TTFB；放客户端意味着不影响首屏，代价是脚本
	 * 略晚一点执行。CSS 与统计代码这类典型用途都能接受这个顺序。
	 *
	 * 后台不注入：/admin 是纯 CSR 且与前台共用 origin，注入站长写的 JS 等于
	 * 让任意脚本在后台上下文里跑，风险面不必要地放大。
	 */
	const customization = isAdmin
		? { theme_tokens: [], head_snippets: [], body_snippets: [] }
		: await loadPublicCustomization();
	const themeCss = buildThemeOverrideCss(customization.theme_tokens);
	// 片段按 kind 选容器，且 html 只落 body_end：三类内容进不同的解析状态
	// （css→<style>、js→<script>、html→容器 div），混装会既失效又可能截断容器。
	const headSnippets = emitSnippets(customization.head_snippets, 'head');
	const bodySnippets = emitSnippets(customization.body_snippets, 'body_end');

	queryClient.setQueryData(qk.settings, settings);
	queryClient.setQueryData(qk.spaces.tree, spaces);
	queryClient.setQueryData(qk.tags.list({ page: 1, page_size: 50 }), tags);
	// 自定义配置也要 seed：否则首屏 HomeModules/CustomNav 读到 pending 直接
	// 返回 null（区块先消失再出现），并且各自再发一次与 RSC 相同的请求。
	queryClient.setQueryData(qk.customization, customization);
	if (user) queryClient.setQueryData(qk.profile, user);

	return (
		<html lang="zh-CN" suppressHydrationWarning>
			<head>
				{/* 基线 __root.tsx 有 <link rel=manifest>；public/manifest.webmanifest 一直在
				    被构建复制却没被引用，等于图标资源白占体积。补回引用，PWA 安装能力归 D7。 */}
				<link href="/manifest.webmanifest" rel="manifest" />
				{/* 主题须在首绘前落地，内联脚本是唯一无闪烁方案；
				    本文件的 innerHTML 规则在 biome.json 的 overrides 里按文件关闭（受控注入点）。 */}
				<script dangerouslySetInnerHTML={{ __html: THEME_SCRIPT }} />
				{/*
				 * 主题 token 覆盖：必须排在 globals.css 之后才能盖住默认值。
				 * 只输出 CSS 变量声明，选择器由 buildThemeOverrideCss 固定。
				 */}
				{themeCss && <style id="fc-theme-overrides" dangerouslySetInnerHTML={{ __html: themeCss }} />}
				{/*
				 * 站长自定义 CSS：这是**有意允许**的任意样式注入（P1 需求）。
				 * 与 snippets 里的 JS 一样，风险由「仅 admin 可写 + 审计 + 机器写入走草稿」控制，
				 * 不在渲染层做过滤——过滤会让站长没法写 .

复杂选择器。
				 */}
				{/*
				 * 站长自定义片段（head 段）：**有意允许**的任意内容。
				 * 容器按 kind 决定，见 lib/customization/emit.ts；机器凭证写入的
				 * JS/HTML 在服务端被强制存草稿，需人工启用后才到这里。
				 */}
				{headSnippets.map((snippet) => (
					<SnippetSlot key={snippet.id} snippet={snippet} />
				))}
			</head>
			<body className="relative min-h-screen">
				{/* WebSite + SearchAction：全站一份，只放首页语义，子页不重复注入。 */}
				{!isAdmin && <JsonLd data={websiteJsonLd(settings)} />}
				<Providers dehydratedState={dehydrate(queryClient)} initialUser={user}>
					{children}
				</Providers>
				{/*
				 * body_end 段：放在最后，不阻塞首屏渲染。
				 * html 片段进容器 div（塞进 <script> 只会变成语法错误），
				 * js 片段进 <script>，css 片段进 <style>。
				 */}
				{bodySnippets.map((snippet) => (
					<SnippetSlot key={snippet.id} snippet={snippet} />
				))}
			</body>
		</html>
	);
}
