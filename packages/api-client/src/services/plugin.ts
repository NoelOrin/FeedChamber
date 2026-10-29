import { post } from '../client';

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
};

export const pluginService = {
	list() {
		return post<Plugin[]>('/admin/plugins/list', {});
	},
	install(body: Partial<Plugin>) {
		return post<Plugin>('/admin/plugins/install', body);
	},
	update(id: string, body: Partial<Plugin>) {
		return post<null>(`/admin/plugins/${id}/update`, { ...body, id });
	},
	setEnabled(id: string, enabled: boolean) {
		return post<null>(`/admin/plugins/${id}/status`, { enabled });
	},
	delete(id: string) {
		return post<null>(`/admin/plugins/${id}/delete`, {});
	},
};
