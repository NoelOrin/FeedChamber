import type { Metadata } from 'next';
import { notFound } from 'next/navigation';

import { MarkdownRenderer } from '~/components/content/markdown-renderer';
import { PageHeader } from '~/components/ui/page-header';
import { Prose } from '~/components/ui/prose';
import '~/lib/api/adapter';
import { customizeService } from '@feedchamber/api-client';
import { loadPublicSettings } from '~/lib/queries/server';

type PageProps = { params: Promise<{ slug: string }> };

/**
 * 自定义页 `/p/{slug}`（P1）。
 *
 * 只渲染已发布页面：草稿在服务端表现为 404（不是 403）——否则等于
 * 告诉访客「这里有个未发布页」。判定在 service 层，前端不再二次判断。
 */
async function loadPage(slug: string) {
	try {
		return await customizeService.getPublicPage(slug);
	} catch {
		return null;
	}
}

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
	const { slug } = await params;
	const [page, settings] = await Promise.all([loadPage(slug), loadPublicSettings()]);
	if (!page) return { title: '页面不存在' };
	// 只返回页面标题：根布局的 title.template 会补 `%s — <默认标题>`，
	// 这里再拼一次会得到「关于 — Feed Chamber — Feed Chamber」。
	return {
		title: page.title,
		description: settings.seo_description || settings.site_description || undefined,
	};
}

export default async function CustomPageRoute({ params }: PageProps) {
	const { slug } = await params;
	const page = await loadPage(slug);
	if (!page) notFound();

	return (
		<div className="mx-auto flex w-full max-w-3xl flex-col gap-4 pb-16">
			<PageHeader title={page.title} />
			<Prose>
				<MarkdownRenderer mdText={page.content} />
			</Prose>
		</div>
	);
}
