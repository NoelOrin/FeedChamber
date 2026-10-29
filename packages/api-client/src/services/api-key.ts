import { post } from '../client';

export type APIKey = {
	id: number;
	key_id: string;
	name: string;
	scopes: string;
	rate_limit: number;
	expires_at?: number | null;
	last_used_at?: number | null;
	status: string;
	created_at: number;
};

export type APIKeyCreateResult = APIKey & { api_key: string };

export const apiKeyService = {
	create(body: { name: string; scopes?: string; rate_limit?: number; expires_at?: number | null }) {
		return post<APIKeyCreateResult>('/api-keys', body);
	},
	list() {
		return post<APIKey[]>('/api-keys/list', {});
	},
	revoke(id: number) {
		return post<null>(`/api-keys/${id}/revoke`);
	},
};
