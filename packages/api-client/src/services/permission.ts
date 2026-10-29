import { post } from '../client';

export const permissionService = {
	list() {
		return post<{ id: string; code: string; name: string; group?: string }[]>('/permission/list');
	},

	getByRole(role: string) {
		return post<{ role: string; permissions: string[] }>('/permission/role', { role });
	},

	sync(role: string, permissions: string[]) {
		return post<{ role: string; permissions: string[] }>('/permission/sync', { role, permissions });
	},

	getSpaceRole(space_id: string, role: string) {
		return post<{ space_id: string; role: string; permissions: string[]; has_override: boolean }>('/permission/space', {
			space_id,
			role,
		});
	},

	syncSpaceRole(space_id: string, role: string, permissions: string[]) {
		return post<{ space_id: string; role: string; permissions: string[] }>('/permission/space/sync', {
			space_id,
			role,
			permissions,
		});
	},

	clearSpaceRole(space_id: string, role: string) {
		return post<null>('/permission/space/clear', { space_id, role });
	},

	getEffectiveSpaceRole(space_id: string, role: string) {
		return post<{
			space_id: string;
			role: string;
			permissions: string[];
			source_space_id: string;
			has_override: boolean;
			inherited: boolean;
		}>('/permission/space/effective', { space_id, role });
	},

	getSpacePermissionsTree(role: string, space_ids?: string[]) {
		return post<{ role: string; permissions: Record<string, string[]> }>('/permission/space/tree', {
			role,
			space_ids,
		});
	},
};
