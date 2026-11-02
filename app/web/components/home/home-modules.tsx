'use client';

import type { HomeModule } from '@feedchamber/api-client';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import Link from 'next/link';

import { feedParamsOf, postListInfiniteOptions, postsOf } from '~/components/post/queries';
import { Skeleton } from '~/components/ui/skeleton';
import { publicCustomizationOptions } from '~/lib/customization/client';
import { ArrowUpRight, Folder, Layers, Trophy } from '~/lib/icons';

/**
 * 首页模块渲染（P1）。
 *
 * 站长在后台「站点自定义 → 首页模块」配置后，这里按 sort 顺序渲染到 /timeline 顶部。
 * **配置为空时不渲染任何东西**——首页保持既有时间线布局，不会出现空白区块
 * （见 design D7 的「配置优先，无配置回退」）。
 *
 * 这是客户端组件而不是 RSC：模块的可见性依赖登录态与等级，而这些在网关阶段
 * 才确定，客户端读更直接；且模块是装饰性内容，晚一帧出现可以接受。
 */

/** html 模块的内容放在 config 里，形状见 service 层校验：{"html":"..."}。 */
function htmlOf(config?: string): string {
	if (!config) return '';
	try {
		const parsed = JSON.parse(config) as { html?: string };
		return parsed.html ?? '';
	} catch {
		return '';
	}
}

function limitOf(config?: string, fallback = 5): number {
	if (!config) return fallback;
	try {
		const parsed = JSON.parse(config) as { limit?: number };
		const n = parsed.limit;
		return typeof n === 'number' && n > 0 && n <= 20 ? n : fallback;
	} catch {
		return fallback;
	}
}

function ModuleCard({ module: m }: { module: HomeModule }) {
	// html 模块：内容由站长写入，等价于一段可执行标记。
	// 这是产品决定的「有意允许任意内容」，风险由写入侧控制
	// （机器凭证写 HTML 强制草稿、仅 admin 可写、全量审计）。
	if (m.kind === 'html') {
		const html = htmlOf(m.config);
		if (!html) return null;
		return (
			<section className="rounded-md border border-border p-4">
				{m.title ? <h2 className="mb-2 text-xs font-medium text-muted-foreground">{m.title}</h2> : null}
				{/* biome-ignore lint/security/noDangerouslySetInnerHtml: 首页 html 模块是产品决定的允许项 */}
				<div dangerouslySetInnerHTML={{ __html: html }} />
			</section>
		);
	}

	if (m.kind === 'spaces') {
		return (
			<section className="rounded-md border border-border p-4">
				<div className="mb-2 flex items-center justify-between">
					<h2 className="text-xs font-medium text-muted-foreground">{m.title || '版面'}</h2>
					<Link
						href="/spaces"
						className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
					>
						全部 <ArrowUpRight size={12} aria-hidden="true" />
					</Link>
				</div>
				<div className="flex flex-wrap gap-2">
					<Link
						href="/spaces"
						className="inline-flex items-center gap-1.5 rounded-md border border-border px-3 py-2 text-[13px] hover:bg-muted"
					>
						<Folder size={14} aria-hidden="true" className="text-muted-foreground" />
						浏览版面
					</Link>
				</div>
			</section>
		);
	}

	// latest / hot：渲染真实帖子列表。
	//
	// 此前这里只放一条「查看最新帖子（最多 N 条）」的链接，limit 除了拼进文案
	// 毫无作用——站长配了模块却看不到内容。数据源与信息流共用同一套
	// query factory，命中缓存时不会额外发请求。
	return <FeedModule module={m} />;
}

/** 首页模块里的「最新 / 热门」列表。 */
function FeedModule({ module: m }: { module: HomeModule }) {
	const label = m.kind === 'latest' ? '最新' : '热门';
	const href = m.kind === 'latest' ? '/timeline' : '/timeline?sort_by=hot';
	const Icon = m.kind === 'latest' ? Layers : Trophy;
	const limit = limitOf(m.config);
	const params = feedParamsOf({ sortBy: m.kind === 'hot' ? 'hot' : null });
	const query = useInfiniteQuery(postListInfiniteOptions(params, limit));
	const posts = postsOf(query.data).slice(0, limit);

	return (
		<section className="rounded-md border border-border p-4">
			<div className="mb-2 flex items-center justify-between">
				<h2 className="text-xs font-medium text-muted-foreground">{m.title || label}</h2>
				<Link
					href={href}
					className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
				>
					更多 <ArrowUpRight size={12} aria-hidden="true" />
				</Link>
			</div>
			{query.isPending ? (
				<div className="flex flex-col gap-2">
					<Skeleton className="h-4 w-full" />
					<Skeleton className="h-4 w-3/4" />
				</div>
			) : query.isError ? (
				<p className="text-[13px] text-muted-foreground">
					列表暂时读不到，
					<Link href={href} className="underline">
						去{label}页
					</Link>
					看。
				</p>
			) : posts.length === 0 ? (
				<p className="text-[13px] text-muted-foreground">还没有{label}帖子。</p>
			) : (
				<ul className="flex flex-col gap-1.5">
					{posts.map((post) => (
						<li key={post.id}>
							<Link
								href={`/timeline/post?id=${encodeURIComponent(post.id)}`}
								className="line-clamp-1 text-[13px] text-foreground hover:underline"
							>
								{post.title}
							</Link>
						</li>
					))}
				</ul>
			)}
			<Link
				href={href}
				className="mt-2 inline-flex items-center gap-1.5 text-[13px] text-muted-foreground hover:text-foreground"
			>
				<Icon size={14} aria-hidden="true" />
				查看全部{label}
			</Link>
		</section>
	);
}

/**
 * HomeModules 渲染首页自定义模块区。
 *
 * 无配置时返回 null —— 这是「回退到既有布局」的实现方式，不是空占位。
 */
export function HomeModules() {
	const { data } = useQuery(publicCustomizationOptions);
	const modules = data?.home_modules ?? [];
	if (modules.length === 0) return null;

	return (
		<div className="mb-4 flex flex-col gap-3">
			{modules.map((m) => (
				<ModuleCard key={m.id} module={m} />
			))}
		</div>
	);
}
