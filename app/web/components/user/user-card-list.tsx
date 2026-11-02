import type { User } from '@feedchamber/api-client';

import { Pager } from '~/components/pagination/pager';
import { EmptyState } from '~/components/ui/empty-state';
import { UserCard } from '~/components/user/user-card';

interface UserCardListProps {
	users: User[];
	followingIds: Set<string>;
	currentUserId?: string;
	batchMode: boolean;
	selectedIds: Set<string>;
	onToggleSelect: (id: string) => void;
	onToggleFollow: (id: string) => void;
	currentPage: number;
	totalPages: number;
	onPageChange: (page: number) => void;
}

/** 关注/粉丝列表（旧 -modules/UserCardList.tsx）。 */
export function UserCardList({
	users,
	followingIds,
	currentUserId,
	batchMode,
	selectedIds,
	onToggleSelect,
	onToggleFollow,
	currentPage,
	totalPages,
	onPageChange,
}: UserCardListProps) {
	if (users.length === 0) {
		return <EmptyState title="暂无用户" />;
	}

	return (
		<>
			<ul className="m-0 list-none divide-y divide-border/60 overflow-hidden rounded-lg border border-border bg-card p-0">
				{users.map((u) => (
					<UserCard
						key={u.id}
						user={u}
						isFollowing={followingIds.has(u.id)}
						/* 旧版这里写了两次 `u.id !== currentUserId`（重复条件），语义不变，保留单份 */
						canFollow={u.id !== currentUserId}
						selectable={batchMode}
						selected={selectedIds.has(u.id)}
						onToggleFollow={() => onToggleFollow(u.id)}
						onToggleSelect={() => onToggleSelect(u.id)}
					/>
				))}
			</ul>
			<Pager currentPage={currentPage} totalPages={totalPages} onPageChange={onPageChange} />
		</>
	);
}
