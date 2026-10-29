'use client';

import { authService, type User, uploadService, userService } from '@orinforum/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import dayjs from 'dayjs';
import { useState } from 'react';
import { CheckInPanel } from '~/components/checkin/check-in-panel';
import { Alert, AlertDescription } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Input } from '~/components/ui/input';
import { ActivityHeatmap } from '~/components/user/activity-heatmap';
import { CoverImage } from '~/components/user/cover-image';
import { MyPosts } from '~/components/user/my-posts';
import { ProfileBio } from '~/components/user/profile-bio';
import { ProfileEditForm } from '~/components/user/profile-edit-form';
import { ProfileHeader } from '~/components/user/profile-header';
import {
	dashboardHomeOptions,
	publicProfileOptions,
	totalPages,
	USER_POST_PAGE_SIZE,
	userBadgesOptions,
	userPostsOptions,
} from '~/components/user/queries';
import { UserProfileSkeleton } from '~/components/user/skeletons';
import { StatsCards } from '~/components/user/stats-cards';
import { setSessionUser, useAuthStore } from '~/lib/auth';
import { Activity as ActivityIcon, Award, Calendar, LoaderCircle, ScanLine } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/** 我的主页（旧 pages/(app)/user/index.tsx 整页平移：资料头 + 统计卡 + 签到 + 我的帖子）。 */
export function MyProfilePage({ initialUser }: { initialUser?: User | null }) {
	const queryClient = useQueryClient();
	const authUser = useAuthStore((state) => state.user);
	const { data: dashboard } = useQuery(dashboardHomeOptions());
	/* 身份从 RSC 以 prop 下发：服务端 store 为空、客户端 store 要等 effect 才 seed，
	   两边都回落到同一个 initialUser，SSR 与首帧才会渲染同一棵树（否则水合必然失败）。 */
	const user = authUser ?? initialUser ?? dashboard?.user ?? null;

	const [editing, setEditing] = useState(false);
	const [username, setUsername] = useState('');
	const [displayName, setDisplayName] = useState('');
	const [bio, setBio] = useState('');
	const [avatar, setAvatar] = useState('');
	const [coverDraft, setCoverDraft] = useState('');
	const [postsPage, setPostsPage] = useState(1);
	const [verifyCode, setVerifyCode] = useState('');

	const postsQuery = useQuery({ ...userPostsOptions(user?.id ?? '', postsPage), enabled: Boolean(user?.id) });
	const profileQuery = useQuery({ ...publicProfileOptions(user?.id ?? ''), enabled: Boolean(user?.id) });
	const badgesQuery = useQuery({ ...userBadgesOptions(user?.id ?? ''), enabled: Boolean(user?.id) });

	const posts = postsQuery.data?.data ?? [];
	const postsTotal = postsQuery.data?.total ?? 0;
	const userBadges = badgesQuery.data ?? [];
	const followCounts = dashboard?.follow_counts ?? { followers: 0, following: 0 };
	const daysSinceRegister = user ? Math.max(1, Math.floor(dayjs().diff(dayjs(user.created_at), 'day'))) : 0;

	function startEdit() {
		if (!user) return;
		setUsername(user.username);
		setDisplayName(user.display_name ?? user.username);
		setBio(user.bio ?? '');
		setAvatar(user.avatar ?? '');
		setCoverDraft(user.cover_image ?? '');
		setEditing(true);
	}

	const saveProfile = useMutation({
		mutationFn: () =>
			userService.updateProfile({
				username: username.trim(),
				display_name: displayName.trim(),
				bio: bio.trim(),
				avatar,
				cover_image: coverDraft,
				profile_layout: '',
			}),
		onSuccess: () => {
			if (user) {
				setSessionUser({
					...user,
					username: username.trim(),
					display_name: displayName.trim(),
					bio: bio.trim(),
					avatar,
					cover_image: coverDraft,
					profile_layout: '',
				});
			}
			toast.success('资料已更新');
			setEditing(false);
			void queryClient.invalidateQueries({ queryKey: qk.profile });
			void queryClient.invalidateQueries({ queryKey: qk.home });
			if (user) void queryClient.invalidateQueries({ queryKey: qk.user.publicProfile(user.id) });
		},
		onError: (err) => toast.error((err as Error).message),
	});

	const uploadAvatar = useMutation({
		mutationFn: (file: File) => uploadService.upload(file),
		onSuccess: (res) => {
			setAvatar(res.url);
			toast.success('头像已上传，保存后生效');
		},
		onError: () => toast.error('头像上传失败'),
	});

	const uploadCover = useMutation({
		mutationFn: (file: File) => uploadService.upload(file),
		onSuccess: (res) => {
			setCoverDraft(res.url);
			toast.success('封面已上传，保存后生效');
		},
		onError: () => toast.error('封面上传失败'),
	});

	const sendVerify = useMutation({
		mutationFn: () => authService.sendVerifyEmail(),
		onSuccess: () => toast.success('验证码已发送，请查收邮箱'),
		onError: (err) => toast.error((err as Error).message || '发送失败'),
	});

	const submitVerify = useMutation({
		mutationFn: () => authService.verifyEmail({ code: verifyCode.trim() }),
		onSuccess: () => {
			if (user) setSessionUser({ ...user, email_verified: true });
			toast.success('邮箱验证成功');
			setVerifyCode('');
			void queryClient.invalidateQueries({ queryKey: qk.profile });
		},
		onError: (err) => toast.error((err as Error).message || '验证失败'),
	});

	function handleSave() {
		if (!user) return;
		if (!username.trim()) {
			toast.error('用户名不能为空');
			return;
		}
		if (!displayName.trim()) {
			toast.error('显示名称不能为空');
			return;
		}
		if (/\s/.test(displayName)) {
			toast.error('显示名称不能包含空格');
			return;
		}
		saveProfile.mutate();
	}

	function handleVerify() {
		if (!verifyCode.trim()) {
			toast.error('请输入验证码');
			return;
		}
		submitVerify.mutate();
	}

	if (!user) {
		return (
			<div className="w-full">
				<UserProfileSkeleton />
			</div>
		);
	}

	return (
		<div className="flex w-full flex-col gap-3">
			<Card className="gap-0 overflow-hidden p-0">
				<div className="p-3 sm:p-4">
					<CoverImage
						url={editing ? coverDraft : user.cover_image}
						editing={editing}
						onUpload={(file) => uploadCover.mutate(file)}
						onRemove={() => setCoverDraft('')}
					/>
				</div>

				<div className="flex flex-col gap-4 px-4 pb-4 sm:px-5 sm:pb-5">
					<ProfileHeader
						displayName={user.display_name || user.username}
						role={user.role}
						trustLevel={user.trust_level}
						avatar={editing ? avatar : (user.avatar ?? '')}
						isOwnProfile
						onEdit={startEdit}
						followCounts={followCounts}
					/>

					<div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
						<span>@{user.username}</span>
						<span aria-hidden="true">·</span>
						<span>注册于 {dayjs(user.created_at).format('YYYY-MM-DD')}</span>
						{userBadges.length > 0 ? (
							<span className="flex flex-wrap items-center gap-1.5">
								{userBadges.map((badge) => (
									<span
										key={badge.code}
										className="inline-flex items-center gap-1 rounded-md border border-border px-1.5 py-0.5 text-[11px]"
									>
										<Award className="size-3" aria-hidden="true" /> {badge.name}
									</span>
								))}
							</span>
						) : null}
					</div>

					{editing ? (
						<label className="inline-flex cursor-pointer items-center gap-2 text-xs text-muted-foreground transition-colors hover:text-foreground">
							{uploadAvatar.isPending ? (
								<LoaderCircle className="size-3.5 animate-spin" aria-hidden="true" />
							) : (
								<ScanLine className="size-3.5" aria-hidden="true" />
							)}
							更换头像（建议 400×400）
							<input
								type="file"
								accept="image/*"
								className="hidden"
								disabled={uploadAvatar.isPending}
								onChange={(event) => {
									const file = event.currentTarget.files?.[0];
									if (file) uploadAvatar.mutate(file);
								}}
							/>
						</label>
					) : null}

					{!user.email_verified ? (
						<Alert variant="warning">
							<div className="flex-1">
								<AlertDescription>邮箱尚未验证，验证后才能发帖与评论。</AlertDescription>
								<div className="mt-2 flex flex-wrap items-center gap-2">
									<Button
										size="xs"
										variant="outline"
										disabled={sendVerify.isPending}
										onClick={() => sendVerify.mutate()}
									>
										{sendVerify.isPending ? '发送中…' : '发送验证码'}
									</Button>
									<Input
										aria-label="邮箱验证码"
										className="h-7 w-28 text-xs"
										placeholder="验证码"
										value={verifyCode}
										onChange={(event) => setVerifyCode(event.currentTarget.value)}
									/>
									<Button size="xs" disabled={submitVerify.isPending} onClick={handleVerify}>
										{submitVerify.isPending ? '验证中…' : '验证'}
									</Button>
								</div>
							</div>
						</Alert>
					) : null}

					<StatsCards
						postCount={postsTotal}
						commentCount={profileQuery.data?.comment_count ?? 0}
						likeCount={profileQuery.data?.like_count ?? 0}
						daysSinceRegister={daysSinceRegister}
						points={dashboard?.user?.total_xp}
					/>

					{editing ? (
						<ProfileEditForm
							displayName={displayName}
							username={username}
							bio={bio}
							onDisplayNameChange={setDisplayName}
							onUsernameChange={setUsername}
							onBioChange={setBio}
							onCancel={() => setEditing(false)}
							onSave={handleSave}
							saving={saveProfile.isPending}
						/>
					) : (
						<ProfileBio bio={user.bio ?? ''} />
					)}
				</div>
			</Card>

			<Card className="min-w-0 gap-3 p-4">
				<h2 className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
					<Calendar className="size-3.5" aria-hidden="true" /> 签到
				</h2>
				<CheckInPanel />
			</Card>

			<Card className="min-w-0 gap-3 p-4">
				<h2 className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
					<ActivityIcon className="size-3.5" aria-hidden="true" /> 活跃度
				</h2>
				<ActivityHeatmap userId={user.id} />
			</Card>

			<MyPosts
				posts={posts}
				currentPage={postsPage}
				totalPages={totalPages(postsTotal, USER_POST_PAGE_SIZE)}
				loading={postsQuery.isPending}
				onPageChange={setPostsPage}
			/>
		</div>
	);
}
