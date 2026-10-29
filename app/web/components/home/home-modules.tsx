'use client';

import type { HomeModule } from '@feedchamber/api-client';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';

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

	// latest / hot：交给调用方决定数据来源。这里只渲染容器与标题，
	// 具体列表由 timeline 自己的信息流承担——重复请求同一份列表没有意义。
	const label = m.kind === 'latest' ? '最新' : '热门';
	const href = m.kind === 'latest' ? '/timeline' : '/timeline?sort_by=hot';
	const Icon = m.kind === 'latest' ? Layers : Trophy;
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
			<Link href={href} className="inline-flex items-center gap-1.5 text-[13px] text-foreground hover:underline">
				<Icon size={14} aria-hidden="true" className="text-muted-foreground" />
				查看{label}帖子（最多 {limitOf(m.config)} 条）
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
