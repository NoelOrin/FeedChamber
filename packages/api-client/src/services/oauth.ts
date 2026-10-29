import { get, post } from '../client';
import type { LinkedOAuthAccount, OAuthProviderConfig } from '../types';

export const oauthService = {
	getAllProviders() {
		return get<Record<string, OAuthProviderConfig>>('/oauth/providers');
	},

	getProviderConfig(provider: string) {
		return get<OAuthProviderConfig>(`/oauth/${provider}/config`);
	},

	getAuthURL(provider: string) {
		return get<{ auth_url: string; state: string }>(`/oauth/${provider}/auth`);
	},

	callback(provider: string, code: string, state: string) {
		return get<{ access_token?: string; refresh_token?: string; needs_register?: boolean; oauth_user?: any }>(
			`/oauth/${provider}/callback?code=${encodeURIComponent(code)}&state=${encodeURIComponent(state)}`,
		);
	},

	getLinkedAccounts() {
		return get<LinkedOAuthAccount[]>('/oauth/accounts');
	},

	linkAccount(provider: string, code: string) {
		return post<null>(`/oauth/${provider}/link`, { code });
	},

	unlinkAccount(provider: string) {
		return post<null>(`/oauth/${provider}/unlink`);
	},
};
