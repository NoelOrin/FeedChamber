'use client';

import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';

import { userActivityOptions } from '~/components/user/queries';
import { ActivityHeatmapSkeleton } from '~/components/user/skeletons';
import { cn } from '~/lib/utils';

interface ActivityHeatmapProps {
	userId: string;
	year?: number;
	loading?: boolean;
	className?: string;
}

/* 浓度 = 主色透明度阶梯，和全站唯一的强调色保持同一套色彩语言。 */
const LEVEL_COLORS = ['bg-muted', 'bg-primary/25', 'bg-primary/45', 'bg-primary/70', 'bg-primary'];

const MONTH_LABELS = ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月'];
const DAY_LABELS = ['日', '一', '二', '三', '四', '五', '六'];
const YEAR_CHOICES = [2024, 2025, 2026];

interface WeekCell {
	id: string;
	date: string | null;
}

interface WeekCol {
	id: string;
	cells: WeekCell[];
	monthStart?: string;
}

function toCells(year: number, colIdx: number, days: (string | null)[]): WeekCell[] {
	return days.map((date, dow) => ({ id: date ?? `oob-${year}-${colIdx}-${dow}`, date }));
}

/**
 * 把一年切成以周日开头的列（含月前补位、月份起始标注）。
 * 全程按 UTC 计算：本地 `new Date(y,0,1)` + `.toISOString()` 在 UTC+8 下会把每个键前移一天，
 * 与后端 entries 的 `YYYY-MM-DD` 错位。
 */
const DAY_MS = 86_400_000;

function utcKey(ms: number) {
	return new Date(ms).toISOString().slice(0, 10);
}

function buildWeeks(year: number): WeekCol[] {
	const start = Date.UTC(year, 0, 1);
	const end = Date.UTC(year, 11, 31);
	const cols: WeekCol[] = [];
	const firstSunday = start - new Date(start).getUTCDay() * DAY_MS;

	const firstDays: (string | null)[] = new Array(7).fill(null);
	let firstColStarted = false;
	for (let d = 0; d < 7; d++) {
		const ms = firstSunday + d * DAY_MS;
		const key = utcKey(ms);
		if (ms >= start && ms <= end) {
			firstDays[d] = key;
			firstColStarted = true;
		}
	}
	const firstWeek: WeekCol = {
		id: firstDays.find((day) => day !== null) ?? `week-${year}-0`,
		cells: toCells(year, 0, firstDays),
		monthStart: MONTH_LABELS[0],
	};
	if (firstColStarted) cols.push(firstWeek);

	let next = firstSunday + 7 * DAY_MS;
	let lastMonth = 0;
	let colIdx = 1;
	while (next <= end) {
		const days: (string | null)[] = new Array(7).fill(null);
		for (let d = 0; d < 7; d++) {
			const ms = next + d * DAY_MS;
			if (ms <= end) days[d] = utcKey(ms);
		}
		const col: WeekCol = { id: `week-${year}-${colIdx}`, cells: toCells(year, colIdx, days) };
		for (const day of days) {
			if (day) {
				const m = new Date(`${day}T00:00:00Z`).getUTCMonth();
				if (m !== lastMonth) {
					col.monthStart = MONTH_LABELS[m];
					lastMonth = m;
				}
				break;
			}
		}
		col.id = days.find((day) => day !== null) ?? col.id;
		cols.push(col);
		colIdx += 1;
		next += 7 * DAY_MS;
	}
	return cols;
}

