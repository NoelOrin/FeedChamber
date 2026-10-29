import { post } from '../client';
import type { Role } from '../types';

export const roleService = {
	list() {
		return post<Role[]>('/role/list');
	},

	create(body: { name: string; description?: string; display_name?: string }) {
		return post<Role>('/role/create', body);
	},

	update(id: string, name: string, display_name?: string, description?: string) {
		return post<Role>(`/role/${id}/update`, { name, display_name, description });
	},

	delete(id: string) {
		return post<null>(`/role/${id}/delete`);
	},
};
