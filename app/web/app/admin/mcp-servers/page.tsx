'use client';

import { type MCPServer, type MCPServerInput, mcpServerService } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';

import { Alert, AlertDescription } from '~/components/ui/alert';
import { Badge } from '~/components/ui/badge';
import { Button } from '~/components/ui/button';
import { useConfirm } from '~/components/ui/confirm-dialog';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '~/components/ui/dialog';
import { Input } from '~/components/ui/input';
import { Label } from '~/components/ui/label';
import { PageHeader } from '~/components/ui/page-header';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '~/components/ui/select';
import { Switch } from '~/components/ui/switch';
import { Textarea } from '~/components/ui/textarea';
import { toast } from '~/lib/toast';

/**
 * 外部 MCP Server 登记（P3：把外部 MCP 当能力源）。
 *
 * 与「MCP 工具」页方向相反：那边是本站对外暴露，这里是本站去调别人。
 * 审核用途的服务会在 moderation_mode=external 时被优先调用。
 */
export default function MCPServersPage() {
	const [open, setOpen] = useState(false);
	const [editing, setEditing] = useState<MCPServer | null>(null);
	const [name, setName] = useState('');
	const [endpoint, setEndpoint] = useState('');
	const [purpose, setPurpose] = useState<'general' | 'moderation'>('general');
	const [moderationTool, setModerationTool] = useState('');
	const [headers, setHeaders] = useState('');
	const [enabled, setEnabled] = useState(false);
	const [saving, setSaving] = useState(false);
	const [testing, setTesting] = useState<string | null>(null);
	const [confirm, confirmDialog] = useConfirm();
	const queryClient = useQueryClient();

	const query = useQuery({
		queryKey: ['admin', 'mcp-servers'],
		queryFn: async () => (await mcpServerService.list()) ?? [],
	});
	const data = query.data ?? [];
	const isPending = query.isPending;
	const isError = query.isError;
	const errorMessage = query.error instanceof Error ? query.error.message : '未知错误';

	const invalidate = useCallback(
		() => queryClient.invalidateQueries({ queryKey: ['admin', 'mcp-servers'] }),
		[queryClient],
	);

	const openCreate = useCallback(() => {
		setEditing(null);
		setName('');
		setEndpoint('');
		setPurpose('general');
		setModerationTool('');
		setHeaders('');
		setEnabled(false);
		setOpen(true);
	}, []);

	const openEdit = useCallback((s: MCPServer) => {
		setEditing(s);
		setName(s.name);
		setEndpoint(s.endpoint);
		setPurpose(s.purpose);
		setModerationTool(s.moderation_tool ?? '');
		// 留空 = 保持原凭据不变（接口只返回键名，返回脱敏值）。
		setHeaders('');
		setEnabled(s.enabled);
		setOpen(true);
	}, []);

	const save = useCallback(async () => {
		if (!name.trim() || !endpoint.trim()) {
			toast.error('名称与端点不能为空');
			return;
		}
		setSaving(true);
		try {
			const body: MCPServerInput = {
				name,
				endpoint,
				transport: 'streamable_http',
				purpose,
				moderation_tool: moderationTool,
				enabled,
				...(headers.trim() ? { headers } : {}),
			};
			if (editing) await mcpServerService.update(editing.id, body);
			else await mcpServerService.create(body);
			toast.success('已保存');
			setOpen(false);
			await invalidate();
		} catch (err) {
			toast.error((err as Error).message);
		} finally {
			setSaving(false);
		}
	}, [name, endpoint, purpose, moderationTool, headers, enabled, editing, invalidate]);

	const test = useCallback(async (s: MCPServer) => {
		setTesting(s.id);
		try {
			const res = await mcpServerService.test(s.id);
			if (res?.ok) {
				toast.success(`连通正常，暴露 ${res.tools?.length ?? 0} 个工具`);
			} else {
				toast.error(`连接失败：${res?.error ?? '未知原因'}`);
			}
		} catch (err) {
			toast.error((err as Error).message);
		} finally {
			setTesting(null);
		}
	}, []);

	const remove = useCallback(
		(s: MCPServer) => {
			// useConfirm 是回调式的：确认后执行 onConfirm，不返回 Promise。
			confirm({
				title: '删除外部 MCP 服务',
				description: `确定删除「${s.name}」吗？`,
				danger: true,
				onConfirm: async () => {
					try {
						await mcpServerService.delete(s.id);
						toast.success('已删除');
						await invalidate();
					} catch (err) {
						toast.error((err as Error).message);
					}
				},
			});
		},
		[confirm, invalidate],
	);

	return (
		<div className="flex flex-col gap-4">
			<PageHeader
				title="外部 MCP 服务"
				meta={`${data.length} 个`}
				actions={<Button onClick={openCreate}>登记服务</Button>}
			/>

			<Alert>
				<AlertDescription>
					用途为「内容审核」的服务会在审核模式为 external 时被优先调用（MCP tools/call）；
					未配置或调用失败时自动回退到既有的 HTTP 审核契约。鉴权头在响应中只返回值键名，不返回凭据。
				</AlertDescription>
			</Alert>

			{isError ? (
				<Alert variant="destructive">
					<AlertDescription>读取失败：{errorMessage}。请刷新重试或检查权限。</AlertDescription>
				</Alert>
			) : isPending ? (
				<p className="text-sm text-muted-foreground">加载中…</p>
			) : data.length === 0 ? (
				<p className="text-sm text-muted-foreground">还没有登记外部 MCP 服务。</p>
			) : (
				<div className="flex flex-col gap-2">
					{data.map((s) => (
						<div key={s.id} className="flex items-start gap-3 rounded-md border border-border px-3 py-2.5">
							<div className="min-w-0 flex-1">
								<div className="flex flex-wrap items-center gap-2">
									<span className="text-sm font-medium text-foreground">{s.name}</span>
									<Badge variant="outline">{s.purpose === 'moderation' ? '内容审核' : '通用'}</Badge>
									{s.enabled ? <Badge variant="outline">已启用</Badge> : <Badge variant="outline">已停用</Badge>}
								</div>
								<p className="mt-1 truncate font-mono text-xs text-muted-foreground">{s.endpoint}</p>
								{s.header_keys && s.header_keys.length > 0 ? (
									<p className="mt-1 text-xs text-muted-foreground">鉴权头：{s.header_keys.join(', ')}（值已隐藏）</p>
								) : null}
							</div>
							<Button variant="ghost" size="sm" onClick={() => test(s)} disabled={testing === s.id}>
								{testing === s.id ? '测试中…' : '测试连通'}
							</Button>
							<Button variant="ghost" size="sm" onClick={() => openEdit(s)}>
								编辑
							</Button>
							<Button variant="ghost" size="sm" onClick={() => remove(s)}>
								删除
							</Button>
						</div>
					))}
				</div>
			)}

			<Dialog open={open} onOpenChange={setOpen}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>{editing ? '编辑外部 MCP 服务' : '登记外部 MCP 服务'}</DialogTitle>
					</DialogHeader>
					<div className="flex flex-col gap-3">
						<div className="flex flex-col gap-1.5">
							<Label>名称</Label>
							<Input value={name} onChange={(e) => setName(e.target.value)} placeholder="审核服务" />
						</div>
						<div className="flex flex-col gap-1.5">
							<Label>端点</Label>
							<Input
								value={endpoint}
								onChange={(e) => setEndpoint(e.target.value)}
								placeholder="https://example.com/mcp"
								className="font-mono text-xs"
							/>
						</div>
						<div className="grid grid-cols-2 gap-3">
							<div className="flex flex-col gap-1.5">
								<Label>用途</Label>
								<Select value={purpose} onValueChange={(v) => setPurpose(v as 'general' | 'moderation')}>
									<SelectTrigger>
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="general">通用</SelectItem>
										<SelectItem value="moderation">内容审核</SelectItem>
									</SelectContent>
								</Select>
							</div>
							{purpose === 'moderation' ? (
								<div className="flex flex-col gap-1.5">
									<Label>审核工具名</Label>
									<Input
										value={moderationTool}
										onChange={(e) => setModerationTool(e.target.value)}
										placeholder="check_content"
										className="font-mono text-xs"
									/>
								</div>
							) : null}
						</div>
						<div className="flex flex-col gap-1.5">
							<Label>鉴权头（JSON 对象）</Label>
							<Textarea
								rows={3}
								className="font-mono text-xs"
								placeholder='{"Authorization": "Bearer <token>"}'
								value={headers}
								onChange={(e) => setHeaders(e.target.value)}
							/>
							{editing ? <p className="text-xs text-muted-foreground">留空表示保持原有凭据不变。</p> : null}
						</div>
						<div className="flex items-center gap-2">
							<Switch checked={enabled} onCheckedChange={setEnabled} />
							<span className="text-sm">启用</span>
						</div>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setOpen(false)}>
							取消
						</Button>
						<Button onClick={save} disabled={saving}>
							{saving ? '保存中…' : '保存'}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
			{confirmDialog}
		</div>
	);
}
