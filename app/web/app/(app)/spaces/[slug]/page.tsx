import type { Space } from '@feedchamber/api-client';
import { spaceService } from '@feedchamber/api-client';
import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import {
	dehydrateFeedQuery,
	feedParamsOf,
	normalizeSortBy,
	type PostFeedData,
	postListInfiniteOptions,
	readParam,
	stableFeedKey,
	TIMELINE_QUERY_KEYS,
} from '~/components/post/queries';
import { HydrationBoundary } from '~/components/query-hydration';
import { JsonLd } from '~/components/seo/json-ld';
import { flattenSpaceTree } from '~/components/space/space-card';
import { SpaceHome } from './space-home';
import '~/lib/api/adapter';
import { requireAuthBySetting } from '~/lib/auth/guards';
import { getArticlePageSize } from '~/lib/pagination';
import { loadPublicSettings, loadSpaceTree } from '~/lib/queries/server';
import { getQueryClient } from '~/lib/query/client';
import { listPageMetadataFromSettings, siteBase } from '~/lib/seo';

/** 版面页的 BreadcrumbList（首页 → 版面）。 */
function spaceBreadcrumb(name: string, slug: string, base: string) {
	// base 为空（部署未配 site_url）时退回相对 URL，结构化数据仍然输出。
	return {
		'@context': 'https://schema.org',
		'@type': 'BreadcrumbList',
		itemListElement: [
			{ '@type': 'ListItem', position: 1, name: '首页', item: base || '/' },
			{ '@type': 'ListItem', position: 2, name, item: `${base}/spaces/${encodeURIComponent(slug)}` },
		],
	};
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
	const { slug } = await params;
	const [settings, tree] = await Promise.all([loadPublicSettings(), loadSpaceTree()]);
	const node = flattenSpaceTree(tree).find((item) => item.slug === slug);
	// 不存在的版面交给页面 notFound()，元数据这里不索引即可。
	if (!node) return { title: '版面', robots: { index: false, follow: false } };

	return listPageMetadataFromSettings(settings, {
		title: node.name,
		description: node.description || settings.site_description,
		path: `/spaces/${encodeURIComponent(node.slug)}`,
	});
}

/*
 * /spaces/{slug} —— 单版首页的 RSC 侧。
 *
 * 版面信息来自外壳级版面树（根 layout 已取过一次，同一请求内由 cache() 复用），
 * 按 slug 反查不到就走 notFound()：旧站是渲染出一张空标题卡，这里换成 404 页更符合 App Router。
 * 面包屑在服务端取好，客户端首帧就能渲染出正确文案，不再有一次闪烁。
 * （版面关注状态此前也在这里取，2026-10-28 随「关注版面」一并移除。）
 */

type SearchParams = Record<string, string | string[] | undefined>;

export default async function SpaceDetailPage({
	params,
	searchParams,
}: {
	params: Promise<{ slug: string }>;
	searchParams: Promise<SearchParams>;
}) {
	const [{ slug }, raw] = await Promise.all([params, searchParams]);

	// 公开内容面的动态登录门：space_view_auth_required=true 时匿名访客跳登录并带回跳。
	await requireAuthBySetting('space_view_auth_required', `/spaces/${encodeURIComponent(slug)}`);

	const sortBy = normalizeSortBy(readParam(raw[TIMELINE_QUERY_KEYS.sortBy]));

	const tree = await loadSpaceTree();
	const node = flattenSpaceTree(tree).find((item) => item.slug === slug);
	if (!node) notFound();

	const [ancestors, settings] = await Promise.all([
		spaceService.breadcrumb(node.id).catch((): Space[] => []),
		loadPublicSettings(),
	]);

	// 单版首页聚合整棵子版面树：父版要能看到子版里的帖子（后端 include_sub 递归）。
	// 只传 space_id + include_sub，不再传 space_slug——后端 space_id 优先，slug 会被忽略。
	const feedParams = feedParamsOf({ spaceId: node.id, includeSub: true, sortBy });
	const pageSize = getArticlePageSize(settings);
	const options = postListInfiniteOptions(feedParams, pageSize);

	const queryClient = getQueryClient();
	await queryClient.prefetchInfiniteQuery(options);
	const seed = queryClient.getQueryData<PostFeedData>(options.queryKey);

	return (
		<>
			<JsonLd data={spaceBreadcrumb(node.name, node.slug, siteBase(settings))} />
			<HydrationBoundary state={dehydrateFeedQuery(queryClient, options.queryKey)}>
				<SpaceHome
					space={{
						id: node.id,
						slug: node.slug,
						name: node.name,
						icon: node.icon ?? '',
						description: node.description ?? '',
					}}
					trail={ancestors.filter((item) => item.id !== node.id).map((item) => ({ slug: item.slug, name: item.name }))}
					subSpaces={(node.children ?? []).map((child) => ({
						slug: child.slug,
						name: child.name,
						icon: child.icon,
						description: child.description,
					}))}
					initialPage={seed?.pages?.[0] ?? null}
					initialPageKey={stableFeedKey(feedParams, pageSize)}
				/>
			</HydrationBoundary>
		</>
	);
}
