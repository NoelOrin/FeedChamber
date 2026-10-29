import { get, post } from '../client';
import type { ListParams, Report } from '../types';

export const reportService = {
	create(body: { target_type: string; target_id: string; flag_type?: string; reason: string }) {
		return post<Report>('/reports/create', body);
	},

	getById(id: string) {
		return get<Report>(`/reports/${id}`);
	},

	list(params: ListParams & { status?: string }) {
		return post<{ items: Report[]; total: number }>('/reports/list', params);
	},

	updateStatus(id: string, status: string, action: string = 'none', note: string = '') {
		return post<null>(`/reports/${id}/status`, { status, action, note });
	},

	delete(id: string) {
		return post<null>(`/reports/${id}/delete`);
	},
};
