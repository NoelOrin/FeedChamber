'use client';

import { reportService } from '@orinforum/api-client';
import { formatTime } from '@orinforum/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { Card } from '~/components/ui/card';
import { Descriptions, DescriptionsItem } from '~/components/ui/descriptions';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Label } from '~/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Textarea } from '~/components/ui/textarea';
import { useNavigate, useSearch } from '~/lib/admin/navigation';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const statusEnum: Record<string, { text: string; variant: 'warning' | 'primary' | 'outline' }> = {
	pending: { text: '待处理', variant: 'warning' as const },
	resolved: { text: '已处理', variant: 'primary' as const },
	dismissed: { text: '已驳回', variant: 'outline' as const },
};

const targetTypeMap: Record<string, string> = { post: '帖子', comment: '评论' };

export default function ReportDetail() {
	const navigate = useNavigate();
	const search = useSearch({ strict: false });
	const id = (search as { id?: string }).id || '';
	const [resolveOpen, setResolveOpen] = useState(false);
	const [resolveAction, setResolveAction] = useState('none');
	const [resolveNote, setResolveNote] = useState('');
	const [actionLoading, setActionLoading] = useState(false);
	const queryClient = useQueryClient();

	const { data: report = null } = useQuery({
		queryKey: qk.admin.reports.detail(id),
		queryFn: () => reportService.getById(id),
		enabled: Boolean(id),
	});

	/** 状态变更后让详情缓存失效。 */
	const invalidate = () => queryClient.invalidateQueries({ queryKey: qk.admin.reports.all });

	const handleUpdateStatus = async (status: string, action = 'none', note = '') => {
		setActionLoading(true);
		try {
			await reportService.updateStatus(id, status, action, note);
			toast.success('状态更新成功');
			setResolveOpen(false);
			invalidate();
		} catch (e: any) {
			toast.error(e.message || '操作失败');
		} finally {
			setActionLoading(false);
		}
	};

	const handleDelete = async () => {
		try {
			await reportService.delete(id);
			toast.success('删除成功');
			navigate({ to: '/admin/reports' });
		} catch (e: any) {
			toast.error(e.message || '删除失败');
		}
	};

	const viewTarget = () => {
		if (!report) return;
		if (report.target_type === 'post') {
			navigate({ to: '/admin/posts/detail', search: { id: report.target_id } });
		} else {
			navigate({ to: '/admin/comments/detail', search: { id: report.target_id } });
		}
	};

	const statusCfg = report ? statusEnum[report.status] : undefined;

	if (!id) {
		return (
			<div>
				<p>缺少举报 ID</p>
				<Button onClick={() => navigate({ to: '/admin/reports' })}>返回列表</Button>
			</div>
		);
	}

	return (
		<div>
			<div className="mb-6 flex items-center gap-3">
				<Button onClick={() => navigate({ to: '/admin/reports' })}>← 返回</Button>
				<h1 className="text-2xl font-semibold text-foreground m-0">举报详情</h1>
			</div>

			<Card className="mb-4 p-5">
				{report && (
					<Descriptions>
						<DescriptionsItem label="举报 ID" span={2}>
							{report.id}
						</DescriptionsItem>
						<DescriptionsItem label="目标类型">
							{targetTypeMap[report.target_type] || report.target_type}
						</DescriptionsItem>
						<DescriptionsItem label="Flag 类型">{report.flag_type || '-'}</DescriptionsItem>
						<DescriptionsItem label="目标 ID">
							<div className="inline-flex items-center gap-1.5">
								{report.target_id}
								<Button variant="link" size="xs" onClick={viewTarget}>
									查看目标
								</Button>
							</div>
						</DescriptionsItem>
						<DescriptionsItem label="举报原因" span={2}>
							{report.reason}
						</DescriptionsItem>
						<DescriptionsItem label="目标标题" span={2}>
							{report.target_title || '-'}
						</DescriptionsItem>
						<DescriptionsItem label="目标正文" span={2}>
							<div className="whitespace-pre-wrap max-h-40 overflow-auto">{report.target_body || '-'}</div>
						</DescriptionsItem>
						<DescriptionsItem label="聚合举报数">{report.count ?? 1}</DescriptionsItem>
						<DescriptionsItem label="举报人 ID">
							<div className="inline-flex items-center gap-1.5">
								{report.reporter_id}
								<Button
									variant="link"
									size="xs"
									onClick={() => navigate({ to: '/admin/users/detail', search: { id: report.reporter_id } })}
								>
									查看举报人
								</Button>
							</div>
						</DescriptionsItem>
						<DescriptionsItem label="状态">
							{statusCfg ? <Badge variant={statusCfg.variant}>{statusCfg.text}</Badge> : report.status}
						</DescriptionsItem>
						<DescriptionsItem label="创建时间">{formatTime(report.created_at)}</DescriptionsItem>
						<DescriptionsItem label="更新时间">{formatTime(report.updated_at)}</DescriptionsItem>
						<DescriptionsItem label="处理人">{report.handled_by || '-'}</DescriptionsItem>
						<DescriptionsItem label="处理动作">{report.action || '-'}</DescriptionsItem>
						<DescriptionsItem label="处理时间">
							{report.handled_at ? formatTime(report.handled_at) : '-'}
						</DescriptionsItem>
						<DescriptionsItem label="备注" span={2}>
							{report.note || '-'}
						</DescriptionsItem>
					</Descriptions>
				)}
			</Card>

			<Card className="p-5">
				<div className="inline-flex items-center gap-1.5">
					{report?.status === 'pending' && (
						<>
							<Button
								onClick={() => {
									setResolveAction('none');
									setResolveNote('');
									setResolveOpen(true);
								}}
							>
								处理
							</Button>
							<Button onClick={() => handleUpdateStatus('dismissed')}>驳回</Button>
						</>
					)}
					<Button variant="destructive" size="sm" onClick={handleDelete}>
						删除
					</Button>

					<Button onClick={viewTarget} variant="outline">
						查看被举报内容
					</Button>
				</div>
			</Card>

			<Dialog open={resolveOpen} onOpenChange={(v) => !actionLoading && setResolveOpen(v)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>处理举报</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-3">
						<div className="flex flex-col gap-1.5">
							<Label>处理动作</Label>
							<Select value={resolveAction} onValueChange={setResolveAction}>
								<SelectTrigger className="w-full">
									<SelectValue placeholder="选择动作" />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="none">仅标记已处理</SelectItem>
									<SelectItem value="delete_target">删除目标内容</SelectItem>
									<SelectItem value="lock_post">锁定帖子</SelectItem>
									<SelectItem value="ban_user">禁言作者</SelectItem>
								</SelectContent>
							</Select>
						</div>
						<div className="flex flex-col gap-1.5">
							<Label htmlFor="resolve-note">备注</Label>
							<Textarea
								id="resolve-note"
								rows={3}
								value={resolveNote}
								onChange={(e) => setResolveNote(e.target.value)}
								placeholder="可选备注，禁言时可作为原因"
							/>
						</div>
					</div>
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setResolveOpen(false)}>
							取消
						</Button>
						<Button
							size="sm"
							loading={actionLoading}
							onClick={() => handleUpdateStatus('resolved', resolveAction, resolveNote)}
						>
							确认处理
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}
