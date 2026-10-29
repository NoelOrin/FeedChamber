import { post } from '../client';
import type { Invite, ListParams } from '../types';

export const inviteService = {
	list(params?: ListParams) {
		return post<{ items: Invite[]; total: number }>('/admin/invites/list', params ?? {});
	},

	create(body?: { max_uses?: number; expires_at?: string | null; code?: string }) {
		return post<Invite>('/admin/invites/create', body ?? {});
	},

	delete(id: string) {
		return post<null>(`/admin/invites/${id}/delete`);
	},
};
