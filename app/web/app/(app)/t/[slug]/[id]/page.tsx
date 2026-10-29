import type { PostDetail as PostDetailData } from '@feedchamber/api-client';
import { dehydrate } from '@tanstack/react-query';
import type { Metadata } from 'next';
import { notFound, redirect } from 'next/navigation';

import { postContentsOptions, postDetailOptions } from '~/components/post/detail-queries';
import { PostDetail as PostDetailClient } from '~/components/post/post-detail';
import { JsonLd } from '~/components/seo/json-ld';
import '~/lib/api/adapter';
import { requireAuthBySetting } from '~/lib/auth/guards';
import { getContentPageSize } from '~/lib/pagination';
import { loadPublicSettings } from '~/lib/queries/server';
import { getQueryClient } from '~/lib/query/client';
import { qk } from '~/lib/query/keys';
import { siteImage } from '~/lib/seo';

/*
 * `/t/{slug}/{id}` —— 帖子详情的**规范地址**（openspec change 2026-10-28-seo-public-indexing）。
 *
 * 旧版这里只做 redirect 到 `/timeline/post?id=`，于是 sitemap 里全是 307、爬虫拿不到 200。
 * 现在直接渲染正文，URL 形态不变（站外链接与 sitemap 都指这里），slug 仅用于 SEO。
 *
 * `/timeline/post?id=` 仍然保留（e2e 锁死该查询参数），它的 canonical 指回本路径，
 * 从而把同一内容的两个入口收敛到一个规范地址。
 */

type Params = Promise<{ slug: string; id: string }>;

/** 详情预取：generateMetadata 与页面共用同一份每请求缓存，同一次请求只发一次调用。 */
async function prefetchDetail(id: string): Promise<PostDetailData | undefined> {
	const queryClient = getQueryClient();
	await queryClient.prefetchQuery(postDetailOptions(id));
	return queryClient.getQueryData<PostDetailData>(qk.posts.detail(id));
}

function canonicalPath(post: PostDetailData | undefined, id: string): string {
	const slug = post?.slug || 'post';
	return `/t/${encodeURIComponent(slug)}/${encodeURIComponent(id)}`;
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
	const { id } = await params;
	if (!id) return {};

	const settings = await loadPublicSettings();
	const siteName = settings.site_name || 'Feed Chamber';
	const siteDesc = settings.site_description || 'Feed Chamber 社区论坛';
	const base = (settings.site_url || '').replace(/\/$/, '');
	const post = await prefetchDetail(id).catch(() => undefined);

	if (!post) {
		// 帖子不存在/未公开：不让它进索引，页面里会渲染错误态。
		return { title: siteName, robots: { index: false, follow: false } };
	}

	const title = `${post.title} - ${siteName}`;
	const description = post.content_digest || siteDesc;
	const url = `${base}${canonicalPath(post, id)}`;

	return {
		title: post.title,
		description,
		alternates: { canonical: url },
		openGraph: {
			title,
			description,
			type: 'article',
			url,
			siteName,
			// 页面级 openGraph 整体替换根布局那份，站点分享图须显式转发。
			...(siteImage(settings) ? { images: [{ url: siteImage(settings) }] } : {}),
			...(post.created_at ? { publishedTime: post.created_at } : {}),
			...(post.updated_at ? { modifiedTime: post.updated_at } : {}),
		},
		twitter: {
			card: siteImage(settings) ? 'summary_large_image' : 'summary',
			title,
			description,
			...(siteImage(settings) ? { images: [siteImage(settings)] } : {}),
		},
	};
}

/** DiscussionForumPosting + BreadcrumbList for one topic. */
function postJsonLd(post: PostDetailData, siteName: string, base: string) {
	// base 为空（部署未配 site_url）时退回相对 URL：结构化数据仍然要输出，
	// 否则「帖子页有 DiscussionForumPosting」这条要求会在未配置站点的部署里静默失效。
	const url = `${base}${canonicalPath(post, post.id)}`;
	const home = base || '/';
	const crumbs = [
		{ '@type': 'ListItem', position: 1, name: '首页', item: home },
		...(post.space_name ? [{ '@type': 'ListItem', position: 2, name: post.space_name, item: `${base}/spaces` }] : []),
		{ '@type': 'ListItem', position: post.space_name ? 3 : 2, name: post.title, item: url },
	];

	return [
		{
			'@context': 'https://schema.org',
			'@type': 'DiscussionForumPosting',
			headline: post.title,
			...(post.content_digest ? { text: post.content_digest } : {}),
			url,
			...(post.created_at ? { datePublished: post.created_at } : {}),
			...(post.updated_at ? { dateModified: post.updated_at } : {}),
			author: { '@type': 'Person', name: post.author_name || '匿名用户' },
			...(post.space_name ? { articleSection: post.space_name } : {}),
			publisher: { '@type': 'Organization', name: siteName },
			interactionStatistic: [
				{
					'@type': 'InteractionCounter',
					interactionType: 'https://schema.org/LikeAction',
					userInteractionCount: post.like_count ?? 0,
				},
				{
					'@type': 'InteractionCounter',
					interactionType: 'https://schema.org/CommentAction',
					userInteractionCount: post.comment_count ?? 0,
				},
			],
		},
		{
			'@context': 'https://schema.org',
			'@type': 'BreadcrumbList',
			itemListElement: crumbs,
		},
	];
}

export default async function PostShortLinkPage({ params }: { params: Params }) {
	const { slug, id } = await params;
	if (!id) redirect('/timeline');

	// 公开内容面的动态登录门：post_auth_required=true 时匿名访客跳登录并带回跳。
	await requireAuthBySetting('post_auth_required', `/t/${encodeURIComponent(slug)}/${encodeURIComponent(id)}`);

	const [settings, post] = await Promise.all([loadPublicSettings(), prefetchDetail(id).catch(() => undefined)]);
	// 取不到（不存在 / pending 未公开 / 后端异常）一律 404，避免爬虫把错误态当内容索引。
	if (!post) notFound();

	const pageSize = getContentPageSize(settings);
	const queryClient = getQueryClient();
	await queryClient.prefetchQuery(postContentsOptions(id, 1, pageSize));

	const base = (settings.site_url || '').replace(/\/$/, '');
	const siteName = settings.site_name || 'Feed Chamber';

	return (
		<>
			<JsonLd data={postJsonLd(post, siteName, base)} />
			<HydrationBoundary state={dehydrate(queryClient)}>
				<PostDetailClient postId={id} pageSize={pageSize} />
			</HydrationBoundary>
		</>
	);
}

import { HydrationBoundary } from '~/components/query-hydration';
