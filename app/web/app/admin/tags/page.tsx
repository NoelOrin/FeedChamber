'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { type Tag as TagItem, tagService } from '@orinforum/api-client';
import { formatTime } from '@orinforum/shared-utils';
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
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { RefreshCw } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

const tagSchema = z.object({ name: z.string().min(1, '请输入标签名称') });
type TagValues = z.infer<typeof tagSchema>;

export default function TagList() {
	const [page] = useState(1);
	const [pageSize] = useState(20);
	const [modalOpen, setModalOpen] = useState(false);
	const [editingTag, setEditingTag] = useState<TagItem | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const isEdit = editingTag !== null;
	const form = useForm<TagValues>({ resolver: zodResolver(tagSchema), defaultValues: { name: '' } });

	const {
		data: res,
		isPending: loading,
		refetch,
	} = useQuery({
		queryKey: qk.admin.tags.list({ page, pageSize }),
		queryFn: () => tagService.list(),
	});

	const data = res?.data ?? [];
	const total = res?.total ?? 0;

	/** 写操作后让该模块的列表缓存失效，交由 React Query 重新取数。 */
	const invalidate = useCallback(() => queryClient.invalidateQueries({ queryKey: qk.admin.tags.all }), [queryClient]);

	const openEdit = useCallback(
		(record: TagItem) => {
			setEditingTag(record);
			form.reset({ name: record.name });
			setModalOpen(true);
		},
		[form],
	);

	async function handleSubmit(values: TagValues) {
		setSubmitting(true);
		try {
			if (isEdit && editingTag) {
				await tagService.update(editingTag.id, values.name);
				toast.success('更新成功');
			} else {
				await tagService.create(values.name);
				toast.success('创建成功');
			}
			setModalOpen(false);
			setEditingTag(null);
			form.reset({ name: '' });
			invalidate();
		} catch (e: any) {
			toast.error(e.message || '操作失败');
		} finally {
			setSubmitting(false);
		}
	}

	const handleDelete = useCallback(
		async (id: string) => {
			try {
				await tagService.delete(id);
				toast.success('删除成功');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '删除失败');
			}
		},
		[invalidate],
	);

	const columns = useMemo<ColumnDef<TagItem>[]>(
		() => [
			{
				accessorKey: 'name',
				header: '标签名称',
				size: 200,
				cell: ({ getValue }) => <Badge variant="secondary">{getValue<string>()}</Badge>,
			},
			{
				accessorKey: 'created_at',
				header: '创建时间',
				size: 180,
				cell: ({ getValue }) => formatTime(getValue<string>()),
			},
			{
				id: 'actions',
				header: '操作',
				size: 160,
				cell: ({ row }) => (
					<div className="flex items-center gap-1.5">
						<Button variant="outline" size="xs" onClick={() => openEdit(row.original)}>
							编辑
						</Button>
						<Button
							variant="destructive"
							size="xs"
							onClick={() =>
								confirm({
									title: '删除该标签？',
									description: '标签会从所有已关联的帖子上移除，帖子本身不受影响。',
									danger: true,
									confirmText: '删除',
									onConfirm: () => handleDelete(row.original.id),
								})
							}
						>
							删除
						</Button>
					</div>
				),
			},
		],
		[confirm, openEdit, handleDelete],
	);

	return (
		<div>
			<PageHeader
				title="标签管理"
				actions={
					<div className="flex items-center gap-2">
						<Button
							size="sm"
							onClick={() => {
								setEditingTag(null);
								form.reset({ name: '' });
								setModalOpen(true);
							}}
						>
							新建标签
						</Button>
						<Button variant="outline" size="sm" onClick={() => refetch()}>
							<RefreshCw className="size-3.5" />
							刷新
						</Button>
					</div>
				}
			/>
			<DataTable
				columns={columns}
				data={data}
				loading={loading}
				getRowId={(row) => row.id}
				pagination={{ page, pageSize, total, onChange: () => {} }}
			/>

			<Dialog
				open={modalOpen}
				onOpenChange={(v) => {
					if (submitting) return;
					setModalOpen(v);
					if (!v) {
						setEditingTag(null);
						form.reset({ name: '' });
					}
				}}
			>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>{isEdit ? '编辑标签' : '新建标签'}</DialogTitle>
					</DialogHeader>
					<Form {...form}>
						<form onSubmit={form.handleSubmit(handleSubmit)} className="flex flex-col gap-3">
							<FormField
								control={form.control}
								name="name"
								render={({ field }) => (
									<FormItem>
										<FormLabel>标签名称</FormLabel>
										<FormControl>
											<Input placeholder="请输入标签名称" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<DialogFooter className="mt-2 gap-2">
								<Button type="button" variant="outline" size="sm" onClick={() => setModalOpen(false)}>
									取消
								</Button>
								<Button type="submit" size="sm" loading={submitting}>
									保存
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
