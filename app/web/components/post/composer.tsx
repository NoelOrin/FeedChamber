'use client';

import { draftService, PostStatus, postService, spaceService } from '@orinforum/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import { lazy, Suspense, useEffect, useRef, useState } from 'react';

import { clearDraft, loadDraft, saveDraft } from '~/components/markdown-editor/draft';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Skeleton } from '~/components/ui/skeleton';
import { t } from '~/lib/i18n';
import { Plus, Send, X } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/* 编辑器 592KB（tiptap + prosemirror）。懒加载：`/timeline/create` 的路由预取因此
 只拖表单壳，编辑器等真正渲染时才取；沿用仓内既有的 lazy+Suspense 写法
 （`layout/nav.tsx:16`、`content/code-block.tsx:11`），vinext 下 next/dynamic 不分包。 */
const MarkdownEditor = lazy(() => import('~/components/markdown-editor').then((m) => ({ default: m.MarkdownEditor })));

interface PostComposerProps {
	/** 版内发帖（/spaces/{slug}/post）时锁定版面；时间线发帖页不传 */
	spaceSlug?: string;
}

/** 草稿 localStorage 作用域键：沿用旧 create.tsx 的 `post:create:{spaceId||'global'}`；
 * 正文由 MarkdownEditor 写在该键（兼容旧 {title,content} 对象），标题/标签写 `…:meta` 键。 */
function draftScope(spaceId: string) {
	return `post:create:${spaceId || 'global'}`;
}

/**
 * Solid 版 pages/(app)/timeline/create.tsx 平移成的共用表单组件：
 * timeline/create 与 spaces/[slug]/post 两个页面都渲染它，只是版面来源不同。
 */
