import { post } from '../client';

/** 插件来源审批状态。未审批 / 已拒绝的来源不可安装。 */
export type PluginSourceStatus = 'pending' | 'approved' | 'rejected';

/** 插件来源类型。当前只支持受信任代码仓库。 */
export type PluginSourceKind = 'git';

/** 插件来源登记（POST /admin/plugins/sources/list）。 */
export type PluginSource = {
	id: string;
	name: string;
	kind: PluginSourceKind;
	url: string;
	status: PluginSourceStatus;
	approved_by: string;
	created_at: string;
	updated_at: string;
};

/** 新建来源的入参（POST /admin/plugins/sources/create）。 */
export type PluginSourceInput = {
	name: string;
	kind: PluginSourceKind;
	url: string;
};

/** 插件运行时状态机（宿主重启只恢复 running）。 */
export type PluginInstallStatus = 'installed' | 'starting' | 'running' | 'crashed' | 'stopped';

/** 一次安装的运行时记录（POST /admin/plugins/installs/list）。 */
export type PluginInstall = {
	id: string;
	plugin_id: string;
	source_id: string;
	dir: string;
	entry: string;
	status: PluginInstallStatus;
	/** 安装时授予的 capability 快照（JSON 数组字符串）。 */
	caps: string;
	restarts: number;
	last_error: string;
	last_started_at: string | null;
	created_at: string;
	updated_at: string;
};

/**
 * 安装入参（POST /admin/plugins/install）。
 *
 * Manifest 必须是 plugin.json 的**原文**：安装记录保留原始文本，
 * hooks / capabilities 由后端解析派生，前端不预先拆字段。
 */
export type PluginInstallInput = {
	plugin_id: string;
	source_id: string;
	manifest: string;
	/** 插件代码在磁盘上的绝对路径；留空时后端按插件根目录拼 <root>/<pluginID>。 */
	dir?: string;
};

/**
 * 更新入参（POST /admin/plugins/{id}/update）。
 *
 * `enabled` 必须显式传入：后端对零值 false 不做区分（`pluginRow.Enabled = input.Enabled`），
 * 省略会把插件静默禁用。编辑表单务必带上 `editing.enabled`。
 */
export type PluginUpdateInput = {
	name?: string;
	version?: string;
	author?: string;
	description?: string;
	homepage?: string;
	settings?: string;
	enabled: boolean;
};

/** 插件登记记录（POST /admin/plugins/list）。v2 起插件是 Node 子进程。 */
export type Plugin = {
	id: string;
	name: string;
	version: string;
	author: string;
	description: string;
	homepage: string;
	settings: string;
	enabled: boolean;
	created_at: string;
	updated_at: string;
	// v2 字段：plugin.json 原文 + 解析后的冗余列（JSON 数组字符串）。
	manifest?: string;
	protocol_version?: string;
	hooks?: string;
	capabilities?: string;
	source_id?: string;
};

export const pluginService = {
	list() {
		return post<Plugin[]>('/admin/plugins/list', {});
	},
	listSources() {
		return post<PluginSource[]>('/admin/plugins/sources/list', {});
	},
	createSource(body: PluginSourceInput) {
		return post<PluginSource>('/admin/plugins/sources/create', body);
	},
	approveSource(id: string) {
		return post<null>(`/admin/plugins/sources/${id}/approve`, {});
	},
	rejectSource(id: string) {
		return post<null>(`/admin/plugins/sources/${id}/reject`, {});
	},
	install(body: PluginInstallInput) {
		return post<PluginInstall>('/admin/plugins/install', body);
	},
	uninstall(id: string) {
		return post<null>(`/admin/plugins/${id}/uninstall`, {});
	},
	listInstalls() {
		return post<PluginInstall[]>('/admin/plugins/installs/list', {});
	},
	update(id: string, body: PluginUpdateInput) {
		return post<null>(`/admin/plugins/${id}/update`, { ...body, id, enabled: body.enabled });
	},
	setEnabled(id: string, enabled: boolean) {
		return post<null>(`/admin/plugins/${id}/status`, { enabled });
	},
	delete(id: string) {
		return post<null>(`/admin/plugins/${id}/delete`, {});
	},
};
