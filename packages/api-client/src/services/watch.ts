import { post, postPaginated } from '../client';
import type { ListParams } from '../types';

export type Watch = {
	id: string;
	user_id: string;
	target_type: 'post' | 'space' | 'tag' | string;
	target_id: string;
	level: 'watching' | 'tracking' | 'muted' | string;
	created_at?: string;
};

export const watchService = {
	set(body: { target_type: string; target_id: string; level?: string }) {
		return post<Watch>('/watches/set', body);
	},

	list(params: ListParams = {}) {
		return postPaginated<Watch[]>('/watches/list', params);
	},

	delete(body: { target_type: string; target_id: string }) {
		return post<null>('/watches/delete', body);
	},

	status(body: { target_type: string; target_id: string }) {
		return post<{ watching: boolean; level: string; watch?: Watch }>('/watches/status', body);
	},
};
