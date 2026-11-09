'use client';

import { historyService, type PostListItem } from '@feedchamber/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import Link from 'next/link';
import { parseAsInteger, useQueryState } from 'nuqs';
import { useState } from 'react';

import { historyListOptions, totalPages } from '~/components/history/queries';
import { Pager } from '~/components/pagination/pager';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from '~/components/ui/dialog';
import { EmptyState } from '~/components/ui/empty-state';
import { PageHeader } from '~/components/ui/page-header';
import { Skeleton } from '~/components/ui/skeleton';
import { Clock, Trash2 } from '~/lib/icons';
import { DEFAULT_PAGE_SIZE } from '~/lib/pagination';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

function errText(error: unknown, fallback: string) {
	return error instanceof Error && error.message ? error.message : fallback;
}

/**
 * 浏览记录的时间：被动浏览写 last_viewed_at，升级前的存量行只有 last_read_at，
 * 后端排序用同一套 COALESCE，这里展示也照它，免得两栏说的不是同一件事。
 */
function touchedAt(item: PostListItem) {
	return item.last_viewed_at ?? item.last_read_at ?? null;
}

function HistoryRow({ post, onDelete, deleting }: { post: PostListItem; onDelete: () => void; deleting: boolean }) {
	const touched = touchedAt(post);
	return (
		<li>
			<Card hover="static" className="flex flex-row items-start justify-between gap-3 p-4">
				<div className="min-w-0">
					<Link
						href={`/timeline/post?id=${encodeURIComponent(post.id)}`}
						className="block truncate text-[15px] font-medium text-foreground hover:underline"
					>
						{post.title}
					</Link>
					<div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
						{post.space_name ? <span>{post.space_name}</span> : null}
						{touched ? (
							<span className="inline-flex items-center gap-1">
								<Clock className="size-3" aria-hidden="true" />
								<span className="tabular-nums">{dayjs(touched).format('YYYY-MM-DD HH:mm')}</span>
							</span>
						) : null}
						{/* 续读游标是「点进去会回到哪条评论」的唯一凭据，值得在这里露一次脸 */}
						{post.last_content_id ? <span>· 已记住阅读位置</span> : null}
					</div>
				</div>
				<Button size="xs" variant="ghost" loading={deleting} onClick={onDelete} aria-label="删除这条浏览记录">
					<Trash2 className="size-3.5" aria-hidden="true" />
				</Button>
			</Card>
		</li>
	);
}

function RowSkeleton() {
	return (
		<li>
			<Card hover="static" className="flex flex-row items-start justify-between gap-3 p-4">
				<div className="min-w-0 flex-1 space-y-2">
					<Skeleton className="h-3.5 w-2/3" />
					<Skeleton className="h-3 w-32" />
				</div>
				<Skeleton className="size-6 shrink-0" />
			</Card>
		</li>
	);
}

/**
 * 我的浏览记录（POST /history/list）。
 *
 * 这里列出的只是「你现在还看得见的」那些帖子——被删除、进回收站或等级不够的
 * 一律不出现（可见性过滤在 SQL 里，与列表/详情同一谓词）。因此条数小于历史总数
 * 是正常现象，不是丢数据。
 */
export function HistoryPage() {
	const queryClient = useQueryClient();
	const [page, setPage] = useQueryState('page', parseAsInteger.withDefault(1));
	const [removing, setRemoving] = useState<string | null>(null);
	const [confirmOpen, setConfirmOpen] = useState(false);

	const query = useQuery(historyListOptions(page ?? 1, DEFAULT_PAGE_SIZE));
	const posts = query.data?.data ?? [];
	const total = query.data?.total ?? 0;

	const remove = useMutation({
		mutationFn: (postId: string) => historyService.delete(postId),
		onMutate: (postId) => setRemoving(postId),
		onSuccess: () => {
			setRemoving(null);
			toast.success('已从浏览记录中移除');
			void queryClient.invalidateQueries({ queryKey: qk.history.all });
		},
		onError: (error) => {
			setRemoving(null);
			toast.error(errText(error, '删除失败'));
		},
	});

	const clearAll = useMutation({
		mutationFn: () => historyService.clear(),
		onSuccess: () => {
			setConfirmOpen(false);
			toast.success('浏览记录已清空');
			void queryClient.invalidateQueries({ queryKey: qk.history.all });
		},
		onError: (error) => toast.error(errText(error, '清空失败')),
	});

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader
				title="浏览记录"
				meta={`${total} 条`}
				actions={
					total > 0 ? (
						<Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
							<DialogTrigger asChild>
								<Button size="sm" variant="outline">
									清空
								</Button>
							</DialogTrigger>
							<DialogContent>
								<DialogHeader>
									<DialogTitle>清空浏览记录？</DialogTitle>
									<DialogDescription>
										只影响你自己的记录，不会删除任何帖子，也不影响其他用户。清空后「上次读到哪里」的位置记忆同样会丢失。
									</DialogDescription>
								</DialogHeader>
								<DialogFooter>
									<Button size="sm" variant="outline" onClick={() => setConfirmOpen(false)}>
										取消
									</Button>
									<Button
										size="sm"
										variant="destructive"
										loading={clearAll.isPending}
										onClick={() => clearAll.mutate()}
									>
										确认清空
									</Button>
								</DialogFooter>
							</DialogContent>
						</Dialog>
					) : null
				}
			/>

			{query.isPending ? (
				<ul className="m-0 flex list-none flex-col gap-2 p-0">
					{[0, 1, 2, 3, 4].map((i) => (
						<RowSkeleton key={i} />
					))}
				</ul>
			) : posts.length === 0 ? (
				<EmptyState title="还没有浏览记录" description="打开过的主题会出现在这里，方便回头接着看。" />
			) : (
				<>
					<ul className="m-0 flex list-none flex-col gap-2 p-0">
						{posts.map((post) => (
							<HistoryRow
								key={post.id}
								post={post}
								deleting={removing === post.id && remove.isPending}
								onDelete={() => remove.mutate(post.id)}
							/>
						))}
					</ul>
					<Pager
						className="mt-6"
						currentPage={page ?? 1}
						totalPages={totalPages(total, DEFAULT_PAGE_SIZE)}
						onPageChange={(next) => void setPage(next)}
					/>
					<p className="text-xs text-muted-foreground">
						看不见的主题（已删除、进回收站或等级不足）不会出现在这里，也不会计入条数。
					</p>
				</>
			)}
		</div>
	);
}

export default HistoryPage;
