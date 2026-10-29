import { get, post, postPaginated } from '../client';
import type { Content, ContentListItem, ListParams } from '../types';

export const contentService = {
	list(postId: string, params: ListParams) {
		return postPaginated<ContentListItem[]>(`/posts/${postId}/contents/list`, params);
	},

	listAll(params: ListParams) {
		return postPaginated<ContentListItem[]>('/posts/comments/list', params);
	},

	adminGetById(id: string) {
		return get<ContentListItem>(`/comments/${id}`);
	},

	create(postId: string, body: { body: string; parent_id?: string }) {
		return post<Content>(`/posts/${postId}/contents/create`, body);
	},

	update(postId: string, id: string, body: { body: string }) {
		return post<Content>(`/posts/${postId}/contents/${id}/update`, body);
	},

	delete(postId: string, id: string) {
		return post<null>(`/posts/${postId}/contents/${id}/delete`);
	},

	replies(contentId: string, params: ListParams) {
		return postPaginated<ContentListItem[]>(`/contents/${contentId}/replies`, params);
	},

	adminDelete(commentId: string) {
		return post<null>(`/comments/${commentId}/delete`);
	},
};
