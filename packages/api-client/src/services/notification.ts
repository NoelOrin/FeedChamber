import { post } from '../client';
import type { ListParams, Notification } from '../types';

export const notificationService = {
	list(params: ListParams & { type?: string; read?: 'all' | 'unread' | 'read' }) {
		return post<{ items: Notification[]; total: number }>('/notifications/list', params);
	},

	unreadCount() {
		return post<{ count: number }>('/notifications/unread_count');
	},

	read(id: string) {
		return post<null>(`/notifications/${id}/read`);
	},

	readAll() {
		return post<null>('/notifications/read_all');
	},

	delete(id: string) {
		return post<null>(`/notifications/${id}/delete`);
	},
};
