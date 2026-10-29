import { post } from '../client';

export const settingService = {
	getPublic() {
		return post<Record<string, string>>('/settings/public');
	},

	getAll() {
		return post<Record<string, string>>('/admin/settings');
	},

	get(key: string) {
		return post<string>(`/admin/settings/${key}`);
	},

	update(key: string, value: string) {
		return post<null>(`/admin/settings/${key}/update`, { value });
	},

	batchUpdate(settings: Record<string, string>) {
		return post<null>('/admin/settings/batch', { settings });
	},

	delete(key: string) {
		return post<null>(`/admin/settings/${key}/delete`);
	},
};
