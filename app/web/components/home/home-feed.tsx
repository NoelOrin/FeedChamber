'use client';

import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';

import { Avatar, AvatarFallback, AvatarImage } from '~/components/ui/avatar';
import { Badge } from '~/components/ui/badge';
import { Card } from '~/components/ui/card';
import { EmptyState } from '~/components/ui/empty-state';
import { PageHeader } from '~/components/ui/page-header';
import { Skeleton } from '~/components/ui/skeleton';
import { dashboardHomeOptions } from '~/components/user/queries';
import { ArrowUpRight, Folder, Layers, Megaphone, MessageSquare, Trophy } from '~/lib/icons';

const ENTRIES = [
	{ href: '/timeline', label: '时间线', icon: Layers },
	{ href: '/messages', label: '私信', icon: MessageSquare },
	{ href: '/leaderboard', label: '荣誉榜', icon: Trophy },
	{ href: '/spaces', label: '版面', icon: Folder },
];

function Stat({ label, value }: { label: string; value: number }) {
	return (
		<div className="flex flex-col gap-0.5">
			<span className="text-lg leading-tight font-semibold text-foreground tabular-nums">{value}</span>
			<span className="text-xs text-muted-foreground">{label}</span>
		</div>
	);
}

export function HomeFeed() {
	const { data, isPending, error } = useQuery(dashboardHomeOptions());
	const user = data?.user ?? null;
	const followers = data?.follow_counts?.followers ?? 0;
	const following = data?.follow_counts?.following ?? 0;
	const announcements = data?.announcements?.items ?? [];

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader title="总览" />

			{isPending ? (
				<div className="flex flex-col gap-3">
					<Skeleton className="h-36 w-full" />
					<Skeleton className="h-36 w-full" />
				</div>
			) : error ? (
				<Card className="p-6">
					<EmptyState title="加载失败" description={(error as Error).message} />
				</Card>
			) : (
				<>
					<Card className="gap-4 p-4 sm:p-5">
						<div className="flex items-center justify-between">
							<h2 className="text-xs font-medium text-muted-foreground">我的账号</h2>
							<Link
								href="/user"
								className="inline-flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
							>
								查看账号
								<ArrowUpRight className="size-3" aria-hidden="true" />
							</Link>
						</div>

						<div className="flex flex-wrap items-center gap-4">
							<Avatar className="size-11">
								{user?.avatar ? <AvatarImage src={user.avatar} alt="" /> : null}
								<AvatarFallback className="text-sm">
									{(user?.display_name || user?.username || '?').slice(0, 1).toUpperCase()}
								</AvatarFallback>
							</Avatar>

							<div className="min-w-0 flex-1">
								<div className="truncate text-[15px] font-semibold text-foreground">
									{user?.display_name || user?.username || '—'}
								</div>
								<div className="text-xs text-muted-foreground">
									@{user?.username || '—'}
									{user?.created_at ? ` · 加入于 ${String(user.created_at).slice(0, 10)}` : ''}
								</div>
							</div>

							<div className="flex gap-6">
								<Stat label="经验" value={user?.total_xp ?? 0} />
								<Stat label="关注" value={following} />
								<Stat label="粉丝" value={followers} />
							</div>
						</div>

						{user?.bio ? <p className="text-sm leading-7 text-muted-foreground">{user.bio}</p> : null}
					</Card>

					<Card className="gap-3 p-4 sm:p-5">
						<div className="flex items-center justify-between">
							<h2 className="text-xs font-medium text-muted-foreground">社区公告</h2>
							<span className="text-xs text-muted-foreground tabular-nums">{announcements.length} 条</span>
						</div>

						{announcements.length === 0 ? (
							<EmptyState variant="default" title="暂无公告" icon={<Megaphone className="size-5" />} />
						) : (
							<ul className="divide-y divide-border/60">
								{announcements.map((ann) => (
									<li key={ann.id} className="flex items-start gap-3 py-3 first:pt-0 last:pb-0">
										<div className="min-w-0 flex-1">
											<div className="flex flex-wrap items-center gap-2">
												<span className="text-sm font-medium text-foreground">{ann.title}</span>
												{ann.is_pinned ? <Badge variant="outline">置顶</Badge> : null}
											</div>
											<p className="mt-0.5 line-clamp-2 text-xs leading-relaxed text-muted-foreground">{ann.content}</p>
										</div>
										<span className="shrink-0 text-[11px] text-muted-foreground tabular-nums">
											{ann.created_at ? String(ann.created_at).slice(5, 10) : ''}
										</span>
									</li>
								))}
							</ul>
						)}
					</Card>

					<Card className="gap-3 p-4 sm:p-5">
						<h2 className="text-xs font-medium text-muted-foreground">快捷入口</h2>
						<div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
							{ENTRIES.map(({ href, label, icon: Icon }) => (
								<Link
									key={href}
									href={href}
									className="flex items-center gap-2 rounded-md border border-border px-3 py-2.5 text-[13px] text-foreground transition-colors hover:bg-muted"
								>
									<Icon size={14} aria-hidden="true" className="text-muted-foreground" />
									{label}
								</Link>
							))}
						</div>
					</Card>
				</>
			)}
		</div>
	);
}
