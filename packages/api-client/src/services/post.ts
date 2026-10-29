import { get, post, postPaginated } from '../client';
import type { PostStatus } from '../constants';
import type { ListParams, Post, PostDetail, PostListItem, Tag } from '../types';

export const postService = {
	list(
		params: ListParams & {
			tag_name?: string;
			author_id?: string;
			space_id?: string;
			space_name?: string;
			space_slug?: string;
			include_sub?: boolean;
			status?: string;
			sort_by?: string;
			list_view?: 'new' | 'unread' | 'read' | 'posted';
		},
	) {
		return postPaginated<PostListItem[]>('/posts/list', params);
	},

	search(params: ListParams & { q: string }) {
		return postPaginated<PostListItem[]>('/posts/search', params);
	},

	suggestions(id: string) {
		return post<PostListItem[]>(`/posts/${id}/suggestions`, {});
	},

	getById(id: string) {
		return get<PostListItem>(`/posts/${id}`);
	},

	getDetail(id: string) {
		return get<PostDetail>(`/posts/${id}/detail`);
	},

	listByIds(ids: string[]) {
		return post<PostListItem[]>(`/posts/batch`, { ids });
	},

	create(body: { title: string; content: string; space_id?: string }) {
		return post<Post>('/posts/create', body);
	},

	update(id: string, body: { title?: string; content?: string }) {
		return post<Post>(`/posts/${id}/update`, body);
	},

	delete(id: string) {
		return post<null>(`/posts/${id}/delete`);
	},

	/** 审核状态流转（后台「通过 / 拒绝」）。权限码 report:manage，成功返回 data: null。 */
	setStatus(id: string, status: PostStatus) {
		return post<null>(`/posts/${id}/status`, { status });
	},

	/** 设为 / 取消精华（权限码 report:manage）。 */
	setFeatured(id: string, featured: boolean) {
		return post<Post>(`/posts/${id}/feature`, { featured });
	},

	togglePin(id: string) {
		return post<Post>(`/posts/${id}/pin`);
	},

	toggleLock(id: string) {
		return post<Post>(`/posts/${id}/lock`);
	},

	setClosed(id: string, closed: boolean) {
		return post<Post>(`/posts/${id}/close`, { closed });
	},

	getTags(postId: string) {
		return post<Tag[]>(`/posts/${postId}/tags`);
	},

	setTags(postId: string, tags: string[]) {
		return post<Tag[]>(`/posts/${postId}/tags/set`, { tags });
	},
	markRead(id: string, lastContentId?: string) {
		return post<null>(`/posts/${id}/read`, { last_content_id: lastContentId });
	},
};
