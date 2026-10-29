'use client';

import { followService, pmService, userBlockService } from '@orinforum/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { EmptyState } from '~/components/ui/empty-state';
import { ActivityHeatmap } from '~/components/user/activity-heatmap';
import { PostList } from '~/components/user/post-list';
import {
	followCountsOptions,
	publicProfileOptions,
	totalPages,
	USER_POST_PAGE_SIZE,
	userBadgesOptions,
	userPostsOptions,
} from '~/components/user/queries';
import { UserProfileSkeleton } from '~/components/user/skeletons';
import { trustLevelName, useTrustLevels } from '~/lib/hooks/use-trust-levels';
import { Activity as ActivityIcon, Award, MessageCircle, Shield, User as UserIcon } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const ROLE_LABELS: Record<string, string> = {
	admin: 'ADMIN — 管理员',
	moderator: 'MOD — 版主',
	user: 'MEMBER',
	ban: 'BANNED',
};

interface VisitorProfilePageProps {
	userId: string;
	currentUserId: string;
}

/** 他人主页。 */
export function VisitorProfilePage({ userId, currentUserId }: VisitorProfilePageProps) {
	const router = useRouter();
	const queryClient = useQueryClient();
	const levels = useTrustLevels();

	const profileQuery = useQuery(publicProfileOptions(userId));
	const countsQuery = useQuery(followCountsOptions(userId));
	const badgesQuery = useQuery(userBadgesOptions(userId));
	const [postsPage, setPostsPage] = useState(1);
	const postsQuery = useQuery(userPostsOptions(userId, postsPage));

	const [isFollowing, setIsFollowing] = useState(false);
	const [isBlocked, setIsBlocked] = useState(false);
	const [blockLoading, setBlockLoading] = useState(false);
	const [pmLoading, setPmLoading] = useState(false);

	const profile = profileQuery.data;
	const person = profile?.user;
	const counts = countsQuery.data ?? { followers: 0, following: 0 };
	const badges = badgesQuery.data ?? [];
	const posts = postsQuery.data?.data ?? [];
	const postsTotal = postsQuery.data?.total ?? 0;
	const isSelf = currentUserId === userId;

	/* 旧版 onMount 三连发里的屏蔽状态：非列表数据，保留本地 state + 挂载时取一次 */
	useEffect(() => {
		let active = true;
		userBlockService
			.status(userId)
			.then((res) => {
				if (active) setIsBlocked(Boolean(res?.blocked));
			})
			.catch(() => {});
		return () => {
			active = false;
		};
	}, [userId]);

	const follow = useMutation({
		mutationFn: (followTarget: boolean) =>
			followTarget ? followService.follow(userId) : followService.unfollow(userId),
		onSuccess: (_data, followTarget) => {
			setIsFollowing(followTarget);
			toast.success(followTarget ? '已关注' : '已取消关注');
			void queryClient.invalidateQueries({ queryKey: qk.follow.all });
			void queryClient.invalidateQueries({ queryKey: qk.user.publicProfile(userId) });
		},
		onError: () => toast.error('操作失败'),
	});

	const block = useMutation({
		mutationFn: (blockTarget: boolean) =>
			blockTarget ? userBlockService.block(userId) : userBlockService.unblock(userId),
		onMutate: () => setBlockLoading(true),
		onSuccess: (_data, blockTarget) => setIsBlocked(blockTarget),
		onError: () => toast.error('操作失败'),
		onSettled: () => setBlockLoading(false),
	});

	const startConversation = useMutation({
		mutationFn: () => pmService.create({ peer_user_id: userId, body: '你好' }),
		onSuccess: (res) => router.push(`/messages/${encodeURIComponent(res.conversation.id)}`),
		onError: () => toast.error('发起私信失败'),
		onSettled: () => setPmLoading(false),
	});

	function startPM() {
		if (pmLoading || startConversation.isPending) return;
		if (!userId) return;
		if (currentUserId && currentUserId === userId) {
			toast.error('不能给自己发私信');
			return;
		}
		setPmLoading(true);
		startConversation.mutate();
	}

	if (profileQuery.isPending) {
		return (
			<div className="w-full">
				<UserProfileSkeleton />
			</div>
		);
	}

	/* 未登录 404 边界：拿不到档案就画空状态，不抛错 */
	if (!person) {
		return (
			<div className="w-full">
				<EmptyState title="查无此档" description="档案索引里没有这个编号 — 可能已被移除或从未建立。" />
			</div>
		);
	}

	const following = profile?.is_following ?? isFollowing;

	return (
		<div className="flex w-full flex-col gap-3">
			<Card className="gap-4 p-4 sm:p-5">
				<div className="flex flex-wrap items-center gap-3">
					<div className="grid size-16 shrink-0 place-items-center overflow-hidden rounded-full border border-border bg-muted text-lg font-medium text-muted-foreground">
						{person.avatar ? (
							<img src={person.avatar} alt="" loading="lazy" className="size-full object-cover" />
						) : (
							(person.display_name || person.username || '?').slice(0, 1).toUpperCase()
						)}
					</div>

					<div className="min-w-0 flex-1">
						<div className="flex flex-wrap items-center gap-2">
							<h1 className="truncate text-xl font-semibold tracking-tight text-foreground">{person.display_name}</h1>
							{person.role ? <Badge variant="state">{ROLE_LABELS[person.role] ?? person.role}</Badge> : null}
							{trustLevelName(levels, person.trust_level) ? (
								<Badge variant="secondary">{trustLevelName(levels, person.trust_level)}</Badge>
							) : null}
						</div>
						<div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
							<span>@{person.username}</span>
							<span aria-hidden="true">·</span>
							<span>注册于 {person.created_at ? dayjs(person.created_at).format('YYYY-MM-DD') : '——'}</span>
						</div>
					</div>

					<div className="flex shrink-0 items-center gap-2">
						{!isSelf ? (
							<Button size="sm" variant="outline" loading={pmLoading} onClick={startPM} className="gap-1.5">
								<MessageCircle className="size-3.5" aria-hidden="true" /> 私信
							</Button>
						) : null}
						<Button
							size="sm"
							variant={following ? 'outline' : 'default'}
							disabled={follow.isPending}
							onClick={() => follow.mutate(!following)}
							className="gap-1.5"
						>
							<UserIcon className="size-3.5" aria-hidden="true" /> {following ? '已关注' : '关注'}
						</Button>
						{!isSelf ? (
							<Button
								size="sm"
								variant="ghost"
								loading={blockLoading}
								onClick={() => block.mutate(!isBlocked)}
								className="gap-1.5 text-muted-foreground"
							>
								<Shield className="size-3.5" aria-hidden="true" /> {isBlocked ? '取消屏蔽' : '屏蔽'}
							</Button>
						) : null}
					</div>
				</div>

				<dl className="grid grid-cols-3 gap-1 rounded-lg border border-border p-1 sm:grid-cols-5 sm:gap-0 sm:divide-x sm:divide-border sm:p-0">
					{[
						{ key: 'posts', value: profile?.post_count ?? 0, label: '帖子' },
						{ key: 'comments', value: profile?.comment_count ?? 0, label: '评论' },
						{ key: 'likes', value: profile?.like_count ?? 0, label: '获赞' },
						{ key: 'followers', value: counts.followers, label: '粉丝' },
						{ key: 'following', value: counts.following, label: '关注' },
					].map((cell) => (
						<div
							key={cell.key}
							className="flex flex-col gap-0.5 rounded-md px-3 py-2.5 sm:rounded-none sm:px-4 sm:py-3"
						>
							<dd className="text-lg leading-tight font-semibold text-foreground tabular-nums">{cell.value}</dd>
							<dt className="text-xs text-muted-foreground">{cell.label}</dt>
						</div>
					))}
				</dl>

				<section className="flex flex-col gap-2">
					<h2 className="text-xs font-medium text-muted-foreground">个人简介</h2>
					<p className="text-sm leading-7 break-words whitespace-pre-wrap text-foreground">
						{person.bio || '该用户还没有填写简介。'}
					</p>
				</section>

				{badges.length > 0 ? (
					<div className="flex flex-wrap gap-1.5">
						{badges.map((badge) => (
							<span
								key={badge.code}
								className="inline-flex items-center gap-1 rounded-md border border-border px-1.5 py-0.5 text-[11px] text-muted-foreground"
							>
								<Award className="size-3" aria-hidden="true" /> {badge.name}
							</span>
						))}
					</div>
				) : null}
			</Card>

			<Card className="min-w-0 gap-3 p-4">
				<h2 className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
					<ActivityIcon className="size-3.5" aria-hidden="true" /> 活跃度
				</h2>
				<ActivityHeatmap userId={userId} />
			</Card>

			<section className="flex flex-col gap-2">
				<h2 className="text-xs font-medium text-muted-foreground">发布的主题</h2>
				<PostList
					posts={posts}
					currentPage={postsPage}
					totalPages={totalPages(postsTotal, USER_POST_PAGE_SIZE)}
					onPageChange={setPostsPage}
					loading={postsQuery.isPending}
				/>
			</section>
		</div>
	);
}
