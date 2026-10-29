import { getApiBase, post } from '../client';
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

	/** POST /notifications/stream — connect to SSE stream for real-time unread count.
	 *  Returns a Response (status 200 with `text/event-stream` body).
	 *  Caller reads via `response.body.getReader()` or passes to an EventSource polyfill.
	 */
	async streamRaw(): Promise<Response> {
		const res = await fetch(`${getApiBase()}/notifications/stream`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			// The HttpOnly session cookie is the only credential; there is no
			// JS-readable access token to put in an Authorization header.
			credentials: 'include',
		});
		if (!res.ok) {
			throw new Error(`SSE stream failed: ${res.status}`);
		}
		return res;
	},
};
