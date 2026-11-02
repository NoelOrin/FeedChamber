'use client';

import { motionService } from '@feedchamber/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { MOTION_STATUS_LABEL, MOTION_TYPE_LABEL } from '~/components/motion/queries';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from '~/components/ui/dialog';
import { Textarea } from '~/components/ui/textarea';
import { useAuthStore } from '~/lib/auth/session';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/** 动议列表行（barrel 未导出 MotionListItem，从返回值剥类型）。
 *  post<T> 已拆信封，list 返回 { items, total }，没有 data 层。 */
export type MotionRow = Awaited<ReturnType<typeof motionService.list>>['items'][number];

const TARGET_LABEL: Record<string, string> = { post: '帖子', comment: '评论', user: '用户' };

const STATUS_VARIANT: Record<string, 'warning' | 'success' | 'outline' | 'info'> = {
	pending: 'warning',
	passed: 'success',
	rejected: 'outline',
	overturned: 'info',
};

/* 法定人数默认 20；用户类动议服务端会加倍，这里只做可视化提示。 */
const QUORUM = 20;

function Progress({ approve, reject }: { approve: number; reject: number }) {
	const total = approve + reject;
	const ratio = Math.min(1, total / QUORUM);
	const approveShare = total > 0 ? approve / total : 0;

	return (
		<div className="flex flex-col gap-1">
			<div className="flex h-1.5 w-full overflow-hidden rounded-full bg-muted">
				<div className="bg-emerald-500" style={{ width: `${ratio * approveShare * 100}%` }} />
				<div className="bg-rose-500" style={{ width: `${ratio * (1 - approveShare) * 100}%` }} />
			</div>
			<span className="text-[11px] text-muted-foreground tabular-nums">
				赞成 {approve.toFixed(1)} · 反对 {reject.toFixed(1)} · 已参与 {total.toFixed(1)}/{QUORUM}
			</span>
		</div>
	);
}

export function MotionCard({ motion, canManage }: { motion: MotionRow; canManage: boolean }) {
	const queryClient = useQueryClient();
	const authed = useAuthStore((s) => s.status === 'authenticated');
	const [manageOpen, setManageOpen] = useState(false);
	const [note, setNote] = useState('');

	const invalidate = () => void queryClient.invalidateQueries({ queryKey: qk.motions.all });

	const vote = useMutation({
		mutationFn: (side: 'approve' | 'reject') => motionService.vote(motion.id, side),
		onSuccess: () => {
			toast.success('已记录你的表决');
			invalidate();
		},
		onError: (err: Error) => toast.error(err.message),
	});

	const overturn = useMutation({
		mutationFn: () => motionService.updateStatus(motion.id, { status: 'overturned' }),
		onSuccess: () => {
			toast.success('已翻案并解除软隔离');
			setManageOpen(false);
			invalidate();
		},
		onError: (err: Error) => toast.error(err.message),
	});

	const markAbuse = useMutation({
		mutationFn: () => motionService.updateStatus(motion.id, { abuse: true }),
		onSuccess: () => {
			toast.success('已标记滥诉');
			setManageOpen(false);
			invalidate();
		},
		onError: (err: Error) => toast.error(err.message),
	});

	const hardBan = useMutation({
		mutationFn: () => motionService.updateStatus(motion.id, { hard_ban: true }),
		onSuccess: () => {
			toast.success('已升级为硬封禁（人工裁量）');
			setManageOpen(false);
			invalidate();
		},
		onError: (err: Error) => toast.error(err.message),
	});

	const isPending = motion.status === 'pending';
	const isUserExile = motion.type === 'exile' && motion.target_type === 'user';
	const busy = vote.isPending || overturn.isPending || markAbuse.isPending || hardBan.isPending;

	return (
		<article className="flex flex-col gap-2.5 rounded-lg border border-border/60 p-3">
			<div className="flex flex-wrap items-center gap-1.5">
				<Badge variant={STATUS_VARIANT[motion.status] ?? 'outline'}>
					{MOTION_STATUS_LABEL[motion.status] ?? motion.status}
				</Badge>
				<span className="text-[13px] font-medium">{MOTION_TYPE_LABEL[motion.type] ?? motion.type}</span>
				<span className="text-[11px] text-muted-foreground">
					{TARGET_LABEL[motion.target_type] ?? motion.target_type} · {motion.target_id}
				</span>
				{motion.abuse_flag ? <Badge variant="destructive">已判滥诉</Badge> : null}
			</div>

			<p className="line-clamp-3 text-[13px] leading-relaxed text-muted-foreground">{motion.reason}</p>

			<Progress approve={motion.approve_weight} reject={motion.reject_weight} />

			<div className="flex flex-wrap items-center gap-1.5">
				{isPending && authed ? (
					<>
						<Button size="sm" variant="outline" disabled={busy} onClick={() => vote.mutate('approve')}>
							赞成
						</Button>
						<Button size="sm" variant="outline" disabled={busy} onClick={() => vote.mutate('reject')}>
							反对
						</Button>
					</>
				) : null}

				{canManage && isPending ? (
					<Button size="sm" variant="ghost" onClick={() => setManageOpen(true)}>
						裁定
					</Button>
				) : null}
			</div>

			<Dialog open={manageOpen} onOpenChange={setManageOpen}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>motion:manage 裁定</DialogTitle>
						<DialogDescription>
							以下操作均为人工作出，不由动议状态机自动执行：翻案解除软隔离；标记滥诉累计达阈值会限制发起者资格；
							硬封禁/封号仅在情节触红线时作为裁量升级。
						</DialogDescription>
					</DialogHeader>
					<Textarea value={note} onChange={(e) => setNote(e.target.value)} placeholder="裁定说明（可选）" rows={3} />
					<DialogFooter>
						<Button variant="outline" onClick={() => setManageOpen(false)}>
							取消
						</Button>
						<Button variant="secondary" disabled={busy} onClick={() => markAbuse.mutate()}>
							标记滥诉
						</Button>
						{isUserExile ? (
							<Button variant="destructive" disabled={busy} onClick={() => hardBan.mutate()}>
								升级硬封禁
							</Button>
						) : null}
						<Button disabled={busy} onClick={() => overturn.mutate()}>
							翻案并解除隔离
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</article>
	);
}
