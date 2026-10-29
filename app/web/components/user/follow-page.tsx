'use client';

import { followService } from '@orinforum/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { parseAsInteger, parseAsString, useQueryState } from 'nuqs';
import { useMemo, useState } from 'react';

import { Button } from '~/components/ui/button';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { Segmented } from '~/components/ui/segmented';
import { BatchActionBar } from '~/components/user/batch-action-bar';
import {
	type FollowTab,
	followingIdsOptions,
	followListOptions,
	toFirstValue,
	totalPages,
	userSearchOptions,
} from '~/components/user/queries';
import { SkeletonList } from '~/components/user/skeletons';
import { UserCardList } from '~/components/user/user-card-list';
import { DEFAULT_PAGE_SIZE } from '~/lib/pagination';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const TABS: { value: FollowTab; label: string }[] = [
	{ value: 'following', label: '关注' },
	{ value: 'followers', label: '粉丝' },
];

/** 关注与粉丝（旧 pages/(app)/user/follow.tsx）：?tab= &q= &page= 走 nuqs。 */
export function FollowPage({ userId }: { userId: string }) {
	const queryClient = useQueryClient();
	const [tabParam, setTab] = useQueryState('tab', parseAsString.withDefault('following'));
	const [qParam, setQ] = useQueryState('q', parseAsString.withDefault(''));
	const [page, setPage] = useQueryState('page', parseAsInteger.withDefault(1));

	const tab: FollowTab = tabParam === 'followers' ? 'followers' : 'following';
	const q = toFirstValue(qParam, '');

	const [searchInput, setSearchInput] = useState(q);
	const [batchMode, setBatchMode] = useState(false);
	const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set<string>());

	/* 旧 createEffect(loaderData) 的等价：URL（tab/q/page）一变就清空多选并回填输入框。
	  用「渲染期对齐 epoch」而不是 effect，避免多渲染一帧后才清选。 */
	const epoch = `${tab}|${q}|${page}`;
	const [seenEpoch, setSeenEpoch] = useState(epoch);
	if (seenEpoch !== epoch) {
		setSeenEpoch(epoch);
		setSelectedIds(new Set<string>());
		setSearchInput(q);
	}

	const followIdsQuery = useQuery(followingIdsOptions(userId));
	const searchQuery = useQuery({ ...userSearchOptions(q, page), enabled: q.length > 0 });
	const listQuery = useQuery({ ...followListOptions(userId, tab, page), enabled: q.length === 0 });

	const users = q.length > 0 ? (searchQuery.data?.list ?? []) : (listQuery.data?.items ?? []);
	/* 旧 loader 把搜索结果的 total 写成 list.length（等价于恒为 1 页），此处照旧 */
	const total = q.length > 0 ? users.length : (listQuery.data?.total ?? 0);

	const followingIds = useMemo(
		() => new Set((followIdsQuery.data?.items ?? []).map((u) => u.id)),
		[followIdsQuery.data],
	);

	const isPending = q.length > 0 ? searchQuery.isPending : listQuery.isPending;

	function refreshFollowCaches() {
		void queryClient.invalidateQueries({ queryKey: qk.follow.all });
	}

	const toggle = useMutation({
		mutationFn: async ({ targetId, follow }: { targetId: string; follow: boolean }) => {
			if (follow) await followService.follow(targetId);
			else await followService.unfollow(targetId);
		},
		onSuccess: (_data, { follow }) => {
			toast.success(follow ? '已关注' : '已取消关注');
			refreshFollowCaches();
		},
		onError: () => toast.error('操作失败'),
	});

	function goPage(next: number) {
		void setPage(next);
	}

	function switchTab(next: string) {
		void setTab(next);
		void setQ('');
		void setPage(1);
		setSelectedIds(new Set<string>());
	}

	function handleSearch() {
		const next = searchInput.trim();
		void setQ(next);
		void setPage(1);
		setSelectedIds(new Set<string>());
	}

	function clearSearch() {
		setSearchInput('');
		void setQ('');
		void setPage(1);
	}

	async function batchFollow() {
		const ids = [...selectedIds].filter((id) => !followingIds.has(id));
		if (ids.length === 0) {
			toast.error('没有可关注的目标');
			return;
		}
		try {
			await Promise.all(ids.map((id) => followService.follow(id)));
			toast.success(`已关注 ${ids.length} 人`);
			setSelectedIds(new Set<string>());
			refreshFollowCaches();
		} catch {
			toast.error('批量关注失败');
		}
	}

	async function batchUnfollow() {
		const ids = [...selectedIds].filter((id) => followingIds.has(id));
		if (ids.length === 0) {
			toast.error('没有可取关的目标');
			return;
		}
		try {
			await Promise.all(ids.map((id) => followService.unfollow(id)));
			toast.success(`已取关 ${ids.length} 人`);
			setSelectedIds(new Set<string>());
			refreshFollowCaches();
		} catch {
			toast.error('批量取关失败');
		}
	}

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader
				title="关注与粉丝"
				meta={`${total} 人`}
				actions={
					<Button
						variant={batchMode ? 'default' : 'outline'}
						size="sm"
						aria-pressed={batchMode}
						onClick={() => {
							setBatchMode((v) => !v);
							setSelectedIds(new Set<string>());
						}}
					>
						批量操作
					</Button>
				}
			/>

			<Segmented label="关注列表筛选" value={tab} options={TABS} onValueChange={switchTab} />

			<div className="flex gap-2">
				<Input
					type="text"
					placeholder="搜索用户名…"
					value={searchInput}
					onChange={(event) => setSearchInput(event.currentTarget.value)}
					onKeyDown={(event) => {
						if (event.key === 'Enter') handleSearch();
					}}
				/>
				<Button size="md" onClick={handleSearch}>
					搜索
				</Button>
				{q ? (
					<Button variant="ghost" size="md" onClick={clearSearch}>
						清除
					</Button>
				) : null}
			</div>

			{batchMode && selectedIds.size > 0 ? (
				<BatchActionBar
					selectedCount={selectedIds.size}
					totalCount={users.length}
					onSelectAll={() => setSelectedIds(new Set(users.map((u) => u.id)))}
					onDeselectAll={() => setSelectedIds(new Set<string>())}
					onBatchFollow={() => void batchFollow()}
					onBatchUnfollow={() => void batchUnfollow()}
				/>
			) : null}

			{isPending ? (
				<ul className="m-0 flex list-none flex-col gap-2 p-0">
					<SkeletonList count={5} type="user-card" />
				</ul>
			) : (
				<UserCardList
					users={users}
					followingIds={followingIds}
					currentUserId={userId}
					batchMode={batchMode}
					selectedIds={selectedIds}
					onToggleSelect={(id) =>
						setSelectedIds((prev) => {
							const next = new Set(prev);
							if (next.has(id)) next.delete(id);
							else next.add(id);
							return next;
						})
					}
					onToggleFollow={(id) => toggle.mutate({ targetId: id, follow: !followingIds.has(id) })}
					currentPage={page}
					totalPages={totalPages(total, DEFAULT_PAGE_SIZE)}
					onPageChange={goPage}
				/>
			)}
		</div>
	);
}
