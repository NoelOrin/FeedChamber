'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import { type OAuthClient, type OAuthClientSecretResult, oauthClientService } from '@orinforum/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { useCallback, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

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

/* redirect_uris 是 JSON 数组字符串，原来完全不做校验就提交。
   这里要求必须是字符串数组，非法 JSON 在字段级就拦下。 */
const clientSchema = z.object({
	name: z.string().min(1, '请输入客户端名称'),
	redirect_uris: z.string().refine((v) => {
		try {
			const parsed = JSON.parse(v);
			return Array.isArray(parsed) && parsed.every((u) => typeof u === 'string');
		} catch {
			return false;
		}
	}, '必须是 JSON 字符串数组，例如 ["https://example.com/cb"]'),
	scopes: z.string().min(1, '请输入 scopes'),
});

type ClientValues = z.infer<typeof clientSchema>;

export default function OAuthClients() {
	const queryClient = useQueryClient();

	const { data = [], isPending: loading } = useQuery({
		queryKey: qk.admin.oauthClients.list,
		queryFn: async () => {
			const items = await oauthClientService.list();
			return items ?? [];
		},
	});

	/** 写操作后让该模块的列表缓存失效。 */
	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: qk.admin.oauthClients.all }),
		[queryClient],
	);
	const [createOpen, setCreateOpen] = useState(false);
	const [creating, setCreating] = useState(false);
	const [secret, setSecret] = useState<OAuthClientSecretResult | null>(null);
	const [confirm, confirmDialog] = useConfirm();

	const form = useForm<ClientValues>({
		resolver: zodResolver(clientSchema),
		defaultValues: { name: '', redirect_uris: '[]', scopes: 'read' },
	});

	async function handleCreate(values: ClientValues) {
		setCreating(true);
		try {
			const res = await oauthClientService.create({
				name: values.name,
				redirect_uris: values.redirect_uris,
				scopes: values.scopes,
			});
			setSecret(res);
			setCreateOpen(false);
			form.reset({ name: '', redirect_uris: '[]', scopes: 'read' });
			invalidate();
		} catch (e: any) {
			toast.error(e.message || '创建失败');
		} finally {
			setCreating(false);
		}
	}

	const handleResetSecret = useCallback(async (id: number) => {
		try {
			const res = await oauthClientService.resetSecret(id);
			setSecret(res);
		} catch (e: any) {
			toast.error(e.message || '重置密钥失败');
		}
	}, []);

	const handleDelete = useCallback(
		async (id: number) => {
			try {
				await oauthClientService.delete(id);
				toast.success('已删除');
				invalidate();
			} catch (e: any) {
				toast.error(e.message || '删除失败');
			}
		},
		[invalidate],
	);

	const columns = useMemo<ColumnDef<OAuthClient>[]>(
		() => [
			{ accessorKey: 'id', header: 'ID', size: 80 },
			{ accessorKey: 'name', header: '名称', size: 180 },
			{ accessorKey: 'client_id', header: 'Client ID' },
			{ accessorKey: 'scopes', header: 'Scopes', size: 180 },
			{ accessorKey: 'redirect_uris', header: 'Redirect URIs' },
			{ accessorKey: 'created_by', header: '创建人', size: 160 },
			{
				id: 'actions',
				header: '操作',
				size: 180,
				cell: ({ row }) => (
					<div className="flex items-center gap-1.5">
						<Button variant="outline" size="xs" onClick={() => handleResetSecret(row.original.id)}>
							重置密钥
						</Button>
						<Button
							variant="destructive"
							size="xs"
							onClick={() =>
								confirm({
									title: '删除该客户端？',
									description: '使用该客户端授权的集成会立即失效，且不可恢复。',
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
		[confirm, handleResetSecret, handleDelete],
	);

	return (
		<div>
			<PageHeader
				title="OAuth 客户端"
				actions={
					<Button size="sm" onClick={() => setCreateOpen(true)}>
						创建客户端
					</Button>
				}
			/>
			<DataTable columns={columns} data={data} loading={loading} getRowId={(row) => String(row.id)} />

			<Dialog open={createOpen} onOpenChange={(v) => !creating && setCreateOpen(v)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>创建 OAuth 客户端</DialogTitle>
					</DialogHeader>
					<Form {...form}>
						<form onSubmit={form.handleSubmit(handleCreate)} className="flex flex-col gap-3">
							<FormField
								control={form.control}
								name="name"
								render={({ field }) => (
									<FormItem>
										<FormLabel>名称</FormLabel>
										<FormControl>
											<Input placeholder="客户端名称" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={form.control}
								name="redirect_uris"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Redirect URIs（JSON 数组）</FormLabel>
										<FormControl>
											<Textarea rows={4} {...field} />
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
										<FormMessage />
									</FormItem>
								)}
							/>
							<DialogFooter className="mt-2 gap-2">
								<Button type="button" variant="outline" size="sm" onClick={() => setCreateOpen(false)}>
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

			<Dialog open={Boolean(secret)} onOpenChange={(v) => !v && setSecret(null)}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>客户端密钥</DialogTitle>
					</DialogHeader>
					{secret ? (
						<div className="flex flex-col gap-2 text-[13px]">
							<p className="text-muted-foreground">
								Client ID：
								<code className="bg-muted rounded px-1.5 py-0.5 text-foreground select-all">{secret.client_id}</code>
							</p>
							<p className="text-muted-foreground">
								Client Secret：
								<code className="bg-muted rounded px-1.5 py-0.5 text-foreground select-all">
									{secret.client_secret}
								</code>
							</p>
							<p className="text-warning">密钥只显示这一次，请立即保存。</p>
						</div>
					) : null}
					<DialogFooter>
						<Button size="sm" onClick={() => setSecret(null)}>
							我已保存
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{confirmDialog}
		</div>
	);
}
