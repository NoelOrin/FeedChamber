import type { PostDetail } from '@orinforum/api-client';
import { dehydrate } from '@tanstack/react-query';
import type { Metadata } from 'next';
import { redirect } from 'next/navigation';

import { postContentsOptions, postDetailOptions } from '~/components/post/detail-queries';
import { PostDetail as PostDetailClient } from '~/components/post/post-detail';
import '~/lib/api/adapter';
import { requireAuthBySetting } from '~/lib/auth/guards';
import { getContentPageSize } from '~/lib/pagination';
import { loadPublicSettings } from '~/lib/queries/server';
import { getQueryClient } from '~/lib/query/client';
import { qk } from '~/lib/query/keys';
import { siteImage } from '~/lib/seo';

/*
 * /timeline/post?id=xxx —— 帖子详情（迁移计划 §3.1 / §3.4）。
 *
 * `?id=` 是硬契约（e2e 直接敲这个 URL），所以保持 query 参数而不是路径参数。
 * 首屏两跳（帖子详情 + 第一页评论）在 RSC 里 prefetchQuery，再交给 HydrationBoundary，
 * 等价旧 route loader 的 SSR 首屏。旧 loader 里「拿到 slug 就跳 /t/{slug}/{id}」的短链重定向
 * 不在本切片（/t/** 由另一条切片落地），因此这里不再跳走。
 */

type SearchParams = Record<string, string | string[] | undefined>;

interface RouteProps {
	searchParams: Promise<SearchParams>;
}

/** 旧 validateSearch 语义：重复 ?id= 取最后一个，缺省塌成空串。 */
function readId(raw: SearchParams['id']): string {
	return Array.isArray(raw) ? (raw[raw.length - 1] ?? '') : (raw ?? '');
}

/** 详情预取：generateMetadata 与页面共用同一份每请求缓存，同一次请求只发一次调用。 */
async function prefetchDetail(id: string): Promise<PostDetail | undefined> {
	const queryClient = getQueryClient();
	await queryClient.prefetchQuery(postDetailOptions(id));
	return queryClient.getQueryData<PostDetail>(qk.posts.detail(id));
}

export async function generateMetadata({ searchParams }: RouteProps): Promise<Metadata> {
	const id = readId((await searchParams).id);
	if (!id) return {};

	const settings = await loadPublicSettings();
	const siteName = settings.site_name || 'OrinForum';
	const siteDesc = settings.site_description || 'OrinForum 社区论坛';
	// 预取失败（帖子不存在 / 后端异常）不能连累元数据，页面里会渲染错误态
	const post = await prefetchDetail(id).catch(() => undefined);
	// title 只给帖子标题：根布局的 template（'%s — OrinForum'）负责补站点名，
	// 这里再拼一次会得到「X - 站点 — OrinForum — OrinForum」。og/twitter 不受 template 影响，
	// 需要完整标题，故单独拼。
	const pageTitle = post?.title;
	const fullTitle = post?.title ? `${post.title} - ${siteName}` : siteName;
	const description = post?.content_digest || siteDesc;
	const base = (settings.site_url || '').replace(/\/$/, '');
	// 规范地址是 /t/{slug}/{id}；本入口是 ?id= 查询参数（e2e 锁死），靠 canonical 收敛重复内容。
	// base 为空时退回相对路径——canonical 是「同一内容只有一个规范地址」的声明，
	// 不能因为部署没配 site_url 就整个消失（那时两个入口就都成了无 canon 的重复页）。
	const canonicalPath = `/t/${encodeURIComponent(post?.slug || 'post')}/${encodeURIComponent(id)}`;
	const canonical = `${base}${canonicalPath}`;

	return {
		title: pageTitle,
		description,
		alternates: { canonical },
		openGraph: {
			title: fullTitle,
			description,
			type: 'article',
			siteName,
			url: canonical,
			// 页面级 openGraph 会整体替换根布局的那份，站点分享图必须显式转发，
			// 否则帖子页（最需要分享卡片的页面）反而没有 og:image。
			...(siteImage(settings) ? { images: [{ url: siteImage(settings) }] } : {}),
			...(post?.created_at ? { publishedTime: post.created_at } : {}),
			...(post?.updated_at ? { modifiedTime: post.updated_at } : {}),
		},
		twitter: {
			card: siteImage(settings) ? 'summary_large_image' : 'summary',
			title: fullTitle,
			description,
			...(siteImage(settings) ? { images: [siteImage(settings)] } : {}),
		},
	};
}

export default async function PostDetailRoute({ searchParams }: RouteProps) {
	const id = readId((await searchParams).id);
	if (!id) redirect('/timeline');

	// 公开内容面的动态登录门：post_auth_required=true 时匿名访客跳登录并带回跳。
	await requireAuthBySetting('post_auth_required', `/timeline/post?id=${encodeURIComponent(id)}`);

	const [settings] = await Promise.all([loadPublicSettings(), prefetchDetail(id)]);
	const pageSize = getContentPageSize(settings);

	const queryClient = getQueryClient();
	await queryClient.prefetchQuery(postContentsOptions(id, 1, pageSize));

	return (
		<HydrationBoundary state={dehydrate(queryClient)}>
			<PostDetailClient postId={id} pageSize={pageSize} />
		</HydrationBoundary>
	);
}

import { HydrationBoundary } from '~/components/query-hydration';
