import type { Metadata } from 'next';

/*
 * 列表页/聚合页的元数据helper。
 *
 * 这些页面此前完全没有 generateMetadata，于是全站共用根布局的默认标题与描述——
 * 对搜索引擎来说是一批同质页面。这里统一给出「页面标题 + 自指 canonical」，
 * 并把筛选/分页参数归一：同一列表的不同参数组合只有一个规范地址。
 *
 * 规范地址刻意**不带** ?tag= / ?sort_by= 等筛选参数（它们对内容集合只是投影），
 * 也不带分页（分页是同一集合的翻页）。这样爬虫不会把参数组合当成 N 个页面。
 */

export interface ListPageMetaInput {
	/** 页面标题。字符串会被根布局模板补后缀；`{ absolute }` 则原样使用 */
	title: string | { absolute: string };
	description?: string;
	/** 站点基准 URL（settings.site_url 或 env），空串表示未配置 */
	base: string;
	/** 该页面的规范路径，如 /timeline、/spaces/tech */
	path: string;
	/** 是否允许索引，默认 true */
	indexable?: boolean;
	/** 站点名（og:site_name）。Next 的 metadata 是浅合并，页面级 openGraph 会整体替换父级，故须显式转发 */
	siteName?: string;
	/** 站点分享图（og:image / twitter:image）。同上，不转发就会在页面级丢失 */
	image?: string;
}

export function listPageMetadata({
	title,
	description,
	base,
	path,
	indexable = true,
	siteName,
	image,
}: ListPageMetaInput): Metadata {
	// base 为空（部署未配 site_url）时退回相对路径：canonical 是「同一内容只有一个规范
	// 地址」的声明，不能因为站点 URL 未配置就整个消失——那时列表页与它的 ?tag= / ?sort_by=
	// 变体之间就没有任何归一信号了。与帖子页的处理保持一致。
	const canonical = `${base}${path}`;

	const flatTitle = typeof title === 'string' ? title : title.absolute;

	return {
		title,
		...(description ? { description } : {}),
		...(indexable ? {} : { robots: { index: false, follow: true } }),
		alternates: { canonical },
		/*
		 * 这里必须把站点级的 siteName 与 image 一并带上：根布局的 openGraph 在页面级
		 * openGraph 存在时会被整体替换（Next 的 metadata 合并是浅层的），
		 * 不显式转发就会出现「首页有 og:image、列表页没有」这种不一致。
		 */
		openGraph: {
			title: flatTitle,
			type: 'website',
			...(description ? { description } : {}),
			url: canonical,
			...(siteName ? { siteName } : {}),
			...(image ? { images: [{ url: image }] } : {}),
		},
	};
}

/** Site base URL: settings `site_url` wins, env is the fallback. */
export function siteBase(settings: Record<string, string>): string {
	return (settings.site_url || process.env.SITE_URL || '').replace(/\/$/, '');
}

/** 站点分享图：og_image 优先，回退 site_logo。 */
export function siteImage(settings: Record<string, string>): string {
	return settings.og_image || settings.site_logo || '';
}

/**
 * 列表页/聚合页的元数据便捷入口：从公开设置里取 base / siteName / image，
 * 调用方只关心自己的 title 与 path。
 *
 * 存在的理由：`listPageMetadata` 的 siteName 与 image 必须由调用方转发（见其注释），
 * 让 5 个页面各写一遍 `settings.site_name` / `siteImage(settings)` 既重复又容易漏，
 * 漏一个就是那个页面的 og:image 消失。
 */
export function listPageMetadataFromSettings(
	settings: Record<string, string>,
	input: {
		title: string;
		path: string;
		description?: string;
		indexable?: boolean;
		/**
		 * 传 true 时标题**跳过**根布局的 `%s — 后缀` 模板。
		 * 首页用得到：它本身就是站点标题（seo_title），再叠一次后缀会变成
		 * 「示例论坛 — 示例论坛」。
		 */
		absoluteTitle?: boolean;
	},
): Metadata {
	return listPageMetadata({
		title: input.absoluteTitle ? { absolute: input.title } : input.title,
		path: input.path,
		description: input.description ?? (settings.seo_description || settings.site_description),
		base: siteBase(settings),
		siteName: settings.site_name || 'Feed Chamber',
		image: siteImage(settings),
		indexable: input.indexable,
	});
}