export function PostComposer({ spaceSlug }: PostComposerProps) {
	const router = useRouter();
	const queryClient = useQueryClient();
	const [title, setTitle] = useState('');
	const [content, setContent] = useState('');
	const [spaceId, setSpaceId] = useState('');
	const [tags, setTags] = useState<string[]>([]);
	const [tagInput, setTagInput] = useState('');
	const [serverDraftId, setServerDraftId] = useState('');
	const [submitting, setSubmitting] = useState(false);

	const { data: spaces } = useQuery({ queryKey: qk.spaces.tree, queryFn: () => spaceService.tree() });

	// 版内发帖：由 slug 反查空间（api-client 无 by-slug 接口，走 flat 全量匹配）
	const { data: flatSpaces } = useQuery({
		queryKey: qk.spaces.flat,
		queryFn: () => spaceService.flat(),
		enabled: Boolean(spaceSlug),
	});
	const lockedSpace = spaceSlug ? flatSpaces?.find((s) => s.slug === spaceSlug) : undefined;

	useEffect(() => {
		if (lockedSpace) setSpaceId((prev) => prev || lockedSpace.id);
	}, [lockedSpace]);

	// —— 草稿防抖 800ms：本地 localStorage + 服务端 draftService（旧 create.tsx 语义）——
	const stateRef = useRef({ title, content, spaceId, tags });
	stateRef.current = { title, content, spaceId, tags };
	const draftTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

	const scheduleDraftSave = () => {
		if (draftTimer.current) clearTimeout(draftTimer.current);
		draftTimer.current = setTimeout(() => {
			const s = stateRef.current;
			saveDraft(`${draftScope(s.spaceId)}:meta`, { title: s.title, tags: s.tags, spaceId: s.spaceId });
			draftService
				.save({ kind: 'post', title: s.title, content: s.content, space_id: s.spaceId || undefined })
				.then((d) => {
					if (d?.id) setServerDraftId(d.id);
				})
				.catch(() => {});
		}, 800);
	};

	useEffect(
		() => () => {
			if (draftTimer.current) clearTimeout(draftTimer.current);
		},
		[],
	);

	// 挂载读取草稿：旧对象形态的 title + 新版 :meta 键（正文草稿由编辑器自行读取并回灌 onChange）。
	// 只在挂载（或版面解析完成）时读一次，与旧 onMount 等价。
	const draftReady = !spaceSlug || Boolean(lockedSpace);
	const draftLoaded = useRef(false);
	useEffect(() => {
		if (!draftReady || draftLoaded.current) return;
		draftLoaded.current = true;
		const scope = draftScope(spaceId);
		const legacy = loadDraft<unknown>(scope);
		if (legacy && typeof legacy === 'object') {
			const v = legacy as { title?: string };
			if (v.title) setTitle(v.title);
		}
		const meta = loadDraft<{ title?: string; tags?: string[] }>(`${scope}:meta`);
		if (meta) {
			if (meta.title) setTitle(meta.title);
			if (Array.isArray(meta.tags)) setTags(meta.tags);
		}
		// 只在挂载（或版面解析完成）时读一次，与旧 onMount 等价
	}, [draftReady, spaceId]);

	const addTag = () => {
		const v = tagInput.trim();
		if (!v) return;
		if (tags.includes(v)) {
			setTagInput('');
			return;
		}
		setTags([...tags, v]);
		setTagInput('');
		scheduleDraftSave();
	};

	const removeTag = (tag: string) => setTags(tags.filter((x) => x !== tag));

	const handleSubmit = async () => {
		if (!title.trim() || !content.trim()) {
			toast.error('标题与正文不能为空');
			return;
		}
		setSubmitting(true);
		try {
			const post = await postService.create({
				title: title.trim(),
				content: content.trim(),
				space_id: spaceId || undefined,
			});
			if (tags.length > 0) {
				try {
					await postService.setTags(post.id, tags);
				} catch {
					toast.error('帖子已发布，但标签保存失败');
				}
			}
			clearDraft(draftScope(spaceId));
			clearDraft(`${draftScope(spaceId)}:meta`);
			if (serverDraftId) draftService.delete(serverDraftId).catch(() => {});
			/* 待审帖不会出现在任何公开列表里（契约 2：无权限者只看得到 approved/closed），
			   所以跳列表等于让作者以为发丢了：改为留在自己的帖子详情页并说明状态。 */
			const pending = post.status === PostStatus.PENDING;
			toast.success(pending ? '已提交审核，通过后将公开展示' : '发布成功');
			/* 列表 staleTime 内不会重取（信息流 10s），落回页面会看不到自己刚发的帖子 */
			queryClient.invalidateQueries({ queryKey: qk.posts.all });
			queryClient.invalidateQueries({ queryKey: qk.home });
			router.push(pending ? `/timeline/post?id=${post.id}` : spaceSlug ? `/spaces/${spaceSlug}` : '/timeline');
		} catch (e) {
			toast.error((e as Error)?.message || '发布失败');
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader title="发布新主题" />

			<Card className="gap-4 p-4 sm:p-5">
				<div className="flex flex-col gap-1.5">
					<Label htmlFor="post-title">{t('composer_title_label')}</Label>
					<Input
						id="post-title"
						value={title}
						onChange={(e) => {
							setTitle(e.currentTarget.value);
							scheduleDraftSave();
						}}
						placeholder="输入标题…"
					/>
				</div>

				<div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
					<div className="flex flex-col gap-1.5">
						<Label htmlFor="post-space">{t('composer_space_label')}</Label>
						{spaceSlug ? (
							lockedSpace ? (
								<div className="flex h-9 items-center rounded-md border border-input bg-muted/50 px-3 text-sm">
									<span className="text-foreground">{lockedSpace.name}</span>
									<span className="ml-1 text-muted-foreground">/{lockedSpace.slug}</span>
								</div>
							) : (
								<Skeleton className="h-9 w-full" />
							)
						) : (
							<select
								id="post-space"
								value={spaceId}
								onChange={(e) => {
									setSpaceId(e.currentTarget.value);
									scheduleDraftSave();
								}}
								className="h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm text-foreground shadow-xs outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/80"
							>
								<option value="">未选择（默认总版）</option>
								{(spaces ?? []).map((s) => (
									<option key={s.id} value={s.id}>
										{s.name} /{s.slug}
									</option>
								))}
							</select>
						)}
					</div>

					<div className="flex flex-col gap-1.5">
						<Label htmlFor="post-tag">{t('composer_tags_label')}</Label>
						<div className="flex gap-2">
							<Input
								id="post-tag"
								value={tagInput}
								onChange={(e) => setTagInput(e.currentTarget.value)}
								onKeyDown={(e) => {
									if (e.key === 'Enter') {
										e.preventDefault();
										addTag();
									}
								}}
								placeholder="输入标签回车"
								className="flex-1"
							/>
							<Button variant="outline" size="md" className="gap-1" onClick={addTag}>
								<Plus className="size-3.5" aria-hidden="true" />
								添加
							</Button>
						</div>
						{tags.length > 0 ? (
							<div className="flex flex-wrap gap-1.5">
								{tags.map((tag) => (
									<Badge key={tag} variant="secondary" className="gap-1">
										{tag}
										<button
											type="button"
											onClick={() => removeTag(tag)}
											aria-label={`移除标签 ${tag}`}
											className="cursor-pointer text-muted-foreground transition-colors hover:text-foreground"
										>
											<X className="size-3" aria-hidden="true" />
										</button>
									</Badge>
								))}
							</div>
						) : null}
					</div>
				</div>

				<div className="flex flex-col gap-1.5">
					<span className="text-[13px] leading-none font-medium">{t('composer_content_label')}</span>
					<Suspense fallback={<Skeleton className="h-48 w-full" />}>
						{draftReady ? (
							<MarkdownEditor
								ariaLabel="主题正文"
								value={content}
								onChange={(v) => {
									setContent(v);
									scheduleDraftSave();
								}}
								placeholder="用 Markdown 书写…"
								onSubmit={handleSubmit}
								submitting={submitting}
								draftKey={draftScope(spaceId)}
							/>
						) : (
							<Skeleton className="h-48 w-full" />
						)}
					</Suspense>
				</div>

				<div className="flex flex-wrap items-center justify-between gap-2 border-t border-border pt-3">
					<span className="hidden text-[11px] text-muted-foreground sm:inline">⌘/Ctrl + Enter 快速发布</span>
					<div className="ml-auto flex items-center gap-2">
						<Button
							variant="outline"
							size="md"
							onClick={() => router.push(spaceSlug ? `/spaces/${spaceSlug}` : '/timeline')}
						>
							{t('composer_cancel')}
						</Button>
						<Button size="md" loading={submitting} onClick={handleSubmit} className="gap-1.5">
							<Send className="size-4" aria-hidden="true" /> {t('composer_publish')}
						</Button>
					</div>
				</div>
			</Card>
		</div>
	);
}
