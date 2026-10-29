'use client';

import { type Backup, backupService } from '@feedchamber/api-client';
import { formatTime } from '@feedchamber/shared-utils';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';

import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { DataTable } from '~/components/ui/data-table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Download } from '~/lib/icons';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

export default function BackupList() {
	const [modalOpen, setModalOpen] = useState(false);
	const [name, setName] = useState('');
	const [creating, setCreating] = useState(false);
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const { data = [], isPending: loading } = useQuery({
		queryKey: qk.admin.backups.list,
		queryFn: async () => {
			const res = await backupService.list({ page: 1, page_size: 100 });
			return res.data ?? [];
		},
	});

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.backups.all }),
		[queryClient],
	);

	async function handleCreate() {
		setCreating(true);
		try {
			await backupService.create({ name: name || undefined });
			toast.success('备份已创建');
			setModalOpen(false);
			setName('');
			invalidate();
		} catch (e: any) {
			toast.error(e.message || '创建失败');
		} finally {
			setCreating(false);
		}
	}

	async function handleCleanup() {
		try {
			const res = await backupService.cleanup();
			toast.success(`已清理 ${res?.removed ?? 0} 个过期备份`);
			invalidate();
		} catch (e: any) {
			toast.error(e.message || '清理失败');
		}
	}

	const handleDelete = useCallback(
		async (id: string) => {
			try {
				await backupService.delete(id);
				toast.success('已删除');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '删除失败');
			}
		},
		[invalidate],
	);

	const handleRestore = useCallback(
		async (id: string) => {
			try {
				await backupService.restore(id);
				toast.success('已从备份恢复');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '恢复失败');
			}
		},
		[invalidate],
	);

	const handleDownload = useCallback(async (record: Backup) => {
		try {
			const blob = await backupService.download(record.id);
			const url = URL.createObjectURL(blob);
			const anchor = document.createElement('a');
			anchor.href = url;
			anchor.download = `${record.name}.json`;
			anchor.click();
			URL.revokeObjectURL(url);
		} catch (e: any) {
			toast.error(e.message || '下载失败');
		}
	}, []);

	const columns = useMemo<ColumnDef<Backup>[]>(
		() => [
			{ accessorKey: 'name', header: '名称' },
			{ accessorKey: 'size', header: '大小', cell: ({ getValue }) => `${getValue<number>()} B` },
			{
				accessorKey: 'uploads_size',
				header: '上传归档',
				cell: ({ getValue }) => {
					const v = getValue<number | undefined>();
					return v ? `${v} B` : '-';
				},
			},
			{ accessorKey: 'checksum', header: '校验和' },
			{ accessorKey: 'status', header: '状态' },
			{
				accessorKey: 'created_at',
				header: '创建时间',
				cell: ({ getValue }) => {
					const v = getValue<string | undefined>();
					return v ? formatTime(v) : '-';
				},
			},
			{
				id: 'actions',
				header: '操作',
				size: 260,
				cell: ({ row }) => (
					<div className="flex items-center gap-1.5">
						<Button variant="outline" size="xs" onClick={() => handleDownload(row.original)}>
							<Download className="size-3" />
							下载
						</Button>
						<Button
							variant="outline"
							size="xs"
							onClick={() =>
								confirm({
									title: '从该备份恢复？',
									description: '当前数据库数据将被这份备份覆盖，操作不可撤销。',
									danger: true,
									confirmText: '恢复',
									onConfirm: () => handleRestore(row.original.id),
								})
							}
						>
							恢复
						</Button>
						<Button
							variant="destructive"
							size="xs"
							onClick={() =>
								confirm({
									title: '删除该备份？',
									description: '备份文件会被永久删除，不可恢复。',
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
		[confirm, handleDownload, handleRestore, handleDelete],
	);

	return (
		<div>
			<PageHeader
				title="备份管理"
				actions={
					<div className="flex items-center gap-2">
						<Button variant="outline" size="sm" onClick={handleCleanup}>
							清理过期备份
						</Button>
						<Button size="sm" onClick={() => setModalOpen(true)}>
							创建备份
						</Button>
					</div>
				}
			/>
			<DataTable columns={columns} data={data} loading={loading} getRowId={(row) => row.id} />

			<Dialog open={modalOpen} onOpenChange={(v) => !creating && setModalOpen(v)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>创建备份</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-1.5">
						<Label htmlFor="backup-name">备份名称</Label>
						<Input id="backup-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="留空自动生成" />
					</div>
					<DialogFooter className="mt-2 gap-2">
						<Button variant="outline" size="sm" onClick={() => setModalOpen(false)}>
							取消
						</Button>
						<Button size="sm" loading={creating} onClick={handleCreate}>
							创建
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
