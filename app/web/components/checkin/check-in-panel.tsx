'use client';

import { checkinService } from '@orinforum/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { useMemo, useState } from 'react';
import { Alert, AlertDescription } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { checkinHistoryOptions, checkinStatusOptions } from '~/components/user/queries';
import { ChevronLeft, ChevronRight } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { cn } from '~/lib/utils';

const WEEKDAYS = ['日', '一', '二', '三', '四', '五', '六'];

function daysInMonth(year: number, month: number): number {
	return dayjs(new Date(year, month - 1)).daysInMonth();
}

function firstWeekday(year: number, month: number): number {
	return dayjs(new Date(year, month - 1)).day();
}

interface CheckInPanelProps {
	className?: string;
}

/** 签到打卡：统计行 + 签到按钮 + 月历。 */
export function CheckInPanel({ className }: CheckInPanelProps) {
	const now = dayjs();
	const queryClient = useQueryClient();
	const [year, setYear] = useState(now.year());
	const [month, setMonth] = useState(now.month() + 1);
	const [justChecked, setJustChecked] = useState<string | null>(null);
	const [lastResult, setLastResult] = useState<{ points: number; streak: number } | null>(null);
	const [error, setError] = useState<string | null>(null);

	const { data: status } = useQuery(checkinStatusOptions());
	const { data: history } = useQuery(checkinHistoryOptions(year, month));

	const checkedDates = useMemo(() => {
		const set = new Set((history?.items ?? []).map((record) => record.check_in_date));
		if (justChecked) set.add(justChecked);
		return set;
	}, [history, justChecked]);

	const checkIn = useMutation({
		mutationFn: () => checkinService.checkIn(),
		onMutate: () => {
			setError(null);
			setLastResult(null);
		},
		onSuccess: (res) => {
			setLastResult({ points: res.points_earned, streak: res.streak });
			setJustChecked(now.format('YYYY-MM-DD'));
			void queryClient.invalidateQueries({ queryKey: qk.checkin.status });
			void queryClient.invalidateQueries({ queryKey: checkinHistoryOptions(year, month).queryKey });
			void queryClient.invalidateQueries({ queryKey: qk.home });
			void queryClient.invalidateQueries({ queryKey: qk.profile });
		},
		onError: (err) => {
			const httpStatus = (err as { status?: number }).status;
			setError(httpStatus === 409 ? '今天已签到' : (err as Error)?.message || '签到失败');
		},
	});

	const isToday = (d: number) => year === now.year() && month === now.month() + 1 && d === now.date();
	const fmt = (d: number) => `${year}-${String(month).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
	const isCurrentMonth = year === now.year() && month === now.month() + 1;
	const canGoPrev = !isCurrentMonth;

	const goPrevMonth = () => {
		if (!canGoPrev) return;
		if (month === 1) {
			setYear(year - 1);
			setMonth(12);
		} else setMonth(month - 1);
	};

	const goNextMonth = () => {
		if (month === 12) {
			setYear(year + 1);
			setMonth(1);
		} else setMonth(month + 1);
	};

	const stats = [
		{ key: 'streak', label: '连续', value: status?.current_streak ?? '-' },
		{ key: 'max', label: '最长', value: status?.max_streak ?? '-' },
		{ key: 'total', label: '累计', value: status?.total_checkins ?? '-' },
		{ key: 'points', label: '经验', value: status?.total_xp ?? '-' },
	];

	return (
		<div className={cn('flex flex-col gap-3', className)}>
			{error ? (
				<Alert variant="destructive">
					<AlertDescription>{error}</AlertDescription>
				</Alert>
			) : null}

			{lastResult ? (
				<Alert variant="success">
					<AlertDescription>
						签到成功，获得 {lastResult.points} 积分 · 已连续 {lastResult.streak} 天
					</AlertDescription>
				</Alert>
			) : null}

			<div className="flex flex-col gap-5 sm:flex-row sm:justify-center sm:gap-10">
				<div className="flex shrink-0 flex-col gap-4 sm:w-44">
					<dl className="grid grid-cols-2 gap-3 sm:grid-cols-1">
						{stats.map((cell) => (
							<div key={cell.key} className="flex flex-col gap-0.5">
								<dd className="text-lg leading-tight font-semibold text-foreground tabular-nums">{cell.value}</dd>
								<dt className="text-xs text-muted-foreground">{cell.label}</dt>
							</div>
						))}
					</dl>

					{isCurrentMonth && status?.checked_in_today ? (
						<span className="inline-flex h-8 items-center justify-center rounded-md bg-muted px-3 text-xs font-medium text-muted-foreground">
							今日已签到
						</span>
					) : isCurrentMonth ? (
						<Button size="sm" loading={checkIn.isPending} onClick={() => checkIn.mutate()}>
							立即签到
						</Button>
					) : null}
				</div>

				<div className="min-w-0 flex-1 sm:max-w-sm">
					<div className="flex items-center justify-between">
						<span className="text-xs text-muted-foreground tabular-nums">
							{year} 年 {month} 月
						</span>
						<div className="flex items-center gap-1">
							<Button
								variant="ghost"
								size="icon"
								className="size-7 tap-target"
								aria-label="上一月"
								disabled={!canGoPrev}
								onClick={goPrevMonth}
							>
								<ChevronLeft className="size-3.5" aria-hidden="true" />
							</Button>
							<Button
								variant="ghost"
								size="icon"
								className="size-7 tap-target"
								aria-label="下一月"
								disabled={isCurrentMonth}
								onClick={goNextMonth}
							>
								<ChevronRight className="size-3.5" aria-hidden="true" />
							</Button>
						</div>
					</div>

					<div className="mt-2 grid grid-cols-7 gap-1 text-center text-[11px] text-muted-foreground">
						{WEEKDAYS.map((label) => (
							<div key={`weekday-${label}`} className="py-1">
								{label}
							</div>
						))}
					</div>

					<div className="grid grid-cols-7 gap-1">
						{WEEKDAYS.slice(0, firstWeekday(year, month)).map((label) => (
							<div key={`lead-${label}`} />
						))}
						{Array.from({ length: daysInMonth(year, month) }, (_, index) => index + 1).map((d) => {
							const key = fmt(d);
							const checked = checkedDates.has(key);
							const today = isToday(d);
							return (
								<div
									key={`day-${key}`}
									className={cn(
										'grid aspect-square place-items-center rounded-md text-xs tabular-nums transition-colors',
										checked
											? 'bg-primary font-medium text-primary-foreground'
											: today
												? 'border border-border font-medium text-foreground'
												: 'text-muted-foreground hover:bg-muted',
									)}
									title={`${key}${checked ? ' · 已签到' : ''}`}
								>
									{d}
								</div>
							);
						})}
					</div>
				</div>
			</div>
		</div>
	);
}
