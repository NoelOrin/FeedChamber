import { post } from '../client';
import type { BannedUser } from '../types';

export const bannedService = {
	list(params?: { scope?: string; space_id?: string }) {
		return post<{ items: BannedUser[]; total: number }>('/admin/banned/list', params);
	},

	ban(userId: string, params: { reason?: string; expires_at?: string; is_permanent?: boolean; permanent?: boolean }) {
		return post<null>(`/user/${userId}/banned`, {
			reason: params.reason,
			expires_at: params.expires_at,
			is_permanent: params.is_permanent ?? params.permanent,
		});
	},

	unban(userId: string) {
		return post<null>(`/user/${userId}/unbanned`);
	},

	getBanStatus(userId: string) {
		return post<{ banned: boolean; reason?: string; expires_at?: string }>(`/user/${userId}/banned/status`);
	},

	getBanOverview(userId: string) {
		return post<{ items: BannedUser[]; total: number }>(`/user/${userId}/banned/overview`);
	},

	banInSpace(
		userId: string,
		spaceId: string,
		params: { reason?: string; expires_at?: string; is_permanent?: boolean; permanent?: boolean },
	) {
		return post<null>(`/user/${userId}/space/${spaceId}/banned`, {
			reason: params.reason,
			expires_at: params.expires_at,
			is_permanent: params.is_permanent ?? params.permanent,
		});
	},

	unbanInSpace(userId: string, spaceId: string) {
		return post<null>(`/user/${userId}/space/${spaceId}/unbanned`);
	},

	getBanStatusInSpace(userId: string, spaceId: string) {
		return post<{ banned: boolean; reason?: string; expires_at?: string }>(
			`/user/${userId}/space/${spaceId}/banned/status`,
		);
	},
};
