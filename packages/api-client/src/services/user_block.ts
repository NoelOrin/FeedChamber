import { post, postPaginated } from '../client';
import type { ListParams } from '../types';

export const userBlockService = {
	block(userId: string) {
		return post<null>(`/users/${userId}/block`);
	},
	unblock(userId: string) {
		return post<null>(`/users/${userId}/unblock`);
	},
	status(userId: string) {
		return post<{ blocked: boolean }>(`/users/${userId}/block/status`);
	},
	list(params: ListParams = {}) {
		return postPaginated<string[]>('/blocks/list', params);
	},
};
