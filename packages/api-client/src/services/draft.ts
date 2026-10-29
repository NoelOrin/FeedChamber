import { post, postPaginated } from '../client';
import type { ListParams } from '../types';

export type Draft = {
	id: string;
	user_id: string;
	kind: 'post' | 'reply';
	target_id?: string;
	title: string;
	body: string;
	space_id?: string;
	tags: string;
	updated_at: string;
};

export const draftService = {
	save(body: {
		kind: 'post' | 'reply';
		target_id?: string;
		title?: string;
		content?: string;
		space_id?: string;
		tags?: string[];
	}) {
		return post<Draft>('/drafts/save', body);
	},
	list(params: ListParams = {}) {
		return postPaginated<Draft[]>('/drafts/list', params);
	},
	delete(id: string) {
		return post<null>(`/drafts/${id}/delete`, {});
	},
};
