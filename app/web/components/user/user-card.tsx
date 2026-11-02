'use client';

import type { User } from '@feedchamber/api-client';
import dayjs from 'dayjs';
import { Avatar, AvatarFallback, AvatarImage } from '~/components/ui/avatar';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';

interface UserCardProps {
	user: User;
	isFollowing: boolean;
	canFollow: boolean;
	selectable: boolean;
	selected: boolean;
	onToggleFollow: () => void;
	onToggleSelect: () => void;
}

/** 用户卡（旧 -modules/UserCard.tsx）：`img` → Avatar 原语、`badge badge-xs` → Badge、`li.card` → Card。 */
export function UserCard({
	user,
	isFollowing,
	canFollow,
	selectable,
	selected,
	onToggleFollow,
	onToggleSelect,
}: UserCardProps) {
	const initial = (user.display_name || user.username || '?').slice(0, 1);

	return (
		<li className="flex list-none items-center gap-3 px-4 py-3 transition-colors hover:bg-muted/50">
			{selectable ? (
				<input
					type="checkbox"
					aria-label={`选择 ${user.display_name || user.username}`}
					checked={selected}
					onChange={onToggleSelect}
					className="size-4 shrink-0 cursor-pointer accent-primary"
				/>
			) : null}
			<Avatar className="size-10 shrink-0">
				{user.avatar ? <AvatarImage src={user.avatar} alt="" /> : null}
				<AvatarFallback className="text-xs">{initial}</AvatarFallback>
			</Avatar>
			<div className="min-w-0 flex-1">
				<div className="truncate text-sm font-medium">{user.display_name}</div>
				<div className="truncate text-xs text-muted-foreground">{user.bio || '这个人很懒，什么都没留下。'}</div>
				<div className="mt-0.5 flex gap-3 text-xs text-muted-foreground/60">
					<span>注册于 {dayjs(user.created_at).format('YYYY-MM-DD')}</span>
					{user.role ? (
						<Badge variant="outline" className="h-5 px-1.5 text-[11px]">
							{user.role}
						</Badge>
					) : null}
				</div>
			</div>
			{canFollow ? (
				<Button size="sm" variant={isFollowing ? 'outline' : 'default'} className="shrink-0" onClick={onToggleFollow}>
					{isFollowing ? '已关注' : '关注'}
				</Button>
			) : null}
		</li>
	);
}
