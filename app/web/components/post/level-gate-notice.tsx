'use client';

import { Alert, AlertDescription, AlertTitle } from '~/components/ui/alert';
import { useAuthStore } from '~/lib/auth';
import { trustLevelName, useTrustLevels } from '~/lib/hooks/use-trust-levels';
import { Lock } from '~/lib/icons';

/** 受限内容详情：403 提示所需等级、当前等级与升级进度（不伪造 404）。 */
export function LevelGateNotice({ message }: { message: string }) {
	const user = useAuthStore((s) => s.user);
	const levels = useTrustLevels();
	const required = Number(message.match(/需要等级\s*(\d+)/)?.[1] ?? 0);
	const current = user?.trust_level ?? 0;
	const xp = user?.total_xp ?? 0;
	const currentLevel = levels.find((l) => l.level === current);
	const next = levels.filter((l) => l.level > current).sort((a, b) => a.level - b.level)[0];
	const span = next && currentLevel ? next.min_xp - currentLevel.min_xp : 0;
	const gained = next && currentLevel ? Math.max(0, xp - currentLevel.min_xp) : 0;
	const percent = span > 0 ? Math.min(100, Math.round((gained / span) * 100)) : 0;

	return (
		<div className="mx-auto flex w-full max-w-[680px] flex-col gap-3">
			<Alert variant="warning">
				<Lock aria-hidden="true" />
				<div>
					<AlertTitle>该内容需要等级 {required} 才能查看</AlertTitle>
					<AlertDescription>
						当前等级 {current}（{trustLevelName(levels, current) || '游客'}）
						{next ? ` · 还差 ${Math.max(0, next.min_xp - xp)} XP 升级到 ${next.name}` : ''}
					</AlertDescription>
				</div>
			</Alert>
			{next ? (
				<div
					className="bg-muted h-2 w-full overflow-hidden rounded-full"
					role="progressbar"
					aria-valuenow={percent}
					aria-valuemin={0}
					aria-valuemax={100}
				>
					<div className="bg-primary h-full rounded-full" style={{ width: `${percent}%` }} />
				</div>
			) : null}
		</div>
	);
}
