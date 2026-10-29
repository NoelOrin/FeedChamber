import { post, postPaginated } from '../client';
import type { ListParams } from '../types';

export const favoriteService = {
	toggle(postId: string) {
		return post<{ favorited: boolean }>(`/posts/${postId}/favorite`);
	},

	status(postId: string) {
		return post<{ favorited: boolean }>(`/posts/${postId}/favorite/status`);
	},

	list(params: ListParams) {
		return postPaginated<string[]>('/favorites/list', params);
	},
};
