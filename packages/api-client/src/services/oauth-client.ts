import { post } from '../client';

export type OAuthClient = {
	id: number;
	client_id: string;
	name: string;
	redirect_uris: string;
	scopes: string;
	created_by: string;
	created_at: number;
};

export type OAuthClientSecretResult = {
	client_id: string;
	client_secret: string;
};

export const oauthClientService = {
	list() {
		return post<OAuthClient[]>('/oauth/clients/list', {});
	},
	create(body: { name: string; redirect_uris: string; scopes?: string }) {
		return post<OAuthClientSecretResult>('/oauth/clients', body);
	},
	update(id: number, body: { name?: string; redirect_uris?: string; scopes?: string }) {
		return post<OAuthClient>(`/oauth/clients/${id}/update`, body);
	},
	resetSecret(id: number) {
		return post<OAuthClientSecretResult>(`/oauth/clients/${id}/reset-secret`, {});
	},
	delete(id: number) {
		return post<null>(`/oauth/clients/${id}/delete`, {});
	},
};
