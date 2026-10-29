'use client';

import { type Invite, inviteService } from '@feedchamber/api-client';
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
import { Plus, RefreshCw } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const createSchema = z.object({
	code: z.string().max(32, '最多 32 个字符').optional(),
	max_uses: z
		.string()
		.optional()
		.refine((v) => !v || /^\d+$/.test(v), '请输入非负整数'),
});

type CreateValues = z.infer<typeof createSchema>;

export default function InvitesPage() {
	const [page, setPage] = useState(1);
	const [pageSize] = useState(20);
	const queryClient = useQueryClient();

	const {
		data: res,
		isPending: loading,
		refetch,
	} = useQuery({
		queryKey: qk.admin.invites.list,
		queryFn: () => inviteService.list({ page, page_size: pageSize }),
	});

	const items = res?.items ?? [];
	const total = res?.total ?? 0;

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.invites.all }),
		[queryClient],
	);
	const [open, setOpen] = useState(false);
	const [creating, setCreating] = useState(false);
	const [confirm, confirmDialog] = useConfirm();

	const form = useForm<CreateValues>({
		resolver: zodResolver(createSchema),
		defaultValues: { code: '', max_uses: '1' },
	});

	async function handleCreate(values: CreateValues) {
		setCreating(true);
		try {
			await inviteService.create({
				code: values.code || undefined,
				max_uses: values.max_uses ? Number(values.max_uses) : 0,
			});
			toast.success('邀请码已创建');
			setOpen(false);
			form.reset({ code: '', max_uses: '1' });
			invalidate();
		} catch (e: any) {
			toast.error(e?.message || '创建失败');
		} finally {
			setCreating(false);
		}
	}

	const handleDelete = useCallback(
		async (id: string) => {
			try {
				await inviteService.delete(id);
				toast.success('已删除');
				invalidate();
			} catch (e: any) {
				toast.error(e?.message || '删除失败');
			}
		},
		[invalidate],
	);

	const columns = useMemo<ColumnDef<Invite>[]>(
		() => [
			{
				accessorKey: 'code',
				header: '邀请码',
				cell: ({ getValue }) => <Badge variant="primary">{getValue<string>()}</Badge>,
			},
			{
				id: 'usage',
				header: '使用情况',
				cell: ({ row }) => (
					<span className="tabular-nums">
						{row.original.used_count} / {row.original.max_uses === 0 ? '∞' : row.original.max_uses}
					</span>
				),
			},
			{
				accessorKey: 'expires_at',
				header: '过期时间',
				cell: ({ getValue }) => {
					const v = getValue<string | null>();
					return v ? new Date(v).toLocaleString() : '不过期';
				},
			},
			{
				accessorKey: 'created_at',
				header: '创建时间',
				cell: ({ getValue }) => {
					const v = getValue<string>();
					return v ? new Date(v).toLocaleString() : '-';
				},
			},
			{
				id: 'actions',
				header: '操作',
				size: 100,
				cell: ({ row }) => (
					<Button
						variant="ghost"
						size="xs"
						className="text-destructive hover:bg-destructive/10"
						onClick={() =>
							confirm({
								title: '删除该邀请码？',
								description: '已使用该码注册的用户不受影响，但该码不能再被使用。',
								danger: true,
								confirmText: '删除',
								onConfirm: () => handleDelete(row.original.id),
							})
						}
					>
						删除
					</Button>
				),
			},
		],
		[confirm, handleDelete],
	);

	return (
		<div>
			<PageHeader
				title="邀请码管理"
				actions={
					<div className="flex items-center gap-2">
						<Button variant="outline" size="sm" onClick={() => refetch()}>
							<RefreshCw className="size-3.5" />
							刷新
						</Button>
						<Button size="sm" onClick={() => setOpen(true)}>
							<Plus className="size-3.5" />
							创建邀请码
						</Button>
					</div>
				}
			/>
			<DataTable
				columns={columns}
				data={items}
				loading={loading}
				getRowId={(row) => row.id}
				pagination={{ page, pageSize, total, onChange: setPage }}
			/>

			<Dialog open={open} onOpenChange={(v) => !creating && setOpen(v)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>创建邀请码</DialogTitle>
					</DialogHeader>
					<Form {...form}>
						<form onSubmit={form.handleSubmit(handleCreate)} className="flex flex-col gap-3">
							<FormField
								control={form.control}
								name="code"
								render={({ field }) => (
									<FormItem>
										<FormLabel>自定义邀请码（可选）</FormLabel>
										<FormControl>
											<Input placeholder="留空则自动生成" maxLength={32} {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="max_uses"
								render={({ field }) => (
									<FormItem>
										<FormLabel>最大使用次数</FormLabel>
										<FormControl>
											<Input type="number" min={0} {...field} />
										</FormControl>
										<FormDescription>0 表示无限次</FormDescription>
										<FormMessage />
									</FormItem>
								)}
							/>
							<DialogFooter className="mt-2 gap-2">
								<Button type="button" variant="outline" size="sm" onClick={() => setOpen(false)}>
									取消
								</Button>
								<Button type="submit" size="sm" loading={creating}>
									创建
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
