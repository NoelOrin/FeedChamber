'use client';

import type { Content } from '@feedchamber/api-client';
import { contentService } from '@feedchamber/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { lazy, Suspense, useEffect, useRef, useState } from 'react';

import { Alert, AlertDescription } from '~/components/ui/alert';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Skeleton } from '~/components/ui/skeleton';
import { Send } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';
import { cn } from '~/lib/utils';

/* 与 composer 同一理由：编辑器 592KB，帖子详情/楼层回复框不该在首屏关键路径上同步取它 */
const MarkdownEditor = lazy(() => import('~/components/markdown-editor').then((m) => ({ default: m.MarkdownEditor })));

interface ReplyProps {
	postId: string;
	/** 顶层讨论的根内容 id（帖子正文那条 content） */
	rootContentId?: string;
	/** 回复某一层楼时传父楼层 id；缺省即回复主题 */
	parentId?: string;
	/** 楼层内嵌的精简形态 */
	compact?: boolean;
	/** 外层已经给了账号卡时用它：只渲染内容，不再自带卡片外壳 */
	bare?: boolean;
	placeholder?: string;
	onSuccess?: (created: Content) => void;
	onDone?: () => void;
}

const DEFAULT_PLACEHOLDER = '写下你的回复…支持 Markdown · 拖拽粘贴图片';

function errText(error: unknown, fallback: string) {
	return error instanceof Error && error.message ? error.message : fallback;
}

export function Reply({
	postId,
	rootContentId,
	parentId,
	compact,
	bare,
	placeholder = DEFAULT_PLACEHOLDER,
	onSuccess,
	onDone,
}: ReplyProps) {
	const queryClient = useQueryClient();
	const [value, setValue] = useState('');
	/* 草稿键与旧版同一套命名（reply:<postId>:<root 或父楼层>），迁移窗口内旧草稿仍可续写。 */
	const draftKey = `reply:${postId}:${parentId || rootContentId || 'root'}`;

	const submit = useMutation({
		mutationFn: () =>
			contentService.create(postId, { body: value.trim(), parent_id: parentId || rootContentId || undefined }),
		onSuccess: (created) => {
			setValue('');
			queryClient.invalidateQueries({ queryKey: qk.contents.list(postId) });
			queryClient.invalidateQueries({ queryKey: qk.posts.detail(postId) });
			if (parentId) queryClient.invalidateQueries({ queryKey: qk.contents.replies(parentId) });
			toast.success('回复已发布');
			onSuccess?.(created);
			onDone?.();
		},
		onError: (error) => toast.error(errText(error, '发布失败，请重试')),
	});

	const canSubmit = Boolean(value.trim()) && !submit.isPending;

	/* ⌘/Ctrl+Enter 快速发布：回调读最新闭包走 ref，监听只在挂载时挂一次。 */
	const submitRef = useRef(() => {});
	submitRef.current = () => {
		if (canSubmit) submit.mutate();
	};
	useEffect(() => {
		const onKey = (event: KeyboardEvent) => {
			if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
				event.preventDefault();
				submitRef.current();
			}
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	}, []);

	return (
		<Card
			variant={bare ? 'default' : 'inset'}
			hover="static"
			className={cn(bare ? 'gap-3 border-0 bg-transparent p-0' : compact ? 'gap-2.5 p-3' : 'gap-3 p-4')}
		>
			<Suspense fallback={<Skeleton className={cn('w-full', compact ? 'h-32' : 'h-44')} />}>
				<MarkdownEditor
					ariaLabel={compact ? '回复内容' : '评论内容'}
					value={value}
					onChange={setValue}
					placeholder={placeholder}
					onSubmit={() => submit.mutate()}
					submitting={submit.isPending}
					draftKey={draftKey}
					minHeight={compact ? 120 : 168}
				/>
			</Suspense>

			{submit.isError ? (
				<Alert variant="destructive">
					<AlertDescription>{errText(submit.error, '发布失败，请重试')}</AlertDescription>
				</Alert>
			) : null}

			<div className="flex items-center justify-between gap-2">
				<span className="text-[11px] text-muted-foreground">⌘/Ctrl + Enter 快速发布</span>
				<span className="flex items-center gap-2">
					{onDone ? (
						<Button variant="ghost" size="xs" onClick={onDone}>
							收起
						</Button>
					) : null}
					<Button
						size="sm"
						className="gap-1.5"
						disabled={!canSubmit}
						loading={submit.isPending}
						onClick={() => submit.mutate()}
					>
						<Send className="size-3.5" aria-hidden="true" /> 发布回复
					</Button>
				</span>
			</div>
		</Card>
	);
}

export default Reply;
