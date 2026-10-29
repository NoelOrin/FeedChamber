import { get, post } from '../client';
import type { Announcement, ListParams } from '../types';

export const announcementService = {
	listPublished(params?: ListParams) {
		return post<{ items: Announcement[]; total: number }>('/announcements/published', params);
	},

	getByID(id: string) {
		return get<Announcement>(`/announcements/${id}`);
	},

	// admin
	list(params?: ListParams) {
		return post<{ items: Announcement[]; total: number }>('/admin/announcements/list', params);
	},

	create(data: Partial<Announcement>) {
		return post<Announcement>('/admin/announcements/create', data);
	},

	update(id: string, data: Partial<Announcement>) {
		return post<Announcement>(`/admin/announcements/${id}/update`, data);
	},

	delete(id: string) {
		return post<null>(`/admin/announcements/${id}/delete`);
	},

	publish(id: string) {
		return post<null>(`/admin/announcements/${id}/publish`);
	},

	unpublish(id: string) {
		return post<null>(`/admin/announcements/${id}/unpublish`);
	},
};
