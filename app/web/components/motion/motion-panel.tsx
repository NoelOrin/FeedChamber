'use client';

import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';

import { MOTION_TYPE_LABEL, motionListOptions } from '~/components/motion/queries';

const MAX_ROWS = 4;

/**
 * 侧栏「进行中动议」面板（任务 6.3）。匿名/未登录也会渲染：动议列表接口
 * 需要 motion:create，401 时 useQuery 失败，这里静默退化为不显示。
 */
export function MotionPanel() {
	const { data, isError } = useQuery({
		...motionListOptions({ status: 'pending', page: 1, pageSize: MAX_ROWS }),
		retry: false,
	});
	const rows = data?.items ?? [];

	if (isError || rows.length === 0) return null;

	return (
		<nav aria-label="进行中动议" className="flex flex-col gap-1">
			<div className="flex items-center justify-between px-2 pb-1">
				<Link
					href="/motions"
					className="text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
				>
					进行中动议
				</Link>
				<span className="text-[11px] text-muted-foreground tabular-nums">{rows.length}</span>
			</div>
			<ul className="flex flex-col gap-0.5">
				{rows.map((m) => (
					<li key={m.id}>
						<Link
							href="/motions"
							title={m.reason}
							className="flex h-8 items-center gap-1.5 rounded-md px-2 text-[13px] text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
						>
							<span className="shrink-0 text-[11px]">{MOTION_TYPE_LABEL[m.type] ?? m.type}</span>
							<span className="truncate">{m.reason || m.target_id}</span>
						</Link>
					</li>
				))}
			</ul>
		</nav>
	);
}
