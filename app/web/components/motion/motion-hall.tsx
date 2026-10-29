'use client';

import { motionService } from '@orinforum/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { MotionCard } from '~/components/motion/motion-card';
import { motionListOptions } from '~/components/motion/queries';
import { Button } from '~/components/ui/button';
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from '~/components/ui/dialog';
import { EmptyState } from '~/components/ui/empty-state';
import { PageHeader } from '~/components/ui/page-header';
import { Segmented } from '~/components/ui/segmented';
import { Skeleton } from '~/components/ui/skeleton';
import { Textarea } from '~/components/ui/textarea';
import { qk } from '~/lib/query/keys';

const TABS: Array<{ value: string; label: string }> = [
	{ value: 'pending', label: '表决中' },
	{ value: 'passed', label: '已通过' },
	{ value: 'rejected', label: '未通过' },
	{ value: '', label: '全部' },
];

const MOTION_TYPES = [
	{ value: 'fold', label: '折叠（内容降权）' },
	{ value: 'watch', label: '观察（限时标记）' },
	{ value: 'escalate', label: '移交版主（生成举报）' },
	{ value: 'exile', label: '放逐暗区（软隔离）' },
];

export function MotionHall({ canManage }: { canManage: boolean }) {
	const [status, setStatus] = useState('pending');
	const { data, isPending } = useQuery(motionListOptions({ status, page: 1, pageSize: 20 }));
	const motions = data?.items ?? [];

	return (
		<div className="flex w-full flex-col gap-3 pb-10">
			<PageHeader
				title="自治大厅"
				meta={`${motions.length} 条`}
				actions={
					<Segmented label="动议状态" value={status} options={TABS} onValueChange={(value) => setStatus(value)} />
				}
			/>

			<p className="text-[13px] leading-relaxed text-muted-foreground">
				动议是社区的集体裁判通道，与「举报」并行：举报针对违规，由管理员处理；动议针对灰区争议，由信誉加权表决决定。
				结果一律柔性——折叠降权、限定观察、移交版主、放逐到暗区软隔离，可翻案。硬封禁与封号仍由版主/管理员裁量，不会自动发生。
			</p>

			{isPending ? (
				<div className="flex flex-col gap-2">
					{(['a', 'b', 'c'] as const).map((k) => (
						<Skeleton key={`motion-skeleton-${k}`} className="h-28 w-full rounded-lg" />
					))}
				</div>
			) : motions.length > 0 ? (
				<div className="flex flex-col gap-2">
					{motions.map((motion) => (
						<MotionCard key={motion.id} motion={motion} canManage={canManage} />
					))}
				</div>
			) : (
				<EmptyState title="暂无动议" description="在帖子、评论或用户上发起一条动议后，会在这里公示并进入表决。" />
			)}
		</div>
	);
}

/** 「发起自治动议」入口，放在帖子/评论/用户卡片上复用。 */
export function StartMotionButton({
	targetType,
	targetId,
	label = '发起动议',
}: {
	targetType: 'post' | 'comment' | 'user';
	targetId: string;
	label?: string;
}) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const [type, setType] = useState('fold');
	const [reason, setReason] = useState('');

	const create = useMutation({
		mutationFn: () => motionService.create({ target_type: targetType, target_id: targetId, type, reason }),
		onSuccess: () => {
			void queryClient.invalidateQueries({ queryKey: qk.motions.all });
			setOpen(false);
			setReason('');
		},
	});

	const error = create.error instanceof Error ? create.error.message : '';

	return (
		<>
			<button
				type="button"
				onClick={() => setOpen(true)}
				className="text-[11px] text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
			>
				{label}
			</button>

			<Dialog open={open} onOpenChange={setOpen}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>发起自治动议</DialogTitle>
						<DialogDescription>
							动议会在自治大厅公示并进入信誉加权表决，达法定人数后按赞成/反对结算。请选择类型并说明理由。
						</DialogDescription>
					</DialogHeader>
					<div className="flex flex-col gap-3">
						<label className="flex flex-col gap-1.5 text-[13px]">
							动议类型
							<select
								value={type}
								onChange={(e) => setType(e.target.value)}
								className="h-9 rounded-md border border-border bg-background px-2 text-[13px]"
							>
								{MOTION_TYPES.map((t) => (
									<option key={t.value} value={t.value}>
										{t.label}
									</option>
								))}
							</select>
						</label>
						<Textarea
							value={reason}
							onChange={(e) => setReason(e.target.value)}
							placeholder="说明发起理由（会公示给所有参与者）"
							rows={4}
						/>
						{error ? <p className="text-[12px] text-destructive">{error}</p> : null}
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setOpen(false)}>
							取消
						</Button>
						<Button disabled={create.isPending || reason.trim().length === 0} onClick={() => create.mutate()}>
							提交动议
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</>
	);
}
