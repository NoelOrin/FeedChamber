'use client';

import { type APIKey, apiKeyService } from '@feedchamber/api-client';
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
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '~/components/ui/form';
import { Input } from '~/components/ui/input';
import { PageHeader } from '~/components/ui/page-header';
import { Textarea } from '~/components/ui/textarea';
import { qk } from '~/lib/query/keys';
import { toast } from '~/lib/toast';

/* 校验规则从 antd 的 rules 数组迁到 zod schema：
   name 必填；rate_limit / expires_at 是非负整数（空值视为 0 / 不过期）。 */
const createSchema = z.object({
	name: z.string().min(1, '请输入名称'),
	scopes: z.string().optional(),
	// 输入框给的是字符串；用 refine 校验非负整数，提交时再转 number，
	// 避免 z.coerce 让 input/output 类型分叉（RHF 的 resolver 会因此失配）。
	rate_limit: z
		.string()
		.optional()
		.refine((v) => !v || /^\d+$/.test(v), '请输入非负整数'),
	expires_at: z
		.string()
		.optional()
		.refine((v) => !v || /^\d+$/.test(v), '请输入非负整数'),
});

type CreateValues = z.infer<typeof createSchema>;

export default function ApiKeyList() {
	const [modalOpen, setModalOpen] = useState(false);
	const [saving, setSaving] = useState(false);
	const [createdKey, setCreatedKey] = useState('');
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const form = useForm<CreateValues>({
		resolver: zodResolver(createSchema),
		defaultValues: { name: '', scopes: '', rate_limit: '', expires_at: '' },
	});

	const { data = [], isPending: loading } = useQuery({
		queryKey: qk.admin.apiKeys.list,
		queryFn: async () => {
			const res = await apiKeyService.list();
			return Array.isArray(res) ? res : [];
		},
	});

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.apiKeys.all }),
		[queryClient],
	);

	function openCreate() {
		setCreatedKey('');
		form.reset({ name: '', scopes: '', rate_limit: '', expires_at: '' });
		setModalOpen(true);
	}

	async function handleSubmit(values: CreateValues) {
		setSaving(true);
		try {
			const res = await apiKeyService.create({
				name: values.name,
				scopes: values.scopes || '',
				rate_limit: values.rate_limit ? Number(values.rate_limit) : 0,
				expires_at: values.expires_at ? Number(values.expires_at) : undefined,
			});
			setCreatedKey(res.api_key);
			setModalOpen(false);
			invalidate();
		} catch (e: any) {
			toast.error(e.message || '创建失败');
		} finally {
			setSaving(false);
		}
	}

	const handleRevoke = useCallback(
		async (id: number) => {
			try {
				await apiKeyService.revoke(id);
				toast.success('已吊销');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '吊销失败');
			}
		},
		[invalidate],
	);

	const columns = useMemo<ColumnDef<APIKey>[]>(
		() => [
			{ accessorKey: 'name', header: '名称', size: 160 },
			{ accessorKey: 'key_id', header: 'Key ID', size: 200 },
			{ accessorKey: 'scopes', header: 'Scopes' },
			{ accessorKey: 'rate_limit', header: '限流', size: 80 },
			{
				accessorKey: 'status',
				header: '状态',
				size: 100,
				cell: ({ getValue }) => {
					const v = getValue<string>();
					return <Badge variant={v === 'active' ? 'primary' : 'destructive'}>{v}</Badge>;
				},
			},
			{
				id: 'actions',
				header: '操作',
				size: 100,
				cell: ({ row }) => (
					<Button
						variant="destructive"
						size="xs"
						disabled={row.original.status !== 'active'}
						onClick={() =>
							confirm({
								title: '吊销该 API Key？',
								description: '使用该密钥的集成会立即失效，不可恢复。',
								danger: true,
								confirmText: '吊销',
								onConfirm: () => handleRevoke(row.original.id),
							})
						}
					>
						吊销
					</Button>
				),
			},
		],
		[confirm, handleRevoke],
	);

	return (
		<div>
			<PageHeader
				title="API Key 管理"
				actions={
					<Button size="sm" onClick={openCreate}>
						新建 API Key
					</Button>
				}
			/>
			<DataTable columns={columns} data={data} loading={loading} getRowId={(row) => String(row.id)} />

			<Dialog open={modalOpen} onOpenChange={(v) => !saving && setModalOpen(v)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>新建 API Key</DialogTitle>
					</DialogHeader>
					<Form {...form}>
						<form onSubmit={form.handleSubmit(handleSubmit)} className="flex flex-col gap-3">
							<FormField
								control={form.control}
								name="name"
								render={({ field }) => (
									<FormItem>
										<FormLabel>名称</FormLabel>
										<FormControl>
											<Input {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="scopes"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Scopes</FormLabel>
										<FormControl>
											<Input {...field} />
										</FormControl>
										<FormMessage>逗号分隔，例如 read,write</FormMessage>
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="rate_limit"
								render={({ field }) => (
									<FormItem>
										<FormLabel>每分钟限流</FormLabel>
										<FormControl>
											<Input type="number" min={0} {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="expires_at"
								render={({ field }) => (
									<FormItem>
										<FormLabel>过期时间</FormLabel>
										<FormControl>
											<Input type="number" min={0} placeholder="Unix 时间戳，留空不过期" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<DialogFooter className="mt-2 gap-2">
								<Button type="button" variant="outline" size="sm" onClick={() => setModalOpen(false)}>
									取消
								</Button>
								<Button type="submit" size="sm" loading={saving}>
									创建
								</Button>
							</DialogFooter>
						</form>
					</Form>
				</DialogContent>
			</Dialog>

			<Dialog open={!!createdKey} onOpenChange={(v) => !v && setCreatedKey('')}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>API Key 已创建</DialogTitle>
					</DialogHeader>
					<p className="text-[13px] text-muted-foreground">请立即保存，该密钥只会显示一次。</p>
					<Textarea value={createdKey} readOnly rows={3} />
					<DialogFooter>
						<Button size="sm" onClick={() => setCreatedKey('')}>
							我已保存
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