/** 365 天浓度图。 */
export function ActivityHeatmap({ userId, year: initialYear, className }: ActivityHeatmapProps) {
	const [year, setYear] = useState(initialYear ?? new Date().getFullYear());
	const [tooltip, setTooltip] = useState<{ date: string; count: number } | null>(null);

	const { data, isPending, isError } = useQuery(userActivityOptions(userId, year));
	const entries = data ?? {};

	const weeks = useMemo(() => buildWeeks(year), [year]);
	const total = useMemo(() => Object.values(entries).reduce((sum, n) => sum + n, 0), [entries]);

	/* /user/{id}/activity 是管理员向路由，普通用户读自己也 403。
	 拿不到数据时整块隐藏：画一张全灰的浓度图等于谎报「此人全年无活动」。 */
	if (isError) return null;

	if (isPending) return <ActivityHeatmapSkeleton />;

	return (
		<div className={cn('flex min-w-0 flex-col gap-3', className)}>
			<div className="flex items-center justify-between gap-2">
				<span className="text-xs text-muted-foreground">
					{year} 年 · 共 <span className="text-foreground tabular-nums">{total}</span> 次活跃
				</span>
				<select
					aria-label="活跃度年份"
					className="h-7 rounded-md border border-input bg-transparent px-2 text-xs text-foreground outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/80"
					value={year}
					onChange={(event) => setYear(Number(event.currentTarget.value))}
				>
					{YEAR_CHOICES.map((option) => (
						<option key={option} value={option}>
							{option}
						</option>
					))}
				</select>
			</div>

			{/* biome-ignore lint/a11y/noNoninteractiveTabindex: WCAG 要求可滚动区域能被键盘聚焦（axe: scrollable-region-focusable）；这里的 tabIndex 是滚动容器必需的，不是把 div 伪装成按钮 */}
			<div className="overflow-x-auto" tabIndex={0} role="region" aria-label={`${year} 年活跃度日历（可横向滚动）`}>
				<div className="mb-1 ml-6 flex gap-[3px]">
					{weeks.map((week) => (
						<div key={week.id} className="w-[13px] shrink-0">
							{week.monthStart ? (
								<span className="block whitespace-nowrap text-[11px] leading-none text-muted-foreground">
									{week.monthStart}
								</span>
							) : null}
						</div>
					))}
				</div>

				<div className="flex">
					<div className="mr-1.5 flex flex-col gap-[3px]">
						{DAY_LABELS.map((label, index) => (
							<div key={`day-${label}`} className="flex h-[13px] items-center">
								<span className="w-4 text-right text-[11px] leading-none text-muted-foreground">
									{index % 2 === 1 ? label : ''}
								</span>
							</div>
						))}
					</div>

					{/* 365 个格子不是可点目标：13px 的方块达不到 WCAG 2.2 的 24px 触控下限，
					    逐个可点也只会让键盘用户 Tab 一整年。整块按「一张图」暴露给读屏，
					    悬停提示作为鼠标端的补充信息保留。 */}
					<div className="flex gap-[3px]" role="img" aria-label={`${year} 年活跃度日历，共 ${total} 次活跃`}>
						{weeks.map((week) => (
							<div key={week.id} className="flex flex-col gap-[3px]">
								{week.cells.map((cell) => {
									if (!cell.date) {
										return <div key={cell.id} className="size-[13px]" aria-hidden="true" />;
									}
									const count = entries[cell.date] ?? 0;
									const level = count === 0 ? 0 : Math.min(Math.ceil(count / 2), 4);
									const date = cell.date;
									return (
										<button
											key={cell.id}
											type="button"
											title={`${date}：${count} 次活跃`}
											aria-label={`${date}：${count} 次活跃`}
											onMouseEnter={() => setTooltip({ date, count })}
											onMouseLeave={() => setTooltip(null)}
											onFocus={() => setTooltip({ date, count })}
											onBlur={() => setTooltip(null)}
											className={cn('size-[13px] rounded-[3px] transition-colors', LEVEL_COLORS[level])}
										/>
									);
								})}
							</div>
						))}
					</div>
				</div>
			</div>

			<div className="flex items-center justify-between text-[11px] text-muted-foreground">
				<span className="tabular-nums">{tooltip ? `${tooltip.date} · ${tooltip.count} 次活跃` : ''}</span>
				<span className="flex items-center gap-1">
					少
					{LEVEL_COLORS.map((level) => (
						<span key={level} className={cn('size-[11px] rounded-[2px]', level)} />
					))}
					多
				</span>
			</div>
		</div>
	);
}
