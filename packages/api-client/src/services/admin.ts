import { post, postPaginated } from '../client';
import type {
	AdminStats,
	AuditLog,
	ContentListItem,
	DashboardData,
	ListParams,
	PostListItem,
	SearchResults,
	UserActivities,
} from '../types';

export const adminService = {
	getStats() {
		return post<AdminStats>('/admin/stats');
	},

	getDashboard() {
		return post<DashboardData>('/admin/dashboard');
	},

	search(params: { q: string; page: number; page_size: number }) {
		return post<SearchResults>('/admin/search', params);
	},

	getUserActivities(userId: string, params?: ListParams) {
		return post<UserActivities>(`/admin/user/${userId}/activities`, params);
	},

	batchUsers(params: { ids: string[]; action: string; params?: Record<string, string> }) {
		return post<null>('/admin/batch/users', params);
	},

	batchPosts(params: { ids: string[]; action: string }) {
		return post<null>('/admin/batch/posts', params);
	},

	batchComments(ids: string[]) {
		return post<null>('/admin/batch/comments', { ids, action: 'delete' });
	},

	reconcileCounters() {
		return post<null>('/admin/reconcile/counters');
	},

	listAuditLogs(params?: { page?: number; pageSize?: number; actor_id?: string; action?: string }) {
		return post<{ items: AuditLog[]; total: number }>('/admin/audit_logs/list', params);
	},

	listTrashPosts(params: ListParams = {}) {
		return postPaginated<PostListItem[]>('/admin/trash/posts/list', params);
	},

	restorePost(id: string) {
		return post<null>(`/admin/trash/posts/${id}/restore`, {});
	},

	listTrashComments(params: ListParams = {}) {
		return postPaginated<ContentListItem[]>('/admin/trash/comments/list', params);
	},

	restoreComment(id: string) {
		return post<null>(`/admin/trash/comments/${id}/restore`, {});
	},
};
