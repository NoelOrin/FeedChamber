'use client';

import type { Tag } from '@feedchamber/api-client';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';

import { Card } from '~/components/ui/card';
import { EmptyState } from '~/components/ui/empty-state';
import { PageHeader } from '~/components/ui/page-header';
import { Skeleton } from '~/components/ui/skeleton';
import { tagListOptions } from '~/lib/queries/shell';

/*
 * 标签页：轻触一枚标签 = 进入 /timeline?tag=xxx 切片。
 * 数据来自外壳级 tagListOptions()（根 layout 已水合同一 key），无需 RSC 预取。
 *
 * 标签云必须放在 Card 内部单独一层 flex-wrap 里，**不要**写 `<Card className="flex flex-wrap">`：
 * Card 基类含 `flex-col`，而 tailwind-merge 只合并同一属性组的类（`flex-wrap` 与 `flex-col`
 * 不同组），方向会保持 column —— 28 个标签会被拉成 28 条整行宽（实测每条 1412px × 28px）。
 */

const SKELETON_SLOTS = ['tag-skeleton-001', 'tag-skeleton-002', 'tag-skeleton-003', 'tag-skeleton-004'];

export function TagList() {
	const { data, isPending } = useQuery(tagListOptions());
	/*
	 * 兼容两种形态：queryFn 返回 `{ data, total }`，根 layout 把同一 key 种成了裸 Tag[]。
	 */
	const seeded = data as unknown as Tag[] | { data?: Tag[] } | undefined;
	const tags = Array.isArray(seeded) ? seeded : (seeded?.data ?? []);

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader title="标签" meta={`${tags.length} 个`} />

			{isPending || tags.length > 0 ? (
				<Card className="p-4">
					<div className="flex flex-wrap gap-2">
						{isPending
							? SKELETON_SLOTS.map((slot) => <Skeleton key={slot} className="h-7 w-20" />)
							: tags.map((tag) => (
									<Link
										key={tag.id}
										href={`/timeline?tag=${encodeURIComponent(tag.name)}`}
										className="inline-flex h-7 items-center rounded-md border border-border px-2.5 text-xs text-foreground transition-colors hover:bg-muted"
									>
										{tag.name}
									</Link>
								))}
					</div>
				</Card>
			) : (
				<EmptyState title="暂无标签" description="打标签的帖子多起来后，这里会自动汇总。" />
			)}
		</div>
	);
}
