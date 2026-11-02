'use client';

import { mcpToolService } from '@feedchamber/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';

import { Alert, AlertDescription } from '~/components/ui/alert';
import { Badge } from '~/components/ui/badge';
import { Card } from '~/components/ui/card';
import { PageHeader } from '~/components/ui/page-header';
import { Switch } from '~/components/ui/switch';
import { toast } from '~/lib/toast';

/**
 * 本站 MCP Server 的工具管理。
 *
 * 这是「AI 能对本站做什么」的总开关面板。读工具默认开启，写工具默认关闭——
 * 新增写工具时默认值必须是「关」，不能因为忘登记就自动可写。
 */
export default function MCPToolsPage() {
	const queryClient = useQueryClient();
	const { data, isPending } = useQuery({
		queryKey: ['admin', 'mcp', 'tools'],
		queryFn: () => mcpToolService.list(),
	});
	const tools = data?.tools ?? [];

	const toggle = useCallback(
		async (name: string, enabled: boolean) => {
			try {
				await mcpToolService.setEnabled(name, enabled);
				await queryClient.invalidateQueries({ queryKey: ['admin', 'mcp', 'tools'] });
			} catch (err) {
				toast.error((err as Error).message);
			}
		},
		[queryClient],
	);

	const reads = tools.filter((t) => !t.write);
	const writes = tools.filter((t) => t.write);

	const renderList = (list: typeof tools, emptyText: string) =>
		list.length === 0 ? (
			<p className="text-sm text-muted-foreground">{emptyText}</p>
		) : (
			<div className="flex flex-col gap-2">
				{list.map((t) => (
					<div key={t.name} className="flex items-start gap-3 rounded-md border border-border px-3 py-2.5">
						<div className="min-w-0 flex-1">
							<div className="flex flex-wrap items-center gap-2">
								<code className="font-mono text-[13px] text-foreground">{t.name}</code>
								{t.perm ? <Badge variant="outline">{t.perm}</Badge> : null}
								{t.write ? <Badge variant="outline">写操作</Badge> : null}
							</div>
							<p className="mt-1 text-xs leading-relaxed text-muted-foreground">{t.description}</p>
						</div>
						<Switch checked={t.enabled} onCheckedChange={(v) => toggle(t.name, v)} />
					</div>
				))}
			</div>
		);

	return (
		<div className="flex flex-col gap-4">
			<PageHeader title="MCP 工具" meta={`${tools.filter((t) => t.enabled).length} / ${tools.length} 已启用`} />

			<Alert>
				<AlertDescription>
					MCP 端点为 <code className="font-mono text-xs">/mcp</code>，使用 API Key 鉴权（
					<code className="font-mono text-xs">X-API-Key</code> 或{' '}
					<code className="font-mono text-xs">Authorization: Bearer ak_...</code>）。 先在「服务设置 → 站点」开启{' '}
					<code className="font-mono text-xs">mcp_enabled</code>，
					否则所有请求都会被拒绝。每次工具调用都会写入审计日志。
				</AlertDescription>
			</Alert>

			<Alert>
				<AlertDescription>
					写工具调用走站点既有的审核策略，不会绕过审核。自定义层工具在凭证未持{' '}
					<code className="font-mono text-xs">customize:publish</code> 时，JS/HTML 类内容会被强制存为草稿。
				</AlertDescription>
			</Alert>

			{isPending ? (
				<p className="text-sm text-muted-foreground">加载中…</p>
			) : (
				<>
					<Card className="gap-3 p-4">
						<h2 className="text-xs font-medium text-muted-foreground">读工具</h2>
						{renderList(reads, '没有读工具')}
					</Card>
					<Card className="gap-3 p-4">
						<h2 className="text-xs font-medium text-muted-foreground">写工具（默认关闭）</h2>
						{renderList(writes, '没有写工具')}
					</Card>
				</>
			)}
		</div>
	);
}
