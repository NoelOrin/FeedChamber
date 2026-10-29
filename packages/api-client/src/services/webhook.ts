import { post, postPaginated } from '../client';
import type { ListParams } from '../types';

export type Webhook = {
	id: string;
	url: string;
	events: string;
	enabled: boolean;
	tls_verify?: boolean;
	created_at?: string;
};

export const webhookService = {
	create(body: { url: string; secret?: string; events?: string[]; enabled?: boolean; tls_verify?: boolean }) {
		return post<Webhook>('/admin/webhooks/create', body);
	},
	list(params: ListParams = {}) {
		return postPaginated<Webhook[]>('/admin/webhooks/list', params);
	},
	update(
		id: string,
		body: { url?: string; secret?: string; events?: string[]; enabled?: boolean; tls_verify?: boolean },
	) {
		return post<Webhook>(`/admin/webhooks/${id}/update`, body);
	},
	delete(id: string) {
		return post<null>(`/admin/webhooks/${id}/delete`);
	},
	deliveries(id: string, params: ListParams = {}) {
		return postPaginated<any[]>(`/admin/webhooks/${id}/deliveries/list`, params);
	},
	ping(id: string) {
		return post<any>(`/admin/webhooks/${id}/ping`, {});
	},
	retry(id: string, deliveryId: string) {
		return post<any>(`/admin/webhooks/${id}/deliveries/${deliveryId}/retry`, {});
	},
};
