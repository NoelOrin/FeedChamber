import { get, post } from '../client';
import type { ActivityResponse, ListParams, PublicProfile, User } from '../types';

export const userService = {
	getProfile() {
		return get<User>('/user/profile');
	},

	getById(id: string) {
		return get<User>(`/user/${id}`);
	},

	getPublicProfile(id: string) {
		return get<PublicProfile>(`/user/${id}/profile`);
	},

	getActivity(id: string, year: number) {
		return post<ActivityResponse>(`/user/${id}/activity`, { year });
	},

	list(params: ListParams) {
		return post<{ items: User[]; total: number }>('/user/list', params);
	},

	search(params: ListParams & { keyword: string; role?: string }) {
		return post<{ list: User[] }>('/user/search', params);
	},

	updateProfile(body: {
		username?: string;
		display_name?: string;
		avatar?: string;
		bio?: string;
		cover_image?: string;
		profile_layout?: string;
	}) {
		return post<null>('/user/profile/update', body);
	},

	updateRole(id: string, role: string) {
		return post<null>(`/user/${id}/role/update`, { role });
	},

	delete(id: string) {
		return post<null>(`/user/${id}/delete`);
	},

	ban(id: string, params: { reason?: string; expires_at?: string; is_permanent?: boolean; permanent?: boolean }) {
		return post<null>(`/user/${id}/banned`, {
			reason: params.reason,
			expires_at: params.expires_at,
			is_permanent: params.is_permanent ?? params.permanent,
		});
	},

	unban(id: string) {
		return post<null>(`/user/${id}/unbanned`);
	},

	getBanStatus(id: string) {
		return post<{ banned: boolean; reason?: string; expires_at?: string }>(`/user/${id}/banned/status`);
	},
};
