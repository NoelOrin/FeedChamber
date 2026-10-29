'use client';

import { useMutation, useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useState } from 'react';

import { Avatar, AvatarFallback, AvatarImage } from '~/components/ui/avatar';
import { Button } from '~/components/ui/button';
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from '~/components/ui/dropdown-menu';
import { dashboardHomeOptions } from '~/components/user/queries';
import { authClient, useAuthStore } from '~/lib/auth';
import { Bookmark, LogOut, MessageSquare, Settings, User } from '~/lib/icons';
import { toast } from '~/lib/toast';

/* 未登录态的头像轮廓（旧 UserInfo 内联 SVG，非 lucide，保留原描边） */
function LoginGlyph() {
	return (
		<svg fill="none" height="16" viewBox="0 0 48 48" width="16" xmlns="http://www.w3.org/2000/svg">
			<path
				d="M24 24C28.4183 24 32 20.4183 32 16C32 11.5817 28.4183 8 24 8C19.5817 8 16 11.5817 16 16C16 20.4183 19.5817 24 24 24Z"
				fill="none"
				stroke="currentColor"
				strokeLinejoin="round"
				strokeWidth="4"
			/>
			<path
				d="M42 44C42 34.0589 33.9411 26 24 26C14.0589 26 6 34.0589 6 44"
				stroke="currentColor"
				strokeLinecap="round"
				strokeLinejoin="round"
				strokeWidth="4"
			/>
		</svg>
	);
}

export function UserInfo() {
	const router = useRouter();
	const user = useAuthStore((state) => state.user);
	const loggedIn = useAuthStore((state) => state.status === 'authenticated' && state.user !== null);
	const [open, setOpen] = useState(false);

	// 关注数：登录后且菜单真的展开时才取，复用 dashboardHomeOptions() 的同一份 queryFn。
	const { data: dashboard } = useQuery({ ...dashboardHomeOptions(), enabled: loggedIn && open });
	const followCounts = dashboard?.follow_counts ?? null;

	const logout = useMutation({ mutationFn: () => authClient.logout() });

	function handleLogout() {
		// 旧实现：接口失败也照样清本地凭证 + 提示 + 回首页，故用 onSettled
		logout.mutate(undefined, {
			onSettled: () => {
				toast.success('已退出登录');
				router.push('/');
			},
		});
	}

	if (!loggedIn) {
		return (
			<Button asChild variant="ghost" size="icon" title="登录" aria-label="登录">
				<Link href="/login">
					<LoginGlyph />
				</Link>
			</Button>
		);
	}

	const displayName = user?.display_name || user?.username || '用户';
	const initial = displayName.slice(0, 1).toUpperCase();

	return (
		<DropdownMenu open={open} onOpenChange={setOpen}>
			<DropdownMenuTrigger asChild>
				<button
					type="button"
					aria-label="账号菜单"
					className="ml-0.5 inline-flex cursor-pointer items-center rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring/80"
				>
					<Avatar className="size-7">
						{user?.avatar ? <AvatarImage src={user.avatar} alt="" /> : null}
						<AvatarFallback className="text-[11px]">{initial}</AvatarFallback>
					</Avatar>
				</button>
			</DropdownMenuTrigger>

			<DropdownMenuContent align="end" className="w-56">
				<DropdownMenuLabel className="py-1.5">
					<span className="block truncate text-[13px] font-medium text-foreground">{displayName}</span>
					{user?.username ? (
						<span className="block truncate text-xs text-muted-foreground">@{user.username}</span>
					) : null}
				</DropdownMenuLabel>

				<div className="mb-1 grid grid-cols-2 gap-1 px-1">
					<Link
						href="/user/follow?tab=following&q=&page=1"
						className="rounded-md px-2 py-1.5 text-center transition-colors hover:bg-muted"
					>
						<span className="block text-[13px] font-semibold text-foreground">{followCounts?.following ?? 0}</span>
						<span className="block text-[11px] text-muted-foreground">关注</span>
					</Link>
					<Link
						href="/user/follow?tab=followers&q=&page=1"
						className="rounded-md px-2 py-1.5 text-center transition-colors hover:bg-muted"
					>
						<span className="block text-[13px] font-semibold text-foreground">{followCounts?.followers ?? 0}</span>
						<span className="block text-[11px] text-muted-foreground">粉丝</span>
					</Link>
				</div>

				<DropdownMenuSeparator />

				<DropdownMenuItem asChild>
					<Link href="/user">
						<User /> 个人中心
					</Link>
				</DropdownMenuItem>
				<DropdownMenuItem asChild>
					<Link href="/user/favorites?page=1">
						<Bookmark /> 我的收藏
					</Link>
				</DropdownMenuItem>
				<DropdownMenuItem asChild>
					<Link href="/messages">
						<MessageSquare /> 私信
					</Link>
				</DropdownMenuItem>
				<DropdownMenuItem asChild>
					<Link href="/user/notifications?type=all&read=all&page=1">
						<Settings /> 消息通知
					</Link>
				</DropdownMenuItem>

				<DropdownMenuSeparator />

				<DropdownMenuItem variant="destructive" disabled={logout.isPending} onSelect={handleLogout}>
					<LogOut /> 退出登录
				</DropdownMenuItem>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
