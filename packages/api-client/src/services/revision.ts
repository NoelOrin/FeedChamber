import { post, postPaginated } from '../client';
import type { ListParams } from '../types';

export type Revision = {
	id: string;
	target_type: 'post' | 'comment';
	target_id: string;
	title: string;
	body: string;
	author_id: string;
	created_at: string;
};

export const revisionService = {
	listPost(postId: string, params: ListParams = {}) {
		return postPaginated<Revision[]>(`/posts/${postId}/revisions/list`, params);
	},
	restorePost(postId: string, revisionId: string) {
		return post<unknown>(`/posts/${postId}/revisions/${revisionId}/restore`, {});
	},
	listComment(commentId: string, params: ListParams = {}) {
		return postPaginated<Revision[]>(`/comments/${commentId}/revisions/list`, params);
	},
	restoreComment(commentId: string, revisionId: string) {
		return post<unknown>(`/comments/${commentId}/revisions/${revisionId}/restore`, {});
	},
};
