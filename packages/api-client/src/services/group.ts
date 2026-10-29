import { post } from '../client';
import type { Group, GroupMember } from '../types';

export const groupService = {
	list() {
		return post<Group[]>('/groups/list');
	},

	create(body: { name: string; display_name?: string; visibility?: string }) {
		return post<Group>('/groups/create', body);
	},

	update(id: string, body: { display_name?: string; visibility?: string }) {
		return post<Group>(`/groups/${id}/update`, body);
	},

	delete(id: string) {
		return post<null>(`/groups/${id}/delete`);
	},

	listMembers(id: string, params?: { page?: number; page_size?: number }) {
		return post<{ items: GroupMember[]; total: number }>(`/groups/${id}/members/list`, params);
	},

	addMember(id: string, body: { user_id: string; role?: string }) {
		return post<null>(`/groups/${id}/members/add`, body);
	},

	removeMember(id: string, body: { user_id: string }) {
		return post<null>(`/groups/${id}/members/remove`, body);
	},

	getPermissions(id: string) {
		return post<{ group_id: string; permissions: string[] }>(`/groups/${id}/permissions`, {});
	},

	syncPermissions(id: string, permissions: string[]) {
		return post<{ group_id: string; permissions: string[] }>(`/groups/${id}/permissions/sync`, {
			permissions,
		});
	},
};
