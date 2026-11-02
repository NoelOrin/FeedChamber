'use client';

import { type BannedUser, bannedService, userService } from '@feedchamber/api-client';
import { formatTime } from '@feedchamber/shared-utils';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { Switch } from '~/components/ui/switch';
import { Textarea } from '~/components/ui/textarea';
import { RefreshCw } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/*
 * 禁言表单：原来「是否永久」是 Select(false/true) + Form.Item 的 shouldUpdate
 * 条件渲染到期时间。现在用 Switch + zod 的 superRefine：非永久时 expires_at
 * 必填，规则写在 schema 里而不是渲染逻辑里。
 *
 * 到期时间用原生 <input type="datetime-local">：只有这一处用到日期选择，
 * 引 react-day-picker 只为它不值。
 */
const banSchema = z
	.object({
		user_id: z.string().min(1, '请输入用户 ID'),
		reason: z.string().min(1, '请输入禁言原因'),
		is_permanent: z.boolean(),
		expires_at: z.string().optional(),
	})
	.superRefine((v, ctx) => {
		if (!v.is_permanent && !v.expires_at) {
			ctx.addIssue({ code: 'custom', message: '请选择到期时间', path: ['expires_at'] });
		}
	});

type BanValues = z.infer<typeof banSchema>;

const EMPTY: BanValues = { user_id: '', reason: '', is_permanent: false, expires_at: '' };

export default function AdminBanned() {
	const [modalVisible, setModalVisible] = useState(false);
	const [submitting, setSubmitting] = useState(false);
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const form = useForm<BanValues>({ resolver: zodResolver(banSchema), defaultValues: EMPTY });
	const isPermanent = form.watch('is_permanent');

	const {
		data: res,
		isPending: loading,
		refetch,
	} = useQuery({
		queryKey: qk.admin.banned.list(),
		queryFn: () => bannedService.list(),
	});

	const data = res?.items ?? [];

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(() => queryClient.invalidateQueries({ queryKey: qk.admin.banned.all }), [queryClient]);

	const handleUnban = useCallback(
		(userId: string, username: string) => {
			confirm({
				title: '解禁该用户？',
				description: `用户 ${username} 将恢复发帖与评论权限。`,
				confirmText: '解禁',
				onConfirm: async () => {
					try {
						await userService.unban(userId);
						toast.success('解禁成功');
						invalidate();
					} catch (error: any) {
						toast.error(error.message || '解禁失败');
					}
				},
			});
		},
		[confirm, invalidate],
	);

	async function handleBan(values: BanValues) {
		setSubmitting(true);
		try {
			const params: any = { reason: values.reason, permanent: values.is_permanent };
			// datetime-local 给的是本地时间字符串（无时区），转 ISO 后再发给后端
			if (!values.is_permanent && values.expires_at) params.expires_at = new Date(values.expires_at).toISOString();
			await userService.ban(values.user_id, params);
			toast.success('禁言成功');
			setModalVisible(false);
			form.reset(EMPTY);
			invalidate();
		} catch (error: any) {
			toast.error(error.message || '禁言失败');
		} finally {
			setSubmitting(false);
		}
	}

	const columns = useMemo<ColumnDef<BannedUser>[]>(
		() => [
			{ accessorKey: 'username', header: '用户名', size: 150 },
			{ accessorKey: 'email', header: '邮箱', size: 200 },
			{ accessorKey: 'reason', header: '原因', size: 200 },
			{
				accessorKey: 'banned_at',
				header: '禁言时间',
				size: 180,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{
				accessorKey: 'expires_at',
				header: '到期时间',
				size: 180,
				cell: ({ row }) =>
					row.original.is_permanent ? <Badge variant="destructive">永久</Badge> : formatTime(row.original.expires_at),
			},
			{
				id: 'status',
				header: '状态',
				size: 120,
				cell: ({ row }) => {
					if (row.original.is_permanent) return <Badge variant="destructive">永久封禁</Badge>;
					if (row.original.expires_at && new Date(row.original.expires_at) < new Date())
						return <Badge variant="outline">已过期</Badge>;
					return <Badge variant="warning">封禁中</Badge>;
				},
			},
			{
				id: 'actions',
				header: '操作',
				size: 100,
				cell: ({ row }) => (
					<Button variant="link" size="xs" onClick={() => handleUnban(row.original.user_id, row.original.username)}>
						解禁
					</Button>
				),
			},
		],
		[handleUnban],
	);

	return (
		<div>
			<PageHeader
				title="禁言管理"
				actions={
					<div className="flex items-center gap-2">
						<Button size="sm" onClick={() => setModalVisible(true)}>
							手动禁言
						</Button>
						<Button variant="outline" size="sm" onClick={() => refetch()}>
							<RefreshCw className="size-3.5" />
							刷新
						</Button>
					</div>
				}
			/>
			<DataTable columns={columns} data={data} loading={loading} getRowId={(row) => String(row.id)} />

			<Dialog open={modalVisible} onOpenChange={(v) => !submitting && setModalVisible(v)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>手动禁言</DialogTitle>
					</DialogHeader>
					<Form {...form}>
						<form onSubmit={form.handleSubmit(handleBan)} className="flex flex-col gap-3">
							<FormField
								control={form.control}
								name="user_id"
								render={({ field }) => (
									<FormItem>
										<FormLabel>用户 ID</FormLabel>
										<FormControl>
											<Input placeholder="请输入用户 ID" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="reason"
								render={({ field }) => (
									<FormItem>
										<FormLabel>禁言原因</FormLabel>
										<FormControl>
											<Textarea rows={3} placeholder="请输入禁言原因" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="is_permanent"
								render={({ field }) => (
									<FormItem className="flex flex-row items-center justify-between">
										<FormLabel>永久禁言</FormLabel>
										<FormControl>
											<Switch checked={field.value} onCheckedChange={field.onChange} />
										</FormControl>
									</FormItem>
								)}
							/>
							{!isPermanent ? (
								<FormField
									control={form.control}
									name="expires_at"
									render={({ field }) => (
										<FormItem>
											<FormLabel>到期时间</FormLabel>
											<FormControl>
												<Input type="datetime-local" {...field} />
											</FormControl>
											<FormDescription>到达该时间后自动解禁</FormDescription>
											<FormMessage />
										</FormItem>
									)}
								/>
							) : null}
							<DialogFooter className="mt-2 gap-2">
								<Button type="button" variant="outline" size="sm" onClick={() => setModalVisible(false)}>
									取消
								</Button>
								<Button type="submit" size="sm" loading={submitting}>
									确认禁言
								</Button>
							</DialogFooter>
						</form>
					</Form>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
