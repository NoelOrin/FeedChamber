import { post } from '../client';

/** 外部 MCP Server 登记。`header_keys` 只返回值键名，不返回凭据。 */
export type MCPServer = {
	id: string;
	name: string;
	endpoint: string;
	transport: string;
	header_keys?: string[];
	purpose: 'general' | 'moderation';
	moderation_tool?: string;
	enabled: boolean;
	updated_by?: string;
	created_at?: string;
	updated_at?: string;
};

/** 连通性测试结果。失败时 `error` 有原因，`tools` 为空。 */
export type MCPTestResult = {
	ok: boolean;
	tools?: string[];
	error?: string;
};

export type MCPServerInput = {
	name: string;
	endpoint: string;
	transport?: string;
	/** JSON 对象字符串。更新时留空表示保持原有凭据不变。 */
	headers?: string;
	purpose?: 'general' | 'moderation';
	moderation_tool?: string;
	enabled?: boolean;
};

export const mcpServerService = {
	list() {
		return post<MCPServer[]>('/admin/mcp-servers/list', {});
	},
	create(body: MCPServerInput) {
		return post<MCPServer>('/admin/mcp-servers/create', body);
	},
	update(id: string, body: MCPServerInput) {
		return post<MCPServer>(`/admin/mcp-servers/${id}/update`, body);
	},
	delete(id: string) {
		return post<null>(`/admin/mcp-servers/${id}/delete`, {});
	},
	test(id: string) {
		return post<MCPTestResult>(`/admin/mcp-servers/${id}/test`, {});
	},
};

/** 本站 MCP 工具状态。`write` 为 true 的工具默认停用。 */
export type MCPToolState = {
	name: string;
	description: string;
	perm?: string;
	write: boolean;
	enabled: boolean;
};

/** 本站 MCP Server 的工具管理（P2，对外暴露方向）。 */
export const mcpToolService = {
	list() {
		return post<{ tools: MCPToolState[] }>('/admin/mcp/tools', {});
	},
	setEnabled(name: string, enabled: boolean) {
		return post<{ name: string; enabled: boolean }>('/admin/mcp/tools/status', { name, enabled });
	},
};
