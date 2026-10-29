import { get, post } from '../client';

export type UserSession = {
	id: number;
	session_id: string;
	ip: string;
	user_agent: string;
	device_name: string;
	location: string;
	last_active: number;
	created_at: number;
	expires_at: number;
	is_current: boolean;
};

export const sessionService = {
	list() {
		return get<UserSession[]>('/sessions');
	},
	revoke(id: number) {
		return post<null>(`/sessions/${id}/revoke`, {});
	},
	logoutAll() {
		return post<null>('/sessions/logout-all', {});
	},
};
